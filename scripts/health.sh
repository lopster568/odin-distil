#!/bin/bash
echo "═══════════════════════════════════════════"
echo "           ODIN HEALTH CHECK"
echo "═══════════════════════════════════════════"

echo ""
echo "▶ GPU"
rocm-smi | grep -E "^\s*0\s" | awk '{printf "  Temp: %s | Power: %s | VRAM: %s | GPU%%: %s\n", $5, $6, $12, $13}'

echo ""
echo "▶ OLLAMA"
if pgrep ollama > /dev/null; then
  echo "  Status: UP (pid $(pgrep ollama))"
  ollama list 2>/dev/null | tail -n +2 | awk '{printf "  Model: %s (%s)\n", $1, $3}'
else
  echo "  Status: DOWN"
fi

echo ""
echo "▶ QDRANT"
HEALTH=$(curl -s http://localhost:6333/healthz 2>/dev/null)
if [ "$HEALTH" = "healthz check passed" ]; then
  VECTORS=$(curl -s http://localhost:6333/collections/odin_k8s 2>/dev/null | \
    python3 -c "import sys,json; d=json.load(sys.stdin); print(d['result']['points_count'])" 2>/dev/null)
  echo "  Status: UP"
  echo "  Vectors indexed: ${VECTORS:-unknown}"
else
  echo "  Status: DOWN"
fi

echo ""
echo "▶ INGEST JOBS"
for log in /tmp/ingest-*.log; do
  [ -f "$log" ] || continue
  name=$(basename $log .log | sed 's/ingest-//')
  last=$(tail -1 $log 2>/dev/null)
  if echo "$last" | grep -q "Done"; then
    echo "  [$name] ✓ $last"
  elif echo "$last" | grep -q "indexed"; then
    progress=$(echo "$last" | grep -oP '\d+ / \d+')
    echo "  [$name] 🔄 $progress"
  else
    echo "  [$name] ? $last"
  fi
done

echo ""
echo "═══════════════════════════════════════════"
