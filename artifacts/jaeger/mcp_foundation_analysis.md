# Deep Dive: jaegermcp - The AI Foundation

The `jaegermcp` extension (`jaeger/internal/extension/jaegermcp`) implements the Model Context Protocol (MCP), which acts as the interface between Language Models and Jaeger. Phase 2 "Skills" will build upon this by composing these tools into higher-level workflows.

## Current MCP Tools (`handler.go`)
The following tools are already implemented, providing the "primitives" for any AI skill:

- `get_services`: Returns a list of all services.
- `get_operations`: Returns operations for a specific service.
- `search_traces`: Search traces (supports service, operation, tags, time range, limit).
- `get_trace`: Retrieves a full trace by ID.
- `get_trace_topology`: Provides a summarized graph of service dependencies for a trace.

## Server Implementation (`server.go`)
The MCP server is an HTTP server that speaks JSON-RPC. 
- It implements `extension.Extension` and `component.Component`.
- It uses a `jaegerquery.Extension` to interact with the backend.
- It is registered in the Jaeger binary via `jaeger/cmd/jaeger/internal/extension/jaegermcp/factory.go`.

## Skills Framework: The Next Layer
The proposed Phase 2 framework will likely sit "above" or "beside" this MCP server.
- **Integration Option A**: The Skills Engine is a separate OTel extension that calls the MCP handlers internally.
- **Integration Option B**: The MCP server is extended to support "Skills" as high-level tools.
- **Integration Option C**: A new `jaeger_ai` extension is created that orchestrates `LangChainGo` agents, using the MCP tools as their capabilities.

## LangChainGo Relationship
Although `jaeger/go.mod` might not reference it, the repository structure contains `langchaingo/` at the root. This suggests a monorepo approach where Jaeger and its AI components are co-developed. 

### To-Do:
- Investigate `langchaingo/` content to see if there are Jaeger-specific chains or tools.
- Verify how the Jaeger UI communicates with the MCP server (is it direct from the browser or through a proxy?).
