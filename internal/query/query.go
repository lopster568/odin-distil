package query

import (
	"context"
	"fmt"
	"strings"

	"odin/internal/llm"
	"odin/internal/store"
)

type Engine struct {
	llm   *llm.Client
	store *store.Store
}

func New(l *llm.Client, s *store.Store) *Engine {
	return &Engine{llm: l, store: s}
}

func (e *Engine) Ask(ctx context.Context, question string) (string, error) {
	// 1. embed the question
	vec, err := e.llm.Embed(ctx, question)
	if err != nil {
		return "", fmt.Errorf("embed question: %w", err)
	}

	// 2. retrieve top 8 chunks
	results, err := e.store.Search(ctx, vec, 15)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		return "No relevant code found. Have you run 'odin ingest' yet?", nil
	}

	// 3. build context
	var sb strings.Builder
	sb.WriteString("You are an expert Kubernetes and Go engineer.\n")
	sb.WriteString("Answer the question using ONLY the code context below.\n")
	sb.WriteString("Always cite the file path and function name when referencing code.\n\n")
	sb.WriteString("=== CODE CONTEXT ===\n")
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("\n--- [%d] %s | %s | score: %.3f ---\n", i+1, r.FilePath, r.Symbol, r.Score))
		sb.WriteString(r.Text)
		sb.WriteString("\n")
	}
	sb.WriteString("\n=== QUESTION ===\n")
	sb.WriteString(question)
	sb.WriteString("\n\n=== ANSWER ===\n")

	// 4. generate
	answer, err := e.llm.Generate(ctx, "qwen2.5:72b", sb.String())
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	return answer, nil
}
