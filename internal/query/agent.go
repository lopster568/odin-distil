package query

import (
	"context"
	"fmt"
	"strings"

	"odin/internal/llm"
	"odin/internal/store"
	"odin/internal/tools"
)

type Message struct {
	Role    string // "user" | "assistant"
	Content string
}

type Agent struct {
	llm     *llm.Client
	store   *store.Store
	history []Message
	repoRoot string
}

func NewAgent(l *llm.Client, s *store.Store, repoRoot string) *Agent {
	return &Agent{llm: l, store: s, repoRoot: repoRoot}
}

func (a *Agent) ClearHistory() {
	a.history = nil
}

func (a *Agent) Ask(ctx context.Context, question string) (string, error) {
	// 1. vector search
	vec, err := a.llm.Embed(ctx, question)
	if err != nil {
		return "", fmt.Errorf("embed: %w", err)
	}
	results, err := a.store.Search(ctx, vec, 15)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}

	// 2. build system prompt
	system := `You are Odin, an expert Kubernetes and Go engineer with access to the full Kubernetes source tree.

You have the following tools available. To use a tool, output a line in this exact format:
TOOL: tool_name(argument)

Available tools:
- TOOL: grep_symbol(SymbolName)     — find usages of a function/type across repos
- TOOL: get_file(/path/to/file.go)  — read a specific file for more context  
- TOOL: list_package(/path/to/pkg)  — list all exported symbols in a package

Rules:
- Use tools when the retrieved context is insufficient to answer fully
- Use at most 3 tool calls per response
- After tool results, synthesize a complete answer
- Always cite file paths when referencing code
- Be direct and technical — the user is learning Kubernetes for GSoC`

	// 3. build context from vector results
	var ctxSb strings.Builder
	ctxSb.WriteString("=== RETRIEVED CONTEXT ===\n")
	for i, r := range results {
		ctxSb.WriteString(fmt.Sprintf("\n[%d] %s | %s\n", i+1, r.FilePath, r.Symbol))
		ctxSb.WriteString(r.Text)
		ctxSb.WriteString("\n")
	}

	// 4. build conversation history
	var histSb strings.Builder
	if len(a.history) > 0 {
		histSb.WriteString("=== CONVERSATION HISTORY ===\n")
		// keep last 6 exchanges to avoid context overflow
		start := 0
		if len(a.history) > 6 {
			start = len(a.history) - 6
		}
		for _, m := range a.history[start:] {
			histSb.WriteString(fmt.Sprintf("[%s]: %s\n\n", strings.ToUpper(m.Role), m.Content))
		}
	}

	// 5. first generation — may contain tool calls
	prompt := fmt.Sprintf("%s\n\n%s\n\n%s\n\n=== CURRENT QUESTION ===\n%s\n\n=== YOUR RESPONSE ===\n",
		system, histSb.String(), ctxSb.String(), question)

	response, err := a.llm.Generate(ctx, "qwen2.5:72b", prompt)
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}

	// 6. execute tool calls if present
	finalResponse, err := a.executeTools(ctx, response, prompt)
	if err != nil {
		finalResponse = response // fallback to original if tools fail
	}

	// 7. save to history
	a.history = append(a.history, Message{Role: "user", Content: question})
	a.history = append(a.history, Message{Role: "assistant", Content: finalResponse})

	return finalResponse, nil
}

func (a *Agent) executeTools(ctx context.Context, response, originalPrompt string) (string, error) {
	lines := strings.Split(response, "\n")
	var toolCalls []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "TOOL:") {
			toolCalls = append(toolCalls, strings.TrimSpace(line))
		}
	}
	if len(toolCalls) == 0 {
		return response, nil
	}

	// execute tools
	var toolResults strings.Builder
	toolResults.WriteString("\n=== TOOL RESULTS ===\n")
	for _, call := range toolCalls {
		result := a.executeTool(call)
		toolResults.WriteString(fmt.Sprintf("\n%s → %s\n%s\n",
			call, result.Tool, result.Output))
		if result.Error != "" {
			toolResults.WriteString(fmt.Sprintf("ERROR: %s\n", result.Error))
		}
	}

	// second generation with tool results
	finalPrompt := originalPrompt + response + toolResults.String() +
		"\n\n=== FINAL ANSWER (incorporate tool results above) ===\n"
	return a.llm.Generate(ctx, "qwen2.5:72b", finalPrompt)
}

func (a *Agent) executeTool(call string) tools.Result {
	// parse: TOOL: name(arg)
	call = strings.TrimPrefix(call, "TOOL:")
	call = strings.TrimSpace(call)
	paren := strings.Index(call, "(")
	if paren < 0 {
		return tools.Result{Error: "invalid tool call format"}
	}
	name := strings.TrimSpace(call[:paren])
	arg := strings.TrimSuffix(strings.TrimPrefix(call[paren:], "("), ")")

	switch name {
	case "grep_symbol":
		return tools.GrepSymbol(arg, a.repoRoot)
	case "get_file":
		return tools.GetFile(arg)
	case "list_package":
		return tools.ListPackage(arg)
	default:
		return tools.Result{Error: fmt.Sprintf("unknown tool: %s", name)}
	}
}
