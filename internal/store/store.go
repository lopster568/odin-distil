package store

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

const (
	Collection = "odin_k8s"
	VectorSize = 768 // nomic-embed-text dimension
)

type Store struct {
	client *qdrant.Client
}

func New() (*Store, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant connect: %w", err)
	}
	return &Store{client: client}, nil
}

func (s *Store) EnsureCollection(ctx context.Context) error {
	exists, err := s.client.CollectionExists(ctx, Collection)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: Collection,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     VectorSize,
			Distance: qdrant.Distance_Cosine,
		}),
	})
}

type Chunk struct {
	ID       uint64
	Text     string
	FilePath string
	Package  string
	Symbol   string // function/type name if available
	Vector   []float32
}

func (s *Store) Upsert(ctx context.Context, chunks []Chunk) error {
	points := make([]*qdrant.PointStruct, len(chunks))
	for i, c := range chunks {
		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(c.ID),
			Vectors: qdrant.NewVectors(c.Vector...),
			Payload: qdrant.NewValueMap(map[string]any{
				"text":     c.Text,
				"filepath": c.FilePath,
				"package":  c.Package,
				"symbol":   c.Symbol,
			}),
		}
	}
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: Collection,
		Points:         points,
	})
	return err
}

type Result struct {
	Text     string
	FilePath string
	Package  string
	Symbol   string
	Score    float32
}

func (s *Store) Search(ctx context.Context, vector []float32, limit uint64) ([]Result, error) {
	resp, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: Collection,
		Query:          qdrant.NewQuery(vector...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	results := make([]Result, len(resp))
	for i, p := range resp {
		pl := p.Payload
		results[i] = Result{
			Text:     pl["text"].GetStringValue(),
			FilePath: pl["filepath"].GetStringValue(),
			Package:  pl["package"].GetStringValue(),
			Symbol:   pl["symbol"].GetStringValue(),
			Score:    p.Score,
		}
	}
	return results, nil
}
