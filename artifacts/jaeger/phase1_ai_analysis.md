# Jaeger AI Phase 1 Analysis: MCP and LangChainGo

## Jaeger Model Context Protocol (MCP) Server
The existing AI integration is primarily located in `jaeger/internal/extension/jaegermcp`. This extension implements an MCP server, allowing LLM agents to interact with Jaeger trace data.

### Key Components:
- **Location**: `jaeger/internal/extension/jaegermcp/`
- **Capabilities**:
    - `get_services`: List services.
    - `get_operations`: List operations for a service.
    - `search_traces`: Search for traces based on criteria.
    - `get_trace`: Retrieve a specific trace.
    - `get_trace_topology`: Get a summarized view of a trace's service dependencies.
- **Implementation**:
    - `handler.go`: Defines the JSON-RPC handlers for the MCP protocol.
    - `server.go`: Manages the lifecycle of the MCP server as an OTel extension.

### Extension Registration:
The MCP server is registered as an OpenTelemetry Collector extension. It lists `jaegerquery` as a dependency, ensuring it can access the query engine.

## LangChainGo Integration
The repository includes a `langchaingo/` directory, which appears to be a local copy or a submodule of the `langchaingo` project. This provides the building blocks for:
- LLM interaction (OpenAI, Ollama, etc.).
- Chains and Agents.
- Prompt templates.

## Gap Analysis: Phase 2 "Skills" Framework
Phase 1 (MCP) provides **tools** for an LLM to call. Phase 2 aims to provide **skills** (pre-defined workflows/logic).
- **Current State**: Static set of tools in `jaegermcp`.
- **Target State**: Dynamic loading of "Skills" from configuration. A "Skill" would likely encompass:
    - A system prompt.
    - A set of required tools (from the MCP set).
    - Logic for multi-step reasoning.

## OpenTelemetry Collector Foundation
Jaeger v2 is built on the OpenTelemetry Collector.
- Files found in `jaeger/cmd/jaeger/` suggest a transition.
- The existence of `internal/extension/jaegermcp` and its `Dependencies()` method (returning `[]string{"jaegerquery"}`) confirms the use of the OTel extension architecture.

## Identified "Natural Language Search"
The project description mentions a "baseline AI assistant for natural language search". This likely uses the `search_traces` tool in the MCP server or a dedicated chain in `langchaingo`.
