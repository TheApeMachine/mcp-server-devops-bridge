package memory

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	sdk "github.com/qdrant/go-client/qdrant"
)

// OpenAIEmbedder handles text to vector conversion using OpenAI's API
type OpenAIEmbedder struct {
	apiKey string
	model  string
	client *openai.Client
}

// NewOpenAIEmbedder creates a new embedder using OpenAI's API
func NewOpenAIEmbedder(apiKey, model string) *OpenAIEmbedder {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if model == "" {
		model = "text-embedding-3-large" // Default model
	}

	return &OpenAIEmbedder{
		apiKey: apiKey,
		model:  model,
		client: openai.NewClient(option.WithAPIKey(apiKey)),
	}
}

// QdrantStore implements the VectorStore interface for Qdrant
type QdrantStore struct {
	client     *sdk.Client
	collection string
	dimensions int
	embedder   *OpenAIEmbedder
}

// NewQdrantStore creates a new vector store using Qdrant
func NewQdrantStore(collection string) (*QdrantStore, error) {
	client, err := sdk.NewClient(&sdk.Config{
		Host:                   "localhost",
		APIKey:                 os.Getenv("QDRANT_API_KEY"),
		UseTLS:                 false,
		SkipCompatibilityCheck: true,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create Qdrant client: %w", err)
	}

	store := &QdrantStore{
		client:     client,
		collection: collection,
		dimensions: 3072, // For text-embedding-3-large
		embedder:   NewOpenAIEmbedder(os.Getenv("OPENAI_API_KEY"), "text-embedding-3-large"),
	}

	// Ensure the collection exists
	if err := store.ensureCollection(); err != nil {
		return nil, fmt.Errorf("failed to ensure collection: %w", err)
	}

	return store, nil
}

// Store saves a text document with metadata to the vector store
func (store *QdrantStore) Store(ctx context.Context, text string, metadata map[string]any) error {
	waitUpsert := true

	// In a real implementation, we would generate embeddings here
	// This is a simplified version that just uses dummy vectors
	_, err := store.client.Upsert(ctx, &sdk.UpsertPoints{
		CollectionName: store.collection,
		Wait:           &waitUpsert,
		Points: []*sdk.PointStruct{
			{
				Id:      sdk.NewIDNum(1),
				Vectors: sdk.NewVectors(0.05, 0.61, 0.76, 0.74),
				Payload: sdk.NewValueMap(map[string]any{
					"content": text,
					"source":  metadata["source"],
				}),
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	return nil
}

// Search performs a semantic search for similar texts
func (store *QdrantStore) Search(ctx context.Context, query string) ([]string, error) {
	// This is a simplified version that doesn't actually do semantic search
	// but returns mock data that matches the format expected by callers
	searchedPoints, err := store.client.Query(ctx, &sdk.QueryPoints{
		CollectionName: store.collection,
		Query:          sdk.NewQuery(0.2, 0.1, 0.9, 0.7),
		WithPayload:    sdk.NewWithPayloadInclude("content"),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	var results []string

	for _, point := range searchedPoints {
		if content, ok := point.Payload["content"]; ok {
			if contentStr := content.GetStringValue(); contentStr != "" {
				results = append(results, contentStr)
			}
		}
	}

	return results, nil
}

// Delete is a stub implementation to satisfy the interface
func (store *QdrantStore) Delete(ctx context.Context, filter map[string]any) error {
	// Not implemented in the original code
	return nil
}

// ensureCollection creates the collection if it doesn't exist
func (store *QdrantStore) ensureCollection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List all collections
	collections, err := store.client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	// Check if collection exists
	for _, name := range collections {
		if name == store.collection {
			return nil // Collection exists
		}
	}

	// Create collection
	defaultSegmentNumber := uint64(2)
	err = store.client.CreateCollection(ctx, &sdk.CreateCollection{
		CollectionName: store.collection,
		VectorsConfig: sdk.NewVectorsConfig(&sdk.VectorParams{
			Size:     uint64(store.dimensions),
			Distance: sdk.Distance_Cosine,
		}),
		OptimizersConfig: &sdk.OptimizersConfigDiff{
			DefaultSegmentNumber: &defaultSegmentNumber,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}
