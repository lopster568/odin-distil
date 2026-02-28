# Jaeger v2 Extension Framework & Skills Engine Integration

## OpenTelemetry Collector Extension Pattern
Jaeger v2 (built on OTel Collector) uses a standardized factory pattern to register and instantiate components. The "Skills Engine" should be implemented as a new extension to ensure it is native to the Jaeger binary and can easily access other services like the query engine.

### Reference Implementation: `jaegermcp`
The `jaegermcp` extension (`jaeger/cmd/jaeger/internal/extension/jaegermcp`) provides a perfect template:
- **`factory.go`**: Defines the `extension.Factory` using `extension.NewFactory`.
- **`config.go`**: Defines the YAML configuration structure.
- **`server.go` / `handler.go`**: Implement the actual logic (MCP in this case).

### Proposed Extension Structure for Skills Engine
The framework should likely reside in `jaeger/internal/extension/aiskills/` (or similar).

#### 1. Configuration (`config.go`)
The "Self-Service" aspect requires loading skill definitions from YAML.
```go
type SkillConfig struct {
    Name        string   `yaml:"name"`
    Description string   `yaml:"description"`
    SystemPrompt string  `yaml:"system_prompt"`
    Tools       []string `yaml:"tools"` // e.g., ["search_traces", "get_trace_topology"]
}

type Config struct {
    SkillsDir string        `yaml:"skills_dir"` // Directory to watch for .yaml skills
    Model     ModelConfig   `yaml:"model"`      // LLM settings (Ollama, etc.)
    Skills    []SkillConfig `yaml:"skills"`     // Inline skills
}
```

#### 2. Factory (`factory.go`)
Registers the extension with the Jaeger binary.
```go
const ComponentType = "ai_skills"

func NewFactory() extension.Factory {
    return extension.NewFactory(
        ComponentType,
        createDefaultConfig,
        createExtension,
        component.StabilityLevelAlpha,
    )
}
```

#### 3. Dependencies
Like `jaegermcp`, the Skills Engine will depend on the `jaegerquery` extension to interact with trace data.
```go
func (s *skillsEngine) Dependencies() []string {
    return []string{"jaegerquery"}
}
```

## AI Orchestration with LangChainGo
The `langchaingo` package in the root is ready for use. The Skills Engine will:
1. Load YAML definitions.
2. Initialize LangChainGo `tools` based on the skill requirements.
3. Construct an `Agent` or `Chain` for each skill.
4. Expose an API (or extend the MCP server) to trigger these skills.

## UI Integration Point
The Jaeger React UI will need to:
1. Query the Skills Engine for available skills.
2. Render a "Skills" or "Analyze" dropdown/interface.
3. Send requests to the backend with trace context.
