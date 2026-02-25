package store

import (
	"context"
	"fmt"
	"strings"

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
	ID        uint64
	Text      string
	FilePath  string
	Package   string
	Symbol    string // function/type name if available
	Repo      string
	DirPrefix string
	Vector    []float32
}

func (s *Store) Upsert(ctx context.Context, chunks []Chunk) error {
	points := make([]*qdrant.PointStruct, len(chunks))
	for i, c := range chunks {
		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewIDNum(c.ID),
			Vectors: qdrant.NewVectors(c.Vector...),
			Payload: qdrant.NewValueMap(map[string]any{
				"text":      c.Text,
				"filepath":  c.FilePath,
				"package":   c.Package,
				"symbol":    c.Symbol,
				"repo":      c.Repo,
				"dirprefix": c.DirPrefix,
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
	Text      string
	FilePath  string
	Package   string
	Symbol    string
	Repo      string
	DirPrefix string
	Score     float32
}

func (s *Store) Search(ctx context.Context, vector []float32, limit uint64) ([]Result, error) {
	return s.search(ctx, vector, limit, "")
}

// SearchWithFilter performs a vector search and post-filters results to those whose
// filepath contains pathPrefix. Pass empty string to skip filtering.
func (s *Store) SearchWithFilter(ctx context.Context, vector []float32, limit uint64, pathPrefix string) ([]Result, error) {
	// Fetch more than needed to have enough after prefix filtering.
	fetchLimit := limit * 5
	if fetchLimit < 200 {
		fetchLimit = 200
	}
	return s.search(ctx, vector, fetchLimit, pathPrefix)
}

func (s *Store) search(ctx context.Context, vector []float32, limit uint64, pathPrefix string) ([]Result, error) {
	resp, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: Collection,
		Query:          qdrant.NewQuery(vector...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	results := make([]Result, 0, len(resp))
	for _, p := range resp {
		pl := p.Payload
		fp := pl["filepath"].GetStringValue()
		if pathPrefix != "" && !strings.Contains(fp, pathPrefix) {
			continue
		}
		results = append(results, Result{
			Text:      pl["text"].GetStringValue(),
			FilePath:  fp,
			Package:   pl["package"].GetStringValue(),
			Symbol:    pl["symbol"].GetStringValue(),
			Repo:      pl["repo"].GetStringValue(),
			DirPrefix: pl["dirprefix"].GetStringValue(),
			Score:     p.Score,
		})
	}
	return results, nil
}
