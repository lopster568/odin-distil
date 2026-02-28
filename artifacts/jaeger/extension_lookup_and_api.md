# Extension Lookup and API Integration Mechanics

## Extension Discovery via `component.Host`
In the Jaeger v2 (OTel-based) architecture, extensions interact by retrieving one another from the `component.Host`.

### Pattern: Retrieving `jaeger_query`
The `jaegermcp` extension demonstrates how to obtain a reference to the query engine:
```go
// From jaeger/cmd/jaeger/internal/extension/jaegermcp/server.go
queryExt, err := jaegerquery.GetExtension(host)
if err != nil {
    return nil, fmt.Errorf("failed to get jaegerquery extension: %w", err)
}
```

The `jaegerquery` extension exposes a helper function for this purpose:
```go
// From jaeger/cmd/jaeger/internal/extension/jaegerquery/extension.go
func GetExtension(host component.Host) (Extension, error) {
    // Iterates through host.GetExtensions() and returns the one matching jaegerquery.componentType
}
```

## GraphQL Implementation Status
There is a slight ambiguity in the codebase regarding the GraphQL library:
- `jaeger/query/graphql/server.go` references `github.com/graphql-go/handler`.
- Other parts and search results reference `github.com/99designs/gqlgen`.
- **Finding**: Most modern Jaeger v2 components seem to favor `gqlgen` for its code generation capabilities (as seen in `jaeger/query/graphql/schema.graphql`).

### Schema Extensibility Challenge
The current `NewServer` implementation in `jaeger/query/graphql/server.go` loads a static schema via `LoadSchema()`. To support the "Skills Engine", we need to:
1.  **Static Approach**: Modify the core `schema.graphql` and `resolver.go` in the Jaeger repository to include AI skill fields.
2.  **Dynamic Approach**: Modify `jaeger_query` to allow other extensions to register "Sub-Resolvers" or "Schema Extensions" at runtime.

Given the project's goal of "Self-Service Skills", a **Static Approach** for the API structure (defining what a "Skill" and "AnalysisResult" look like) combined with a **Dynamic Approach** for the skill logic (loading YAML files) is the most robust path.

## The Bridge: MCP to LangChainGo
The `jaegermcp` extension provides the raw handlers for trace data. The `ai_skills` extension should wrap these handlers to create `langchaingo` tools:

```go
// Bridge example
func (b *MCPBridge) GetTraceTool() tools.Tool {
    return tools.Tool{
        Name: "get_trace",
        Description: "Retrieve a full trace by ID",
        Execute: func(ctx context.Context, input string) (string, error) {
            // Calls jaegermcp.handleGetTrace or jaegerquery.GetTrace
        },
    }
}
```
This allows the AI agent to use the exact same logic that the MCP server exposes to external LLMs.
