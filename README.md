# Odin

**A local-first autonomous code-intelligence agent.**

Odin indexes large Go source trees into a vector database, then runs a multi-model agent stack over the index: a planner LLM decides what to investigate, a local grounding LLM answers against retrieved code, and a research loop writes structured markdown artifacts (interface sketches, extension points, risk registers) to disk.

**Impact.** Odin helped me contribute to large codebases by distilling and querying implementations and features I would otherwise spend days manually mapping. The `artifacts/jaeger/` directory in this repo is the real output of a single 30-minute autonomous research run against the Jaeger v2 codebase: 10 structured markdown artifacts covering extension points, the skills-framework design, MCP integration, and frontend/API interop, all cited against exact file paths. No human intervention between `odin research jaeger` and `research_complete.md`.

---

## Why this exists

Most "chat with your code" tools solve a shallow problem: one question, one answer, context forgotten. Serious contribution work on a large CNCF project is the opposite: you spend days mapping interfaces, tracing extension points, drafting design docs before writing a single line.

Odin automates the days part.

Given a project brief (`ideas/jaeger.md`), Odin runs up to 50 rounds of planner-grounder-writer loops, checkpointing findings every few rounds, and stops when it has a self-consistent research package. The output is not a transcript. It is a directory of markdown files you can paste directly into a design doc.

### Measured on a real run (`odin research jaeger`)

Run against a Qdrant index of roughly **30,000 AST-aware chunks** drawn from the Kubernetes, Jaeger v2, and LangChainGo source trees.

| Metric                       | Value                                         |
|------------------------------|-----------------------------------------------|
| Corpus indexed               | ~30K chunks, 768-dim vectors                  |
| Wall-clock time              | 30 min 3 sec, unattended                      |
| Autonomous rounds            | 47 (of 50 max)                                |
| Grounded codebase queries    | 37                                            |
| Research artifacts written   | 10 markdown files (~25 KB)                    |
| Write-reminder nudges fired  | 9 (agent needed frequent pacing pressure)     |
| Hardware                     | Local AMD GPU via ROCm                        |
| External API                 | Gemini planner only; grounding stays local    |

The full `session.log` is committed alongside the artifacts, so every query, every tool call, every artifact write is auditable.

---

## What makes it different

**Dual-model orchestration.** Most agents use a single model for both planning and answering. Odin splits them:

- **Planner (Gemini 3 Flash)** decides what to ask next, calls tools via structured function-calling, and writes artifacts.
- **Grounder (Qwen2.5-72B, local via Ollama on ROCm)** answers every codebase query against retrieved chunks. Never sees the user's project brief directly. Never touches the outside network.

This matters because planning is cheap reasoning on short context, while grounding needs a large model that has actually seen the code. Using the right tool for each halves latency and keeps source code private.

**AST-aware ingestion.** `.go` files are parsed into per-declaration chunks (`func`, `type`, `const`, `var`), not split by line count. Markdown splits on `#`/`##` headings. Each chunk carries repo, directory-prefix, package, and symbol metadata, which the distillation pipeline uses to filter results per subsystem.

**Resumable pipelines.** The `distill` command runs a 6-stage pipeline that checkpoints after every stage. Crash mid-run, restart, skip to where you left off. Stage 2 checkpoints per-bucket, so you never re-run a 72B-model generation you already paid for.

**Local by default.** Grounding model, embedder, and vector DB all run on localhost. The planner is the only optional network call and can be swapped.

---

## Architecture

```
            ┌────────────────────────────────────────────────────────┐
            │                        odin CLI                        │
            └───────────┬────────────────────────────┬───────────────┘
                        │                            │
          ┌─────────────▼─────────────┐   ┌──────────▼──────────┐
          │      ingest pipeline      │   │   query / research  │
          │                           │   │                     │
          │  Walk → AST chunk →       │   │    Gemini planner   │
          │  embed (nomic) → upsert   │   │         ↕           │
          └─────────────┬─────────────┘   │    Qwen grounder    │
                        │                 │         ↕           │
                        │                 │  grep_symbol /      │
                        ▼                 │  get_file /         │
                 ┌────────────┐           │  list_package       │
                 │   Qdrant   │◄──────────┤  tools              │
                 │  (768-dim) │           │                     │
                 └────────────┘           └──────────┬──────────┘
                                                     │
                                                     ▼
                                          ┌────────────────────┐
                                          │  artifacts/<tgt>/  │
                                          │  *.md research     │
                                          │  *.json responsib. │
                                          └────────────────────┘
```

**Four operational modes on top of the same index:**

