package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ollama/ollama/api"
)

type Client struct {
	ol *api.Client
}

func New() (*Client, error) {
	u, _ := url.Parse("http://localhost:11434")
	ol := api.NewClient(u, http.DefaultClient)
	return &Client{ol: ol}, nil
}

func (c *Client) Generate(ctx context.Context, model, prompt string) (string, error) {
	var sb strings.Builder
	stream := false
	err := c.ol.Generate(ctx, &api.GenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: &stream,
	}, func(r api.GenerateResponse) error {
		sb.WriteString(r.Response)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	return sb.String(), nil
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.ol.Embed(ctx, &api.EmbedRequest{
		Model: "nomic-embed-text",
		Input: text,
	})
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	out := make([]float32, len(resp.Embeddings[0]))
	for i, v := range resp.Embeddings[0] {
		out[i] = float32(v)
	}
	return out, nil
}
