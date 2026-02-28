package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const researchSystemPromptTemplate = `You are Odin Research - an autonomous engineering research agent.

Your mission: Conduct a deep, structured investigation of the indexed codebase to produce a 
complete technical intelligence package that directly supports the following project idea.

## THE PROJECT IDEA YOU ARE RESEARCHING FOR
%s

## Your Two Tools

### query_codebase(question)
Searches indexed source trees. Call aggressively - broad first, then targeted follow-ups.
- When you find an interface, follow up asking for its implementations
- When you find a pattern, query for it elsewhere to confirm it is idiomatic
- Never guess or assume file paths - always verify with the tool

### write_artifact(filename, content)
Writes a markdown research document to disk.
- Write early and often - checkpoint findings as you go
- Each artifact must be self-contained and directly useful for the project
- Cite exact file paths, type names, and function signatures throughout
- Filename should reflect content e.g. "extension_points.md", "agent_tool_interface.md"

## Research Agenda
Derive your research agenda from the project idea above. For any project involving:
- A new backend framework -> investigate: existing extension points, config loading patterns, 
  interface conventions, package structure, how new components are registered
- LangChainGo / AI agents -> investigate: agent types, tool interface, memory, prompt templates,
  chain composition, local model (Ollama) integration
- A storage or data layer -> investigate: existing data models, storage interfaces, query patterns
- A UI layer -> investigate: existing API contracts, how the frontend consumes the backend
- Open source contribution -> investigate: code conventions, test patterns, PR/review expectations

## Research Depth Requirements
For each major component you will build:
1. Find the exact file and interface it must satisfy or extend
2. Find 2-3 existing implementations of similar patterns in the codebase as reference
3. Draft the Go interface definition it should expose
4. Identify risks: missing hooks, tight coupling, anything that blocks clean integration

## Completion
When all major components are covered, write "research_complete.md" containing:
- Executive summary of findings
- Component map: what to build, where it lives, what it implements
- Concrete Go interface sketches for each major component
- Risk register: blockers, gaps, open questions
- Cross-references to all other artifacts written

Begin immediately. Do not ask for confirmation. Derive your agenda from the project idea and start.

CRITICAL: Write an artifact every 5-6 query calls - do not wait until research is complete. 
Partial findings are valuable. You MUST write your first artifact before round 6 even if 
incomplete. Loss of progress is worse than incomplete artifacts.`

// QueryAgent is the interface for the underlying RAG agent
type QueryAgent interface {
	Ask(ctx context.Context, question string) (string, error)
	ClearHistory()
}