| Mode       | Orchestrator         | Grounder  | Use case                                    |
| ---------- | -------------------- | --------- | ------------------------------------------- |
| `ingest`   | none                 | nomic     | One-shot index of a source tree             |
| `ask`      | Qwen (plain-text)    | Qwen      | Interactive REPL, local-only                |
| `chat`     | Gemini (func-calling)| Qwen      | Interactive REPL with stronger planning     |
| `research` | Gemini, autonomous   | Qwen      | Multi-hour unattended research run          |
| `distill`  | deterministic stages | Qwen      | K8s-specific 6-stage architecture extract   |

---

## Quickstart

### Prerequisites

- Go 1.24+
- Docker (for Qdrant)
- An AMD or NVIDIA GPU with enough VRAM for a 72B quantised model (48GB+ recommended)
- `GEMINI_API_KEY` in env for `chat` and `research` modes

### One-shot setup

```bash
bash scripts/setup.sh
```

This installs Go, Docker, Ollama (with ROCm), pulls `qwen2.5:72b` and `nomic-embed-text`, starts a Qdrant container, builds the binary, and clones the target repos (Kubernetes, client-go, controller-runtime, enhancements, Jaeger, LangChainGo).

### Manual setup

```bash
# Ollama
curl -fsSL https://ollama.com/install.sh | sh
ollama pull qwen2.5:72b
ollama pull nomic-embed-text

# Qdrant (gRPC on 6334, REST on 6333)
docker run -d --name qdrant -p 6333:6333 -p 6334:6334 \
  -v ~/qdrant_data:/qdrant/storage qdrant/qdrant:latest

# Build
go build -o odin ./cmd/odin/
```

### Health check

```bash
./scripts/health.sh
```

Reports GPU, Ollama, Qdrant status, and progress of any running ingest jobs.

---

## Usage

### 1. Index a codebase

```bash
./odin ingest /path/to/kubernetes
```

Walks the tree, AST-chunks Go files, splits markdown on headings, embeds in batches of 50, upserts to Qdrant collection `odin_k8s`. Deterministic point IDs (FNV-64a hash of `filepath + symbol + index`) so re-ingesting the same tree is idempotent.

### 2. Interactive Q&A

**Local only:**
```bash
./odin ask
>>> how does the scheduler implement preemption?
```

The agent vector-searches the top 15 chunks, builds a prompt, generates a response, and if the response contains `TOOL: grep_symbol(Preempt)` or `TOOL: get_file(path)`, executes the tool and generates a second pass with the results. History is trimmed to the last 6 messages.

**With Gemini planning:**
```bash
GEMINI_API_KEY=... ./odin chat
>>> how does the scheduler implement preemption, and where would I add a new plugin?
```

Gemini plans and may call `query_codebase` (the local Qwen agent) multiple times with refined sub-questions.

### 3. Autonomous research

Write a project brief in `ideas/<target>.md` describing what you want to build or contribute. Then:

```bash
GEMINI_API_KEY=... ./odin research jaeger
```

The agent runs up to 50 rounds: query the codebase, refine, query again, write checkpoints every 5-6 rounds. It writes a final `research_complete.md` when done. Everything lands in `artifacts/<target>/`.

See `artifacts/jaeger/` in this repo for a real output: 10 markdown files covering Jaeger v2 extension points, MCP foundation analysis, skills-engine implementation plan, frontend/API integration, and a complete research summary. Produced unattended from the brief in `ideas/jaeger.md`.

### 4. Architecture distillation (Kubernetes-specific)

```bash
./odin distill k8s
```

Runs 6 deterministic stages:

| # | Stage                         | Output                    |
|---|-------------------------------|---------------------------|
| 1 | Bucket retrieval              | in-memory                 |
| 2 | Responsibility extraction     | `responsibilities.json`   |
| 3 | Pattern mining                | `patterns.json`           |
| 4 | Control-loop abstraction      | `control_loops.md`        |
| 5 | Friction and opportunity mining| `friction.json`          |
| 6 | Study topic generation        | `study_topics.md`         |

Each stage reads the previous stage's artifact, so a run is resumable via `checkpoint.json`. Stage 2 additionally checkpoints per Kubernetes subsystem (apiserver, controllers, scheduler, kubelet, storage, admission), so a crash mid-stage-2 resumes at the right bucket. See `artifacts/k8s/` for a full output.

---

## Real output

Two full runs are committed to this repo as evidence.

**`artifacts/jaeger/`** — autonomous `research` mode, 30-minute run:

```
artifacts/jaeger/
├── initial_structure_and_v2_analysis.md   (2.0 KB)
├── phase1_ai_analysis.md                  (2.3 KB)
├── mcp_foundation_analysis.md             (2.0 KB)
├── extension_and_skills_framework.md      (2.6 KB)
├── extension_lookup_and_api.md            (2.6 KB)
├── api_and_extension_interop.md           (2.2 KB)
├── integration_strategy.md                (2.3 KB)
├── frontend_and_api_integration.md        (2.4 KB)
├── skills_engine_implementation_plan.md   (2.9 KB)
├── research_complete.md                   (4.1 KB, executive summary)
└── session.log                            (full audit trail)
```

