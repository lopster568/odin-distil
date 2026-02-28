# Odin — Comprehensive Project Context

## What It Is

Odin is a **local RAG (Retrieval-Augmented Generation) system** built in Go for indexing and querying large codebases (primarily Kubernetes, but extensible to any source tree). It runs entirely against local infrastructure — Ollama for LLMs and Qdrant for vector search — with an optional Gemini API integration for a multi-step agentic "orchestrator" layer.

The project has three major operating modes:

1. **Interactive agent** (`odin ask`) — question-answering REPL over indexed code
2. **Architecture distillation** (`odin distill`) — automated 6-stage pipeline that compresses a codebase into structured architectural knowledge artifacts
3. **Autonomous research** (`odin research`) — Gemini-orchestrated multi-step research loop that writes markdown artifacts to disk

---

## Infrastructure Requirements

| Service    | Address                       | Role                                                                |
| ---------- | ----------------------------- | ------------------------------------------------------------------- |
| Ollama     | `localhost:11434`             | Serves `qwen2.5:72b` (generation) + `nomic-embed-text` (embeddings) |
| Qdrant     | `localhost:6334`              | gRPC vector database, collection `odin_k8s`, 768-dim cosine         |
| Gemini API | `api.generativeai.google.com` | Used only by `chat` and `research` modes; key via `GEMINI_API_KEY`  |

---

## Module Layout

```
odin (module name)
cmd/odin/main.go          — CLI entrypoint, wires everything together
internal/
  llm/llm.go              — Thin wrapper over Ollama API (Generate + Embed)
  embedder/embedder.go    — Batch embedding, truncates to 6000 runes
  ingester/ingester.go    — AST-parses .go, heading-splits .md, raw fallback
  store/store.go          — Qdrant client (Upsert, Search, SearchWithFilter)
  tools/tools.go          — grep_symbol, get_file, list_package (shell-level)
  query/agent.go          — REPL agent with tool-calling & 6-message history
  query/query.go          — Simpler single-pass engine (not wired into main)
  distill/
    distill.go            — Distiller struct + Run() orchestration with checkpoints
    stages.go             — All 6 stage implementations + helpers (extractJSON, etc.)
    prompt.go             — All LLM prompt templates for each stage
    checkpoint.go         — checkpoint.json load/save/mark/done
  orchestrator/
    orchestrator.go       — Gemini-based chat orchestrator (RunChat)
    research.go           — Autonomous research loop (RunResearch)
```

---

## Ingest Pipeline

`odin ingest <path>` → walks a source tree using `ingester.Walk()`:

- **`.go` files**: AST-parsed per-declaration (one chunk per `FuncDecl`/`GenDecl`); falls back to 100-line raw chunks on parse failure
- **`.md` files**: Split on `#`/`##` headings
- **Metadata per chunk**: `FilePath`, `Package`, `Symbol`, `Repo` (top-level dir), `DirPrefix` (first 2-3 path segments)
- Chunks are embedded in **batches of 50** via `embedder.EmbedChunks()`, truncated to **6000 runes**
- Stored in Qdrant via `store.Upsert()` with **FNV-64a** point IDs (`filepath + symbol + index`)

---

## Query Agent (`odin ask`)

`agent.Ask()` flow:

1. Embed question → `nomic-embed-text`
2. Vector search Qdrant → top **15** results
3. Build single long prompt: system instructions + last **6** history messages + retrieved context + question
4. `qwen2.5:72b` first pass → may contain `TOOL: name(arg)` lines
5. **Tool execution** (plain-text protocol, not JSON function-calling):
   - `TOOL: grep_symbol(X)` → `grep -r` across `repoRoot` (`/root/repos`), max 5 files × 3 hits
   - `TOOL: get_file(/path)` → read file, truncated to 8000 chars; restricted to `/root/repos` or `/root/odin`
   - `TOOL: list_package(/path)` → grep for `^func|^type|^var|^const`, deduped, max 80 lines
6. If tools were called → second `qwen2.5:72b` pass incorporating tool results
7. Append exchange to history; `/clear` resets, `/quit` exits

---

## Distillation Pipeline (`odin distill [k8s]`)

Runs a **6-stage batch job** writing to `artifacts/k8s/`. Stages are idempotent via `checkpoint.json`:

