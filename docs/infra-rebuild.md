# Infra Rebuild Guide

## Before Destroying the VM

### Save These (run on the server)

```bash
# 1. Push all code
cd ~/odin && git push

# 2. Snapshot Qdrant (128k vectors — expensive to re-ingest)
docker cp qdrant:/qdrant/storage ~/qdrant-storage
tar czf ~/qdrant-snapshot.tar.gz ~/qdrant-storage
```

Then from your local machine:
```bash
scp root@<vm-ip>:~/qdrant-snapshot.tar.gz ~/qdrant-snapshot.tar.gz
```

### Also grab artifacts if distillation completed:
```bash
scp -r root@<vm-ip>:~/odin/artifacts ~/odin-distil/artifacts
```

---

### Delete / Don't Bother Saving

| Thing | Why |
|---|---|
| Ollama models (`qwen2.5:72b`, `nomic-embed-text`) | Re-pull with `ollama pull` — ~50GB but automated |
| `/root/repos` (k8s source) | `git clone` — no custom state |
| `odin` binary | `go build ./cmd/odin/` — 2 seconds |
| Qdrant Docker image | Re-pulled automatically on `docker run` |

---

## Rebuild From Bare Machine

### 1. System dependencies

```bash
# ROCm (for AMD GPU) — follow AMD's official installer for your distro
# Docker
curl -fsSL https://get.docker.com | sh

# Go 1.24+
wget https://go.dev/dl/go1.24.1.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && source ~/.bashrc
```

### 2. Ollama

```bash
curl -fsSL https://ollama.com/install.sh | sh
ollama pull qwen2.5:72b
ollama pull nomic-embed-text
```

### 3. Qdrant — restore from snapshot

```bash
# Copy snapshot to new machine, then:
tar xzf ~/qdrant-snapshot.tar.gz

docker run -d \
  -p 6333-6334:6333-6334 \
  -v $(pwd)/qdrant-storage:/qdrant/storage \
  --name qdrant \
  qdrant/qdrant:latest
```

Verify 128k vectors are back:
```bash
curl -s http://localhost:6333/collections/odin_k8s | \
  python3 -c "import sys,json; d=json.load(sys.stdin); print(d['result']['points_count'])"
```

**If snapshot is lost** (full re-ingest):
```bash
git clone https://github.com/kubernetes/kubernetes /root/repos/kubernetes
./odin ingest /root/repos/kubernetes
# Expect several hours of embedding compute
```

### 4. Clone and build Odin

```bash
git clone <your-odin-repo> ~/odin
cd ~/odin
go build ./cmd/odin/
```

### 5. Verify everything is healthy

```bash
./scripts/health.sh
```

Expected output: Ollama UP, Qdrant UP, vectors > 0, GPU visible via `rocm-smi`.

### 6. Resume distillation (if not complete)

```bash
# Copy artifacts back if you saved them
cp -r ~/odin-distil/artifacts ~/odin/artifacts

# Re-run — checkpoint.json will skip completed stages
./odin distill k8s
```

---

## Quick Reference

| Service | Port | How to start |
|---|---|---|
| Ollama | `11434` (HTTP) | `ollama serve` or auto-starts |
| Qdrant | `6333` (REST), `6334` (gRPC) | `docker start qdrant` |

| Command | What it does |
|---|---|
| `./odin ingest <path>` | Index a source tree into Qdrant |
| `./odin ask` | Interactive RAG REPL |
| `./odin distill k8s` | Run 6-stage distillation pipeline |
| `./scripts/health.sh` | Check GPU + Ollama + Qdrant + ingest jobs |