**`artifacts/k8s/`** — deterministic `distill` pipeline across Kubernetes subsystems:

```
artifacts/k8s/
├── responsibilities.json  (4.8 KB, 6 subsystems: apiserver, controllers,
│                           scheduler, kubelet, storage, admission)
├── patterns.json          (1.8 KB, recurring architectural patterns)
├── control_loops.md       (8.7 KB, reconciliation loop abstraction)
├── friction.json          (2.6 KB, friction + opportunity register)
├── study_topics.md        (7.1 KB, ranked learning agenda)
└── checkpoint.json        (resume state)
```

Every artifact cites exact file paths, interface signatures, and type definitions from source. The grounder always sees retrieved chunks before answering, and is prompted to cite paths from those chunks — spot-checking the committed artifacts against the Jaeger source shows the cited paths exist.

---

## Repo layout

```
cmd/odin/              CLI entrypoint
internal/
  ingester/            AST + markdown chunker
  embedder/            nomic-embed-text client
  store/               Qdrant wrapper (gRPC)
  llm/                 Ollama client
  query/               Agent with plain-text tool protocol
  orchestrator/        Gemini chat + autonomous research loop
  distill/             6-stage K8s architecture pipeline
  tools/               grep_symbol, get_file, list_package
scripts/
  setup.sh             Full environment bootstrap
  health.sh            GPU + Ollama + Qdrant + ingest status
ideas/                 Project briefs (input to research)
artifacts/             Generated research + distillation output
```

---

## Key implementation choices

- **Vector store:** Qdrant on gRPC (`localhost:6334`, not the REST `6333`). 768-dim cosine vectors. Single collection `odin_k8s`. Deterministic FNV-64a point IDs make re-ingestion idempotent.
- **Embedding:** `nomic-embed-text`, chunks truncated to 6000 runes. Batch size 50 per Ollama call. AST-aware for `.go`, heading-aware for `.md`, line-chunked fallback otherwise.
- **Retrieval:** Top-15 for interactive queries. For distill's subsystem buckets, top-200 with per-prefix filtering to keep `pkg/scheduler` results from bleeding into `pkg/kubelet` analysis.
- **Grounding:** `qwen2.5:72b` via Ollama on local AMD GPU (ROCm). Same model powers both local-only `ask` and Gemini-orchestrated grounded queries. Zero source code leaves the machine.
- **Planning:** `gemini-3-flash-preview`. Chosen for fast structured function-calling, not for raw reasoning depth. Rate-limit aware: research loop retries up to 5 times with linear backoff (30s, 60s, 90s, 120s, 150s) on "high demand" errors.
- **Context discipline:** research-mode grounder responses are truncated to 1500 chars before being fed back into Gemini's history, preventing context blow-up on long autonomous runs. The full answer is still used locally by the grounder. Raw source files are never sent to Gemini, though retrieved snippets quoted in grounder answers are visible to the planner within that truncation.
- **Artifact discipline:** research loop injects a hard-nudge reminder every 5 rounds without a write, because LLMs left unsupervised will query forever and never produce output. On the committed Jaeger run this nudge fired 9 times, and each firing preceded a subsequent write — validating the mechanism's necessity, not just its presence.
- **Resumability:** every `distill` stage writes its artifact before marking the checkpoint, so a crashed stage leaves no stale checkpoint. Stage 2 additionally checkpoints per subsystem, so a crash after processing `apiserver` but before `controllers` picks up at `controllers` on restart.

---

## Honest limitations

- Distillation is Kubernetes-specific. The subsystem buckets in `internal/distill/stages.go` are hardcoded.
- `/root/repos` is hardcoded as the tool allowlist root in `cmd/odin/main.go` and `internal/tools/tools.go`. Runs fine on a Linux box where repos are synced there, anywhere else requires a small patch.
- No tests. `go build` and `go vet` are the only static checks.
- Go-only AST parsing. Other languages fall back to naive line chunking.
- Single-user, single-machine. No auth, no multi-tenancy, no hosted mode.

These are deliberate. The project's goal was to ship a working research pipeline for one engineer on one codebase, not a product.

---

## Stack

| Layer      | Tech                                       |
|------------|--------------------------------------------|
| Language   | Go 1.24                                    |
| Planner    | Google Gemini 3 Flash (function-calling)   |
| Grounder   | Qwen2.5 72B, quantised, local via Ollama   |
| Embedder   | nomic-embed-text (local, 768-dim)          |
| Vector DB  | Qdrant (gRPC, cosine distance)             |
| Runtime    | AMD GPU via ROCm (NVIDIA/CUDA also works)  |

---

## License

MIT.
