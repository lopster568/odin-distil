# Odin — Copilot Instructions

## What This Project Is

Odin is a RAG (Retrieval-Augmented Generation) agent for exploring Kubernetes and Go source code. It indexes a source tree into a vector database, then answers questions using semantic search + an LLM with tool-calling.

## Architecture Overview

**Ingest pipeline** (`odin ingest <path>`):

```
Source files → ingester.Walk() → embedder.EmbedChunks() → store.Upsert()
```

- `ingester` AST-parses `.go` files into per-declaration chunks; `.md` files split on `#`/`##` headings; unparseable files fall back to 100-line raw chunks
- `embedder` truncates chunks to **6000 runes** before calling `nomic-embed-text` via Ollama
- `store` upserts into Qdrant collection `odin_k8s` (768-dim cosine vectors); point IDs are **FNV-64a** hashes of `filepath + symbol + index`

**Query pipeline** (`odin ask`):

```
Question → embed → Qdrant search (top 15) → build prompt → qwen2.5:72b → tool calls → final answer
```

- `Agent` in `internal/query/agent.go` is the active agentic path (conversation history, 2-pass generation)
- `Engine` in `internal/query/query.go` is a simpler single-pass version (no tools, no history) — kept but not wired into `main.go`

## External Dependencies (must be running)

| Service | Address           | Notes                                                                   |
| ------- | ----------------- | ----------------------------------------------------------------------- |
| Ollama  | `localhost:11434` | HTTP; serves `qwen2.5:72b` + `nomic-embed-text`                         |
| Qdrant  | `localhost:6334`  | **gRPC** port (not the REST 6333); collection auto-created on first run |

Check with `./scripts/health.sh` (also shows AMD GPU via `rocm-smi` and ingest log progress from `/tmp/ingest-*.log`).

## Key Constants & Config

- `repoRoot = "/root/repos"` — hardcoded in `cmd/odin/main.go`; passed to `Agent` for `grep_symbol` searches
- `get_file` tool restricts paths to `/root/repos` or `/root/odin`
- Embed batch size: **50 chunks** per Ollama call
- Conversation history: last **6 messages** kept (trimmed in `agent.Ask`)
- Vector size: **768** (`store.VectorSize`); Qdrant collection name: `"odin_k8s"` (`store.Collection`)

## Tool-Calling Protocol

The agent uses a plain-text protocol (not JSON function-calling):

```
TOOL: grep_symbol(ReconcileLoop)
TOOL: get_file(/root/repos/staging/src/k8s.io/api/core/v1/types.go)
TOOL: list_package(/root/repos/pkg/controller/deployment)
```

Parsed in `agent.executeTools()` by scanning lines for the `TOOL:` prefix. A second LLM generation incorporates the results. Up to 3 tool calls are permitted per turn (by system prompt instruction, not enforced in code).

## Build & Run

```bash
go build ./cmd/odin/          # produces ./odin binary
./odin ingest /root/repos/... # index a Go source tree
./odin ask                    # start interactive REPL (/clear, /quit)
./scripts/health.sh           # check Ollama + Qdrant + GPU status
```

## Go Module

Module name is `odin` (bare name, not a GitHub path). Internal imports use `odin/internal/...`. All `go.mod` dependencies are marked `// indirect` — this is intentional as the single main package drives everything.

## Adding a New Tool

1. Implement in `internal/tools/tools.go` returning a `tools.Result`
2. Add a `case` in `agent.executeTool()` in `internal/query/agent.go`
3. Update the `Available tools:` section of the system prompt in `agent.Ask()`
