# Skills Engine Architecture & Implementation Plan

The "Skills Engine" will be a new OpenTelemetry Collector extension in Jaeger v2 that orchestrates LangChainGo agents to perform domain-specific trace analysis.

## 1. Extension Definition
**Path**: `jaeger/internal/extension/aiskills/`

The extension will implement the `extension.Extension` interface and depend on `jaeger_query` and `jaeger_mcp`.

```go
// factory.go
func NewFactory() extension.Factory {
    return extension.NewFactory(
        "ai_skills",
        createDefaultConfig,
        createExtension,
        component.StabilityLevelAlpha,
    )
}

// extension.go
type SkillsEngine struct {
    config *Config
    registry *Registry
    executor *Executor
}
```

## 2. Dynamic Skill Loading (Self-Service)
Skills will be defined in YAML files. The engine will watch a configured directory for these files.

**Example Skill Definition (`critical_path.yaml`):**
```yaml
id: analyze_critical_path
description: "Identifies the slowest spans in a trace that contribute to total latency."
system_prompt: |
  You are an expert at distributed tracing. 
  Given a trace topology, identify the critical path.
  Focus on spans where duration is a high percentage of the parent's duration.
tools:
  - get_trace_topology
  - get_trace
```

## 3. Tool Orchestration
The engine will use `langchaingo` agents. It will wrap existing `jaegermcp` handlers as `langchaingo.Tool` objects.

```go
func (e *Executor) getTools(skill Skill) []tools.Tool {
    var agentTools []tools.Tool
    for _, toolName := range skill.Tools {
        // Map "get_trace" to the handler from jaegermcp
        agentTools = append(agentTools, e.mcpBridge.GetTool(toolName))
    }
    return agentTools
}
```

## 4. GraphQL API Extension
To expose these skills to the Jaeger UI, the `jaeger_query` GraphQL schema must be extended.

**File**: `jaeger/query/graphql/schema.graphql`
```graphql
extend type Query {
    availableSkills: [Skill!]!
    runSkill(skillId: String!, traceId: String!): AnalysisResult!
}

type Skill {
    id: String!
    description: String!
}

type AnalysisResult {
    content: String!
    steps: [String!]
}
```

**Implementation**: 
- Add these fields to `schema.graphql`.
- Run `make generate-graphql`.
- Implement the resolvers in `jaeger/query/graphql/resolver.go`. The resolver will delegate to the `aiskills` extension instance.

## 5. Local Model Support
The configuration will allow specifying an Ollama endpoint.

```yaml
ai_skills:
  model:
    provider: ollama
    endpoint: "http://localhost:11434"
    model_name: "llama3"
  skills_dir: "/etc/jaeger/skills.d"
```

## 6. UI Visualization
The `Assistant` component in `jaeger/cmd/jaeger-ui/src/components/Assistant/` will be updated to:
- Call `availableSkills` on mount.
- Provide a dropdown to select a skill.
- Call `runSkill` and display the `content` (Markdown) and `steps` (as a collapsible list or "reasoning" timeline).
