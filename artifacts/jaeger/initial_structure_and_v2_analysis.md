# Jaeger v2 and AI Integration - Initial Research

## Repository Overview
The repository appears to contain two major components relevant to the project:
1.  `jaeger/`: The main Jaeger source code.
2.  `langchaingo/`: A Go implementation of LangChain, likely used for the AI orchestration layer.

## Jaeger v2 Architecture
Jaeger v2 is built on the OpenTelemetry (OTel) Collector framework. The main entry points and component configurations are located under `jaeger/cmd/jaeger/`.

### Key Directories:
- `jaeger/cmd/jaeger/`: Contains the binary entry points.
- `jaeger/internal/extension/`: Contains OTel Collector extensions. This is a primary candidate for the "Skills Engine" implementation.
- `jaeger/internal/extension/jaegermcp/`: Implements a Model Context Protocol (MCP) server. This suggests existing work on AI integration (likely Phase 1).

## AI Components (Phase 1 Status)
The project description mentions a baseline AI assistant. Evidence of this is found in:
- `jaeger/internal/extension/jaegermcp`: Model Context Protocol server. MCP is a standard for connecting LLMs to data sources.
- `langchaingo/`: Included as a dependency/module to facilitate LLM interactions.

## Skills Engine - Implementation Hypothesis
The "Skills Engine" needs to be a dynamic backend framework in Go. Given the Jaeger v2 architecture:
- It will likely be implemented as an **extension** within the OpenTelemetry-based Jaeger binary.
- It should probably live in a new package under `jaeger/internal/extension/ai_skills` or similar.
- It will need to interface with the `jaegermcp` or the Jaeger Query API to retrieve trace data for analysis.

## Next Research Steps
1.  Verify how Jaeger v2 uses the OpenTelemetry Collector (inspect `jaeger/cmd/jaeger/main.go`).
2.  Analyze the `jaegermcp` extension to see how it currently exposes Jaeger data to LLMs.
3.  Locate the "Natural Language Search" implementation from Phase 1.
4.  Identify the configuration loading mechanism in Jaeger v2 to support "Self-Service Skills" via config files.