| Stage | Key                | Input                                   | Output file                     |
| ----- | ------------------ | --------------------------------------- | ------------------------------- |
| 1     | — (always runs)    | Qdrant vector search per bucket         | In-memory `map[string][]string` |
| 2     | `responsibilities` | Bucket chunks                           | `responsibilities.json`         |
| 3     | `patterns`         | `responsibilities.json`                 | `patterns.json`                 |
| 4     | `control_loops`    | controllers entry from responsibilities | `control_loops.md`              |
| 5     | `friction`         | responsibilities + patterns             | `friction.json`                 |
| 6     | `study_topics`     | all prior artifacts                     | `study_topics.md`               |

**Kubernetes subsystem buckets** (hardcoded in `stages.go`): `apiserver`, `controllers`, `scheduler`, `kubelet`, `storage`, `admission`. Each has a probe query and path-prefix matchers used against Qdrant with `SearchWithFilter`.

**LLM fallout handling**: `extractJSON()` in `stages.go` strips markdown fences, fixes missing opening `[`, and heals truncated arrays before `json.Unmarshal`.

Stage 2 supports **resuming** — it reads any existing `responsibilities.json` on startup and skips already-processed buckets.

---

## Orchestrator / Chat (`odin chat`)

Uses the **Gemini API** (`gemini-3-flash-preview`) as the outer orchestrator with proper JSON function-calling:

- Exposes a single tool to Gemini: `query_codebase(question)` — which internally calls `agent.Ask()` (the local `qwen2.5:72b` RAG agent)
- Agentic loop: up to **5 rounds** of Gemini → tool calls → results → Gemini
- Full multi-turn conversation history maintained as `[]geminiContent`

---

## Autonomous Research (`odin research [target]`)

Uses Gemini with **two tools**: `query_codebase` and `write_artifact`:

- Reads a project idea from `ideas/<target>.md` (default: `jaeger`)
- Gemini drives a multi-step research session, calling `query_codebase` repeatedly
- `write_artifact` writes markdown documents to `artifacts/<target>/`
- Session progress logged to `artifacts/<target>/session.log`
- Prompt instructs Gemini to write artifacts every 5-6 tool calls to checkpoint findings

**Current research target**: a Jaeger GSoC project — "AI-powered trace analysis: self-service skills framework" built on LangChainGo + Jaeger v2 (OTel Collector architecture). The indexed repos include `repos/jaeger/` and `repos/jaeger/langchaingo/`.

---

## Key Constants

| Constant          | Value                          | Location                                |
| ----------------- | ------------------------------ | --------------------------------------- |
| `repoRoot`        | `/root/repos`                  | `cmd/odin/main.go`                      |
| `Collection`      | `"odin_k8s"`                   | `internal/store/store.go`               |
| `VectorSize`      | `768`                          | `internal/store/store.go`               |
| `defaultModel`    | `"qwen2.5:72b"`                | `internal/distill/distill.go`           |
| `maxChunkRunes`   | `6000`                         | `internal/embedder/embedder.go`         |
| Ingest batch size | `50`                           | `cmd/odin/main.go`                      |
| History window    | `6` messages                   | `internal/query/agent.go`               |
| Max tool calls    | `3` per turn (prompt-enforced) | `internal/query/agent.go`               |
| Max Gemini rounds | `5`                            | `internal/orchestrator/orchestrator.go` |

---

## Go Module Dependencies

```
github.com/ollama/ollama v0.17.0     — Ollama API client
github.com/qdrant/go-client v1.17.1  — Qdrant gRPC client
google.golang.org/grpc v1.78.0       — gRPC transport for Qdrant
```

Gemini is called via raw HTTP (`net/http`), not a Go SDK. No Gemini SDK dependency.

---

## Notable Design Decisions / Quirks

- **Plain-text tool protocol** in `agent.go` rather than structured function-calling — lines matching `TOOL: name(arg)` are parsed by string scanning, not JSON
- `store.SearchWithFilter` fetches `limit*5` results from Qdrant then post-filters by `strings.Contains(filepath, prefix)` — not a native Qdrant filter
- Point IDs are `hash(filepath + symbol + batchIndex)` — **not content-hashed**, so re-ingesting with different batch sizes creates duplicate points
- `query.Engine` in `internal/query/query.go` is a simpler single-pass version with no tool-calling; exists but is **not wired into main**
- `get_file` tool path allowlist is hardcoded to `/root/repos` and `/root/odin` — fails in non-root environments
- All distillation stage prompts instruct the LLM to return raw JSON with no markdown fences, but `extractJSON()` exists as a defense layer against that failing
