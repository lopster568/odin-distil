#!/usr/bin/env bash
# rebuild.sh — Odin full environment rebuild
# Run from inside the odin/ directory: bash scripts/rebuild.sh
set -euo pipefail

ODIN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="/tmp/odin-rebuild-logs"
mkdir -p "$LOG_DIR"

# ─── Colors ───────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
ok()   { echo -e "${GREEN}✓ $1${NC}"; }
warn() { echo -e "${YELLOW}⚠ $1${NC}"; }
die()  { echo -e "${RED}✗ $1${NC}"; exit 1; }
step() { echo -e "\n${YELLOW}▶ $1${NC}"; }

echo "═══════════════════════════════════════════"
echo "         ODIN ENVIRONMENT REBUILD"
echo "═══════════════════════════════════════════"

# ─── Step 1: Go ───────────────────────────────────────────────────────────────
step "Go 1.23.5"
if command -v go &>/dev/null && go version | grep -q "go1.23"; then
  ok "Go already installed: $(go version)"
else
  wget -q --show-progress https://go.dev/dl/go1.23.5.linux-amd64.tar.gz -O /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz

  # Add to bashrc if not already there
  if ! grep -q '/usr/local/go/bin' ~/.bashrc; then
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
  fi
  export PATH=$PATH:/usr/local/go/bin
  ok "Go installed: $(go version)"
fi

# ─── Step 2: Docker ───────────────────────────────────────────────────────────
step "Docker"
if command -v docker &>/dev/null; then
  ok "Docker already installed: $(docker --version)"
else
  curl -fsSL https://get.docker.com | sh
  ok "Docker installed"
fi

# Make sure docker daemon is running
if ! docker info &>/dev/null; then
  warn "Docker daemon not running, starting..."
  systemctl start docker || die "Could not start Docker daemon"
fi

# ─── Step 3: Ollama ───────────────────────────────────────────────────────────
step "Ollama (ROCm)"
if ! command -v ollama &>/dev/null; then
  curl -fsSL https://ollama.com/install.sh | sh
  ok "Ollama installed"
else
  ok "Ollama already installed"
fi

# Start ollama serve if not running
if ! pgrep -x ollama &>/dev/null; then
  OLLAMA_HOST=0.0.0.0 nohup ollama serve > /tmp/ollama.log 2>&1 &
  echo "  Waiting for Ollama to start..."
  sleep 10
fi

# Verify it's up
if curl -s http://localhost:11434/api/tags &>/dev/null; then
  ok "Ollama is running"
else
  die "Ollama failed to start — check /tmp/ollama.log"
fi

# Pull models if missing
step "Ollama models"
MODELS=$(ollama list 2>/dev/null || true)

if echo "$MODELS" | grep -q "qwen2.5:72b"; then
  ok "qwen2.5:72b already present"
else
  warn "Pulling qwen2.5:72b — this will take a while..."
  ollama pull qwen2.5:72b
  ok "qwen2.5:72b pulled"
fi

if echo "$MODELS" | grep -q "nomic-embed-text"; then
  ok "nomic-embed-text already present"
else
  warn "Pulling nomic-embed-text..."
  ollama pull nomic-embed-text
  ok "nomic-embed-text pulled"
fi

# ─── Step 4: Qdrant ───────────────────────────────────────────────────────────
step "Qdrant"
if docker ps -a --format '{{.Names}}' | grep -q '^qdrant$'; then
  # Container exists — make sure it's running
  if ! docker ps --format '{{.Names}}' | grep -q '^qdrant$'; then
    warn "Qdrant container exists but is stopped, starting..."
    docker start qdrant
  fi
  ok "Qdrant running (existing container, data preserved)"
else
  warn "Qdrant container not found, creating fresh..."
  mkdir -p ~/qdrant_data
  docker run -d --name qdrant \
    -p 6333:6333 -p 6334:6334 \
    -v ~/qdrant_data:/qdrant/storage \
    qdrant/qdrant:latest
  ok "Qdrant started (fresh)"
fi

# Wait for Qdrant to be ready
echo "  Waiting for Qdrant..."
for i in {1..15}; do
  if curl -s http://localhost:6333/healthz &>/dev/null; then
    ok "Qdrant is healthy"
    break
  fi
  sleep 2
  if [ $i -eq 15 ]; then
    die "Qdrant did not become healthy in time"
  fi
done

# ─── Step 5: Build Odin ───────────────────────────────────────────────────────
step "Build Odin"
cd "$ODIN_DIR"
go build -o odin ./cmd/odin/ || die "go build failed"
cp odin /usr/local/bin/odin
ok "Odin built and installed to /usr/local/bin/odin"

# ─── Step 6: health alias ─────────────────────────────────────────────────────
step "Shell aliases"
if ! grep -q 'alias health=' ~/.bashrc; then
  echo "alias health='$ODIN_DIR/scripts/health.sh'" >> ~/.bashrc
  ok "Added 'health' alias to ~/.bashrc"
else
  ok "health alias already in ~/.bashrc"
fi

# ─── Step 7: Clone repos if missing ──────────────────────────────────────────
step "Source repos"
REPO_DIR="$ODIN_DIR/repos"
mkdir -p "$REPO_DIR/jaeger"

clone_if_missing() {
  local url=$1
  local dest=$2
  if [ -d "$dest/.git" ]; then
    ok "Already cloned: $dest"
  else
    warn "Cloning $(basename $dest)..."
    git clone --depth=1 "$url" "$dest"
    ok "Cloned: $(basename $dest)"
  fi
}

clone_if_missing https://github.com/kubernetes/kubernetes.git         "$REPO_DIR/kubernetes"
clone_if_missing https://github.com/kubernetes/client-go.git          "$REPO_DIR/client-go"
clone_if_missing https://github.com/kubernetes-sigs/controller-runtime.git "$REPO_DIR/controller-runtime"
clone_if_missing https://github.com/kubernetes/enhancements.git       "$REPO_DIR/enhancements"
clone_if_missing https://github.com/jaegertracing/jaeger.git          "$REPO_DIR/jaeger/jaeger"
clone_if_missing https://github.com/tmc/langchaingo.git               "$REPO_DIR/jaeger/langchaingo"

# ─── Step 8: Ingestion reminder ───────────────────────────────────────────────
echo ""
echo "═══════════════════════════════════════════"
echo "  Rebuild complete. Run health to verify."
echo ""
echo "  To ingest all repos (run in background):"
echo ""
echo "  nohup odin ingest $REPO_DIR/kubernetes          > $LOG_DIR/ingest-k8s.log 2>&1 &"
echo "  nohup odin ingest $REPO_DIR/client-go           > $LOG_DIR/ingest-client-go.log 2>&1 &"
echo "  nohup odin ingest $REPO_DIR/controller-runtime  > $LOG_DIR/ingest-cr.log 2>&1 &"
echo "  nohup odin ingest $REPO_DIR/enhancements        > $LOG_DIR/ingest-keps.log 2>&1 &"
echo "  nohup odin ingest $REPO_DIR/jaeger/jaeger       > $LOG_DIR/ingest-jaeger.log 2>&1 &"
echo "  nohup odin ingest $REPO_DIR/jaeger/langchaingo  > $LOG_DIR/ingest-langchaingo.log 2>&1 &"
echo "═══════════════════════════════════════════"

source ~/.bashrc 2>/dev/null || true
