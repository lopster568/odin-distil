Perfect. We’re going to document Odin like an internal design doc — clear, system-level, no fluff — so your coding agent understands what exists and what we’re extending.

This is the current Odin architecture based on your structure and behavior.

---

## ODIN – Current Architecture Overview

Odin is a CLI-based Retrieval-Augmented Generation (RAG) system designed to index large codebases and answer structural questions using a local LLM + vector store.

It is composed of:

- Ingestion pipeline
- Embedding layer
- Vector store (Qdrant)
- LLM interface (Ollama)
- Query agent
- CLI entrypoint
- Operational health tooling

The system follows this high-level data flow:

Source Code → Chunk → Embed → Store → Retrieve → Augment → LLM → Answer

---

## Directory-Level Architecture

```
cmd/odin/
internal/
    embedder/
    ingester/
    llm/
    query/
    store/
    tools/
scripts/
```

Each module has a single responsibility.

---

## 1. CLI Layer

Location:
`cmd/odin/main.go`

Responsibilities:

- Parse commands like:
  - `odin ask`

- Initialize dependencies
- Wire components together
- Act as orchestration entrypoint

This layer should contain zero business logic.

---

## 2. Ingestion Pipeline

Location:
`internal/ingester/`

Responsibilities:

- Read source repositories
- Split into chunks
- Attach metadata
- Generate embeddings
- Store vectors in Qdrant

Critical design assumption:
Embeddings are derived artifacts.
Source corpus is the real asset.

Important:
Metadata quality determines future distillation quality.

Each chunk should include metadata such as:

- repo name
- file path
- package name
- directory prefix
- optional language
- optional churn score

Without strong metadata, structural filtering becomes weak.

---

## 3. Embedding Layer

Location:
`internal/embedder/`

Responsibilities:

- Interface with embedding model (currently `nomic-embed-text`)
- Convert text chunks → vector embeddings
- Handle batching

Stateless and deterministic.

---

## 4. Vector Store Layer

Location:
`internal/store/`

Backed by:
Qdrant

Responsibilities:

- Insert vectors
- Filter by metadata
- Search by similarity
- Return relevant chunks

This layer is foundational for:

- `odin ask`
- upcoming `odin distill`

Filtering by metadata must be supported for subsystem-specific queries.

---

## 5. LLM Layer

Location:
`internal/llm/`

Backed by:
Ollama running `qwen2.5:72b`

Responsibilities:

- Accept structured prompts
- Stream responses
- Enforce response formatting (e.g., JSON)
- Handle retries

This layer must remain generic.

Distillation logic must not live here.

---

## 6. Query Agent

Location:
`internal/query/`

Responsibilities:

- Take user question
- Retrieve relevant chunks
- Construct augmented prompt
- Call LLM
- Return formatted answer

This is interactive mode (`odin ask`).

It is:

On-demand
Stateless
User-driven

---

## 7. Operational Layer

Location:
`scripts/health.sh`

Responsibilities:

- Verify GPU state
- Verify Ollama is running
- Verify Qdrant is running
- Display model list
- Show ingest job status
- Display vector counts

This gives observability into system state.

Important:
The system is currently environment-dependent and must be made reproducible before machine destruction.

---

# What Odin Is Right Now

Odin is:

A structured RAG engine specialized for large codebases.

It supports:

- Batch ingestion
- Vector search
- Augmented LLM reasoning
- Interactive querying

It does NOT yet support:

- Long-running architectural distillation
- Multi-stage synthesis
- Structured artifact generation
- Resume-safe batch intelligence jobs

That is what we are adding.

---

# Architectural Style

Odin follows layered architecture:

CLI
↓
Query / Ingest Orchestration
↓
Embedding + Vector Store
↓
LLM Reasoning
↓
Response

It is cleanly modular and Go-idiomatic.

There is no distributed complexity inside Odin.
It delegates heavy compute to Ollama + Qdrant.

---

# Design Philosophy

Current design assumptions:

- Embeddings are reproducible.
- Vector store is disposable.
- Models are swappable.
- Metadata is critical.
- LLM is stateless.
- Pipeline stages should be deterministic.

We now extend this with a new concept:

Distillation Jobs.

---

# Upcoming Extension: Distillation Layer

We are adding:

`internal/distill/`

This layer will:

- Run long-running batch jobs.
- Operate over vector store.
- Perform structured multi-pass reasoning.
- Persist intermediate artifacts.
- Be resumable.
- Generate reusable architectural intelligence.

It is NOT interactive.

It is pipeline-based.

---

# System Capabilities After Extension

Odin will then have two modes:

1. Interactive Mode
   `odin ask`

2. Batch Intelligence Mode
   `odin distill k8s`

Interactive = user-driven RAG
Distill = architecture extraction engine

Same storage. Same LLM. Different orchestration.

---

# Key Architectural Constraints Going Forward

- No monolithic prompts.
- All distillation stages must write intermediate artifacts.
- Each stage must be resumable.
- Metadata filtering must be reliable.
- JSON outputs must be validated.
- No logic leakage into LLM layer.
- Distill must be deterministic given same corpus.

---

# Conceptual Model

Odin is evolving from:

Search + Answer Tool

Into:

Architecture Pattern Compiler

That’s the direction.

---

Give this architecture summary to your coding agent.

If it understands this structure clearly, it can implement distillation cleanly without entangling responsibilities.

And that separation discipline — more than GPU power — is what keeps systems scalable instead of collapsing into spaghetti.
