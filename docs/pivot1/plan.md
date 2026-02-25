# Pivot 1 — `odin distill k8s`

## Objective

Extend Odin from an interactive Q&A tool into a **long-running architecture distillation engine**.
The output is not chat responses. It is structured, reusable architectural intelligence written to disk.

---

## What Needs to Be Built

### New package: `internal/distill/`

Core of the pivot. Responsible for:

- Defining the 6-stage pipeline
- Bucketing Qdrant results by Kubernetes subsystem
- Running multi-pass LLM reasoning per stage
- Persisting intermediate artifacts with checkpointing
- Resuming from last completed stage

### New CLI command: `odin distill k8s`

Wire into `cmd/odin/main.go` under a new `case "distill"` branch.
Pass `store`, `llmClient`, and a configurable `artifactsDir` (`artifacts/k8s/` by default).

### New output directory: `artifacts/k8s/`

All stages write here. Files must be present and valid before the next stage runs.

---

## Ingestion Prerequisite (fix before distill)

The distillation pipeline depends on **path-prefix filtering** of Qdrant results to bucket chunks by subsystem.
Current `store.Search()` does a pure vector search — no payload filter.

**Required change to `internal/store/store.go`:**
Add a `SearchWithFilter(ctx, vector, limit, pathPrefix string)` method that passes a Qdrant payload filter on the `filepath` field.

Without this, subsystem bucketing falls back to keyword post-filtering in Go, which is acceptable for a first pass but should be noted as a limitation.

Also verify that `ingester` metadata includes:

- `repo` (currently absent — derive from path prefix or add as a parameter to `ingester.Walk`)
- `filepath`, `package` (already present)
- `dirprefix` (first 2-3 path segments — useful for bucket matching)

---

## The 6-Stage Pipeline

Each stage: reads from Qdrant or prior artifact → calls LLM → validates output → writes artifact → marks stage done.

### Stage 1 — Bucket Retrieval

**Input:** Qdrant vector store  
**Output:** in-memory chunk lists per bucket (not persisted — ephemeral)

Hardcoded buckets and their path-prefix matchers:

| Bucket        | Path prefix                                                 |
| ------------- | ----------------------------------------------------------- |
| `apiserver`   | `pkg/apiserver`, `staging/src/k8s.io/apiserver`             |
| `controllers` | `pkg/controller`                                            |
| `scheduler`   | `pkg/scheduler`                                             |
| `kubelet`     | `pkg/kubelet`                                               |
| `storage`     | `pkg/storage`, `staging/src/k8s.io/apiextensions-apiserver` |
| `admission`   | `plugin/pkg/admission`                                      |

Retrieve top **200 chunks** per bucket via vector search seeded with a bucket-specific probe query.
Post-filter by path prefix in Go to enforce bucket isolation.

---

### Stage 2 — Responsibility Extraction

**Input:** chunks per bucket  
**Output:** `artifacts/k8s/responsibilities.json`

For each bucket, batch chunks into LLM calls (max ~4000 runes of context per call).
Prompt asks for structured JSON:

```json
{
  "component": "controllers",
  "core_responsibility": "...",
  "state_managed": ["..."],
  "event_sources": ["..."],
  "reconciliation_logic": "...",
  "failure_handling": "..."
}
```

Checkpoint after each bucket. If `responsibilities.json` already contains a bucket's entry, skip it (resume support).

Validate JSON before writing. Fail fast on malformed output — do not silently write garbage.

---

### Stage 3 — Pattern Mining

**Input:** `artifacts/k8s/responsibilities.json`  
**Output:** `artifacts/k8s/patterns.json`

Feed all responsibilities back to the LLM in a single structured call.
Ask it to extract:

- Recurring architectural patterns
- Systemic invariants (e.g., level-triggered reconciliation)
- Extensibility mechanisms (admission webhooks, CRDs, etc.)
- Coupling strategies between subsystems

Output format: JSON array of pattern objects with `name`, `description`, `examples` fields.

---

### Stage 4 — Reconciliation Loop Abstraction

**Input:** `artifacts/k8s/responsibilities.json` (controllers bucket only)  
**Output:** `artifacts/k8s/control_loops.md`

Target: `controllers` bucket.
Ask the model to:

- Generalize the reconciliation loop into a pseudocode template
- Identify idempotency guarantees
- Describe retry and failure semantics
- Note where `workqueue` and `informer` patterns appear

Markdown output — no JSON needed here.

---

### Stage 5 — Friction & Opportunity Mining

**Input:** `artifacts/k8s/patterns.json` + `artifacts/k8s/responsibilities.json`  
**Output:** `artifacts/k8s/friction.json`

Ask the model to identify:

- High cognitive load areas
- Likely contributor pain points
- High-churn or tightly coupled subsystems
- GSoC-suitable improvement opportunities

Output: JSON array with `area`, `friction_type`, `rationale`, `opportunity` fields.

---

### Stage 6 — Study Topic Generation

**Input:** all prior artifacts  
**Output:** `artifacts/k8s/study_topics.md`

Generate 25–30 deep study topics that:

- Reference real Kubernetes mechanisms (cite file paths where possible)
- Generalize to distributed systems principles
- Are interview-relevant

Markdown list with brief rationale per topic.

---

## Implementation Order

1. `internal/store`: add `SearchWithFilter` (or path-prefix post-filter helper)
2. `internal/distill/distill.go`: `Distiller` struct, `Run(ctx)` entry point, stage orchestration
3. `internal/distill/stages.go`: one function per stage, each returning an error
4. `internal/distill/checkpoint.go`: read/write a `checkpoint.json` in `artifacts/k8s/` to track completed stages
5. `internal/distill/prompt.go`: prompt templates for each stage (keep prompts out of stage logic)
6. `cmd/odin/main.go`: wire `case "distill"` → `distill.New(llmClient, st, artifactsDir).Run(ctx)`

---

## Engineering Constraints (non-negotiable)

- **No monolithic prompts.** Each stage gets its own bounded context window.
- **All stages write intermediate artifacts.** Nothing lives only in memory across stage boundaries.
- **Resume support.** Check for existing artifact before running a stage. `checkpoint.json` tracks which stages completed.
- **JSON validation.** Use `encoding/json.Unmarshal` to validate before writing any `.json` artifact.
- **No logic in `internal/llm/`.** LLM layer stays a dumb transport. Prompt construction lives in `internal/distill/prompt.go`.
- **Log stage progress** to stdout with clear stage names and timing.
- **Fail fast on malformed LLM output** — return an error, do not silently save bad data.

---

## Expected Output Structure

```
artifacts/k8s/
├── checkpoint.json       ← tracks completed stages
├── responsibilities.json
├── patterns.json
├── control_loops.md
├── friction.json
└── study_topics.md
```

---

## Success Criteria

After a full run of `odin distill k8s`, the following must exist and be non-trivially populated:

1. Subsystem responsibility map (`responsibilities.json`)
2. Extracted distributed systems patterns (`patterns.json`)
3. Reconciliation loop template (`control_loops.md`)
4. Contributor friction and GSoC opportunities (`friction.json`)
5. Study roadmap derived from real architecture (`study_topics.md`)

If all five exist with substantive content, the pipeline worked.
