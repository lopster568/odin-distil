# Repository Structure and AI Integration Strategy

## Multi-Module Layout
The repository is a monorepo containing at least two distinct Go modules:
1. `jaeger/`: The main Jaeger tracing system (module `github.com/jaegertracing/jaeger`).
2. `langchaingo/`: A fork or copy of the LangChainGo library (module `github.com/tmc/langchaingo`).

### Implications for Phase 2
Since `jaeger/go.mod` does not currently depend on `langchaingo/`, the first step of the project will involve:
- Adding a `replace` directive or using a Workspace (`go.work`) to allow the Jaeger module to use the local `langchaingo` module.
- Importing `github.com/tmc/langchaingo` into the new Jaeger AI Skills extension.

## Jaeger v2 Extension Mechanism
All AI capabilities in Jaeger v2 are being built as OpenTelemetry Collector extensions.
- **Current Extension**: `jaegermcp` (Model Context Protocol).
- **Target Extension**: A new `aiskills` extension that will orchestrate LangChainGo agents.

### Skills Engine Design (Go Interfaces)
The Skills Engine must dynamically load YAML-defined skills. Based on research, the following interface structure is proposed:

```go
// Package internal/extension/aiskills

type Skill struct {
    ID           string   `yaml:"id"`
    Description  string   `yaml:"description"`
    SystemPrompt string   `yaml:"system_prompt"`
    Tools        []string `yaml:"tools"` // References to MCP tools
}

type SkillsRegistry interface {
    RegisterSkill(s Skill) error
    GetSkill(id string) (Skill, bool)
    ListSkills() []Skill
}

type Executor interface {
    // Execute runs a skill given a query and trace context
    Execute(ctx context.Context, skillID string, query string, traceID string) (string, error)
}
```

## Integration with jaegermcp
The `jaegermcp` extension already implements tool handlers for:
- Trace Search
- Trace Retrieval
- Topology Generation

The `aiskills` engine should consume these handlers as "Tools" for its LangChainGo agents. This can be achieved by making `aiskills` depend on `jaegermcp` in the OTel extension lifecycle.

## Local-First Support (Ollama)
The `langchaingo` module contains an `llms/ollama` package. This confirms that local model support is architecturally ready and just needs to be wired into the `aiskills` configuration.