// RunResearch runs the autonomous research loop.
// ideasFile: path to a markdown file describing the project idea (e.g. ideas/jaeger.md)
func RunResearch(ctx context.Context, agent QueryAgent, artifactDir string, ideasFile string) error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	// Load project idea
	ideaContent, err := loadIdea(ideasFile)
	if err != nil {
		return err
	}
	fmt.Printf("  Loaded project idea: %s\n\n", ideasFile)

	systemPrompt := fmt.Sprintf(researchSystemPromptTemplate, ideaContent)

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("create artifact dir: %w", err)
	}

	// Session log
	logPath := filepath.Join(artifactDir, "session.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("create log: %w", err)
	}
	defer logFile.Close()

	logf := func(format string, args ...any) {
		msg := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
		fmt.Print(msg)
		logFile.WriteString(msg)
	}

	logf("=== Odin Research Session ===")
	logf("Project idea: %s", ideasFile)
	logf("Artifact dir: %s", artifactDir)
	logf("Model: gemini-3-flash-preview")

	researchTools := []geminiTool{{
		FunctionDeclarations: []functionDecl{
			{
				Name:        "query_codebase",
				Description: "Search indexed source trees. Returns grounded answers with exact file paths and type definitions. Call multiple times with refined questions. Never guess file paths.",
				Parameters: schemaParam{
					Type: "object",
					Properties: map[string]schemaProp{
						"question": {
							Type:        "string",
							Description: "Specific technical question about the codebase",
						},
					},
					Required: []string{"question"},
				},
			},
			{
				Name:        "write_artifact",
				Description: "Write a research document to disk. Checkpoint findings early and often.",
				Parameters: schemaParam{
					Type: "object",
					Properties: map[string]schemaProp{
						"filename": {
							Type:        "string",
							Description: "Descriptive filename e.g. extension_points.md",
						},
						"content": {
							Type:        "string",
							Description: "Full markdown content citing exact file paths and type names.",
						},
					},
					Required: []string{"filename", "content"},
				},
			},
		},
	}}

	tc := &toolConfig{}
	tc.FunctionCallingConfig.Mode = "AUTO"

	system := &geminiContent{
		Parts: []geminiPart{{Text: systemPrompt}},
	}

	history := []geminiContent{
		{
			Role:  "user",
			Parts: []geminiPart{{Text: "Begin the research. Work autonomously through the full agenda. I will wait."}},
		},
	}

	var artifactsWritten []string
	const maxRounds = 50
	lastWriteRound := -1

	for round := range maxRounds {
		logf("--- Round %d ---", round+1)

		// Hard enforcement: fire a write reminder every 5 rounds without an artifact.
		// Using modulo so it fires once at rounds lastWrite+5, lastWrite+10, etc.
		// (not every single round after the threshold, which would spam history).
		roundsSinceWrite := round - lastWriteRound
		if round > 0 && roundsSinceWrite >= 5 && roundsSinceWrite%5 == 0 {
			nudge := fmt.Sprintf(
				"REMINDER: You have made %d research queries without writing any artifact. "+
					"You MUST call write_artifact RIGHT NOW before any more queries. "+
					"Write everything you have learned so far — partial findings are valuable.",
				roundsSinceWrite,
			)
			history = append(history, geminiContent{
				Role:  "user",
				Parts: []geminiPart{{Text: nudge}},
			})
			logf("  [nudge] injected write-artifact reminder (%d rounds since last write)", roundsSinceWrite)
		}

		req := geminiRequest{
			SystemInstruction: system,
			Contents:          history,
			Tools:             researchTools,
			ToolConfig:        tc,
		}

		var resp *geminiResponse
		var respErr error
		for attempt := range 5 {
			resp, respErr = doGeminiRequest(ctx, apiKey, "gemini-3-flash-preview", req)
			if respErr == nil {
				break
			}
			if strings.Contains(respErr.Error(), "high demand") || strings.Contains(respErr.Error(), "temporarily") {
				waitSecs := time.Duration((attempt+1)*30) * time.Second
				logf("Rate limited, retrying in %s (attempt %d/5)...", waitSecs, attempt+1)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(waitSecs):
				}
				continue
			}
			logf("ERROR: %v", respErr)
			return respErr
		}
		if respErr != nil {
			return fmt.Errorf("failed after 5 retries: %w", respErr)
		}
		if len(resp.Candidates) == 0 {
			logf("Empty response, stopping")
			break
		}

		candidate := resp.Candidates[0]
		history = append(history, candidate.Content)

		if text := extractText(candidate.Content); text != "" {
			logf("Gemini: %s", truncateStr(text, 250))
		}

		toolCalls := extractToolCalls(candidate.Content)
		if len(toolCalls) == 0 {
			logf("No tool calls - research loop complete")
			break
		}

		var toolResultParts []geminiPart
		for _, call := range toolCalls {
			switch call.Name {

			case "query_codebase":
				question, _ := call.Args["question"].(string)
				logf("  [query] %s", truncateStr(question, 120))

				result, err := agent.Ask(ctx, question)
				if err != nil {
					result = fmt.Sprintf("error: %v", err)
					logf("  [query] ERROR: %v", err)
				} else {
					logf("  [query] -> %d chars", len(result))
				}

				// Truncate what goes into Gemini's history to prevent context overflow.
				// The full result was already used by the local agent above.
				const maxHistoryResult = 1500
				historyResult := result
				if len(historyResult) > maxHistoryResult {
					historyResult = historyResult[:maxHistoryResult] + fmt.Sprintf("\n... [truncated %d chars for context efficiency]", len(result)-maxHistoryResult)
				}

				toolResultParts = append(toolResultParts, geminiPart{
					FunctionResp: &functionResp{
						Name:     "query_codebase",
						Response: map[string]any{"result": historyResult},
					},
				})

			case "write_artifact":
				filename, _ := call.Args["filename"].(string)
				content, _ := call.Args["content"].(string)

				filename = filepath.Base(filename)
				if !strings.HasSuffix(filename, ".md") && !strings.HasSuffix(filename, ".json") {
					filename += ".md"
				}

				outPath := filepath.Join(artifactDir, filename)
				writeErr := os.WriteFile(outPath, []byte(content), 0644)

				var resultMsg string
				if writeErr != nil {
					resultMsg = fmt.Sprintf("error: %v", writeErr)
					logf("  [write] ERROR %s: %v", filename, writeErr)
				} else {
					resultMsg = fmt.Sprintf("written: %s (%d bytes)", outPath, len(content))
					artifactsWritten = append(artifactsWritten, filename)
					lastWriteRound = round
					logf("  [write] ✓ %s (%d bytes)", filename, len(content))
				}

				toolResultParts = append(toolResultParts, geminiPart{
					FunctionResp: &functionResp{
						Name:     "write_artifact",
						Response: map[string]any{"result": resultMsg},
					},
				})

				// Stop when research is declared complete
				if filename == "research_complete.md" && writeErr == nil {
					history = append(history, geminiContent{
						Role:  "tool",
						Parts: toolResultParts,
					})
					printResearchSummary(artifactDir, artifactsWritten)
					return nil
				}
			}
		}

		history = append(history, geminiContent{
			Role:  "tool",
			Parts: toolResultParts,
		})
	}

	printResearchSummary(artifactDir, artifactsWritten)
	return nil
}

// loadIdea reads the project idea file
func loadIdea(path string) (string, error) {
	if path == "" {
		return "(no project idea provided - research the codebase architecture generally)", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not read idea file %q: %w\nHint: create it with: nano %s", path, err, path)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", fmt.Errorf("idea file %q is empty", path)
	}
	return content, nil
}

// doGeminiRequest makes a raw Gemini API call
func doGeminiRequest(ctx context.Context, apiKey, model string, req geminiRequest) (*geminiResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
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

func truncateStr(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func printResearchSummary(artifactDir string, artifacts []string) {
	fmt.Println("\n═══════════════════════════════════════════════")
	fmt.Printf("  Research complete - %d artifacts written\n", len(artifacts))
	fmt.Printf("  Location: %s\n\n", artifactDir)
	for _, a := range artifacts {
		fmt.Printf("    ✓ %s\n", a)
	}
	fmt.Println("═══════════════════════════════════════════════")
}
