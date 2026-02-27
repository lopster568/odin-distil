package orchestrator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"odin/internal/query"
)

// ─── Gemini API types ────────────────────────────────────────────────────────

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	Tools             []geminiTool    `json:"tools,omitempty"`
	ToolConfig        *toolConfig     `json:"tool_config,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text         string        `json:"text,omitempty"`
	FunctionCall *functionCall `json:"functionCall,omitempty"`
	FunctionResp *functionResp `json:"functionResponse,omitempty"`
    	Thought          bool          `json:"thought,omitempty"`
    	ThoughtSignature string        `json:"thoughtSignature,omitempty"`
}

type functionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type functionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type geminiTool struct {
	FunctionDeclarations []functionDecl `json:"function_declarations"`
}

type functionDecl struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  schemaParam `json:"parameters"`
}

type schemaParam struct {
	Type       string              `json:"type"`
	Properties map[string]schemaProp `json:"properties"`
	Required   []string            `json:"required"`
}

type schemaProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type toolConfig struct {
	FunctionCallingConfig struct {
		Mode string `json:"mode"` // AUTO | ANY | NONE
	} `json:"function_calling_config"`
}

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ─── Orchestrator ────────────────────────────────────────────────────────────

type Orchestrator struct {
	apiKey  string
	agent   *query.Agent
	model   string
	history []geminiContent
}

func New(agent *query.Agent) (*Orchestrator, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}
	return &Orchestrator{
		apiKey: key,
		agent:  agent,
		model:  "gemini-3-flash-preview",
	}, nil
}

func (o *Orchestrator) ClearHistory() {
	o.history = nil
	o.agent.ClearHistory()
}

func (o *Orchestrator) Chat(ctx context.Context, userMessage string) (string, error) {
	// Append user message to history
	o.history = append(o.history, geminiContent{
		Role:  "user",
		Parts: []geminiPart{{Text: userMessage}},
	})

	// Agentic loop — Gemini may call query_codebase multiple times
	for range 5 { // max 5 tool call rounds
		resp, err := o.callGemini(ctx)
		if err != nil {
			return "", err
		}

		if len(resp.Candidates) == 0 {
			return "", fmt.Errorf("empty response from Gemini")
		}

		candidate := resp.Candidates[0]
		o.history = append(o.history, candidate.Content)

		// Check if Gemini wants to call a tool
		toolCalls := extractToolCalls(candidate.Content)
		if len(toolCalls) == 0 {
			// No tool calls — extract final text answer
			return extractText(candidate.Content), nil
		}

		// Execute each tool call and collect results
		var toolResults []geminiPart
		for _, tc := range toolCalls {
			result := o.executeTool(ctx, tc)
			toolResults = append(toolResults, geminiPart{
				FunctionResp: &functionResp{
					Name:     tc.Name,
					Response: map[string]any{"result": result},
				},
			})
			fmt.Printf("  [tool] query_codebase(%q) → %d chars\n",
				tc.Args["question"], len(result))
		}

		// Feed tool results back to Gemini
		o.history = append(o.history, geminiContent{
			Role:  "tool",
			Parts: toolResults,
		})
	}

	return "", fmt.Errorf("exceeded max tool call rounds")
}

func (o *Orchestrator) callGemini(ctx context.Context) (*geminiResponse, error) {
	system := geminiContent{
		Parts: []geminiPart{{Text: `You are Odin, a senior distributed systems and Go engineer.
You have access to a tool called query_codebase that searches an indexed codebase 
(Kubernetes, Jaeger, LangChainGo source trees) and returns grounded answers with file paths.

Guidelines:
- Use query_codebase for any question that requires knowledge of specific code, file locations, interfaces, or implementation details
- You may call query_codebase multiple times with refined questions to build a complete answer
- Synthesize tool results into clear, technical, actionable answers
- Always cite file paths and function names when referencing code
- Be direct and precise — the user is an engineer doing serious work`}},
	}

	tools := []geminiTool{{
		FunctionDeclarations: []functionDecl{{
			Name:        "query_codebase",
			Description: "Search the indexed Jaeger, LangChainGo, and Kubernetes source trees. Returns grounded answers with exact file paths, function signatures, and type definitions. Call this whenever the question involves: how something is implemented, where an interface is defined, what a package exports, how components are wired together, or what patterns are used. Prefer multiple focused calls over one broad call.",
			Parameters: schemaParam{
				Type: "object",
				Properties: map[string]schemaProp{
					"question": {
						Type:        "string",
						Description: "The technical question to answer about the codebase",
					},
				},
				Required: []string{"question"},
			},
		}},
	}}

	tc := &toolConfig{}
	tc.FunctionCallingConfig.Mode = "AUTO"

	req := geminiRequest{
		SystemInstruction: &system,
		Contents:          o.history,
		Tools:             tools,
		ToolConfig:        tc,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		o.model, o.apiKey,
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w\nraw: %s", err, string(respBody))
	}

	if gemResp.Error != nil {
		return nil, fmt.Errorf("gemini error: %s", gemResp.Error.Message)
	}

	return &gemResp, nil
}

func (o *Orchestrator) executeTool(ctx context.Context, tc *functionCall) string {
	question, _ := tc.Args["question"].(string)
	if question == "" {
		return "error: empty question"
	}
	result, err := o.agent.Ask(ctx, question)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func extractToolCalls(content geminiContent) []*functionCall {
	var calls []*functionCall
	for _, part := range content.Parts {
		if part.FunctionCall != nil {
			calls = append(calls, part.FunctionCall)
		}
	}
	return calls
}

func extractText(content geminiContent) string {
	var sb strings.Builder
	for _, part := range content.Parts {
		if part.Text != "" && !part.Thought {
			sb.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(sb.String())
}

// ─── CLI runner ──────────────────────────────────────────────────────────────

func RunChat(ctx context.Context, agent *query.Agent) error {
	orch, err := New(agent)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║   ODIN CHAT — Gemini × Qwen2.5-72B       ║")
	fmt.Println("║   Gemini orchestrates, Qwen grounds       ║")
	fmt.Println("╠═══════════════════════════════════════════╣")
	fmt.Println("║  Commands: /clear  /quit                  ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()

	for {
		fmt.Print(">>> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch input {
		case "/clear":
			orch.ClearHistory()
			fmt.Println("  conversation cleared")
			continue
		case "/quit", "/exit":
			fmt.Println("bye")
			return nil
		}

		fmt.Println()
		answer, err := orch.Chat(ctx, input)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		fmt.Println(answer)
		fmt.Println()
	}

	return nil
}
