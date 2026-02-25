package embedder

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"odin/internal/ingester"
	"odin/internal/llm"
)

const maxChunkRunes = 6000 // safe limit for nomic-embed-text

type EmbeddedChunk struct {
	Text     string
	FilePath string
	Package  string
	Symbol   string
	Vector   []float32
}

type Embedder struct {
	llm *llm.Client
}

func New(l *llm.Client) *Embedder {
	return &Embedder{llm: l}
}

func truncate(s string) string {
	if utf8.RuneCountInString(s) <= maxChunkRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxChunkRunes])
}

func (e *Embedder) EmbedChunks(ctx context.Context, chunks []ingester.Chunk) ([]EmbeddedChunk, error) {
	out := make([]EmbeddedChunk, 0, len(chunks))
	for _, c := range chunks {
		text := strings.TrimSpace(c.Text)
		if len(text) < 20 {
			continue
		}
		text = truncate(text)
		vec, err := e.llm.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed %s: %w", c.FilePath, err)
		}
		out = append(out, EmbeddedChunk{
			Text:     text,
			FilePath: c.FilePath,
			Package:  c.Package,
			Symbol:   c.Symbol,
			Vector:   vec,
		})
	}
	return out, nil
}
