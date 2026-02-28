# Phase 2: AI-Powered Trace Analysis "Skills" Framework - Final Research Report

## Executive Summary
Jaeger v2 provides a solid foundation for the AI-Powered Trace Analysis Phase 2. The project will transition from a static AI assistant to a flexible, self-service "Skills" framework by implementing a new OpenTelemetry Collector extension that orchestrates `langchaingo` agents. The system will leverage the existing Model Context Protocol (MCP) toolset for trace data access and provide a user-programmable layer via YAML-based configuration.

---

## Component Map: What to Build & Where

| Component | Responsibility | File Path / Package |
| :--- | :--- | :--- |
| **Skills Engine Extension** | OTel extension that manages skill lifecycle, config loading, and agent execution. | `jaeger/internal/extension/aiskills/` |
| **Skill Registry** | In-memory store of skills loaded from YAML. | `jaeger/internal/extension/aiskills/registry.go` |
| **Agent Executor** | Orchestrates `langchaingo` agents, mapping skills to tools. | `jaeger/internal/extension/aiskills/executor.go` |
| **MCP Tool Bridge** | Adapts `jaegermcp` tool handlers for `langchaingo`. | `jaeger/internal/extension/aiskills/bridge.go` |
| **GraphQL Schema** | Defines `Skill` and `AnalysisResult` types. | `jaeger/query/graphql/schema.graphql` |
| **GraphQL Resolvers** | Resolves AI queries by calling the Skills Engine. | `jaeger/query/graphql/resolver.go` |
| **Skills UI** | Enhances the `Assistant` React component to support skill selection and reasoning visualization. | `jaeger/cmd/jaeger-ui/src/components/Assistant/` |

---

## Key Interfaces

### 1. Skills Engine Extension
Must satisfy the OTel Collector extension interface and depend on `jaeger_query` and `jaeger_mcp`.
```go
package aiskills

import "go.opentelemetry.io/collector/component"

type Extension interface {
	component.Component
	GetAvailableSkills() []Skill
	ExecuteSkill(ctx context.Context, skillID string, traceID string, query string) (Result, error)
}
```

### 2. User-Defined Skill (YAML)
Loaded dynamically by the Skills Engine.
```yaml
id: n_plus_one_detector
name: "Detect N+1 Queries"
description: "Finds patterns of many small queries to the same database."
system_prompt: "You are a database performance expert. Analyze the provided trace topology..."
tools:
  - get_trace_topology
  - search_traces
```

### 3. LangChainGo Integration
The `aiskills` extension will bridge `jaegermcp` tools to `langchaingo` agents.
```go
// bridge.go
func (b *Bridge) GetTool(toolName string) tools.Tool {
    // Bridges MCP tool handler to langchaingo.tools.Tool
}
```

---

## Technical Risks & Blockers

1. **Circular Extension Dependency**: If `jaeger_query` needs the Skills Engine for its GraphQL resolvers, but the Skills Engine needs `jaeger_query` for trace data, a dependency cycle may occur.
   - *Mitigation*: Use an interface-based "Registry" in `jaeger_query` that the `aiskills` extension registers its resolvers with at runtime.
2. **GraphQL Generation (`gqlgen`)**: Jaeger v2 uses `gqlgen`. Any schema change requires running `make generate-graphql`. The Skills Engine must be integrated into this build process.
3. **Long-Running AI Tasks**: AI reasoning can exceed typical HTTP timeouts. 
   - *Mitigation*: The `runSkill` GraphQL query should either support a long-polling status or be converted to a GraphQL Subscription.
4. **Local-First (Ollama)**: While `langchaingo` supports Ollama, performance on local hardware varies. The UI must handle slow model responses gracefully.

---

## Cross-References
- [Initial structure and v2 analysis](./initial_structure_and_v2_analysis.md)
- [Phase 1 AI Analysis](./phase1_ai_analysis.md)
- [Extension and Skills Framework](./extension_and_skills_framework.md)
- [Integration Strategy](./integration_strategy.md)
- [Frontend and API Integration](./frontend_and_api_integration.md)
- [API and Extension Interoperability](./api_and_extension_interop.md)
- [Skills Engine Implementation Plan](./skills_engine_implementation_plan.md)
- [Extension Lookup and API Mechanics](./extension_lookup_and_api.md)
