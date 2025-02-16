package main

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v3/client"
	"github.com/invopop/jsonschema"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/vectorstores"
	"github.com/tmc/langchaingo/vectorstores/qdrant"
)

/*
Qdrant is a wrapper around the vector store that turns it into a tool,
usable by the agent.
*/
type Qdrant struct {
	ctx        context.Context
	client     *qdrant.Store
	embedder   embeddings.Embedder
	collection string
	dimension  uint64
	Operation  string
	SearchText string
	Documents  []string
	Metadata   map[string]any
}

/*
NewQdrant creates a new Qdrant tool.
*/
func NewQdrant(collection string, dimension uint64) *Qdrant {
	ctx := context.Background()

	var (
		llm    *openai.LLM
		err    error
		e      embeddings.Embedder
		url    *url.URL
		client qdrant.Store
	)

	if llm, err = openai.New(); err != nil {
		log.Error(err)
		return nil
	}

	if e, err = embeddings.NewEmbedder(llm); err != nil {
		log.Error(err)
		return nil
	}

	if url, err = url.Parse("http://localhost:6333"); err != nil {
		log.Error(err)
		return nil
	}

	if err = createCollectionIfNotExists(collection, url, dimension); err != nil {
		log.Error(err)
		return nil
	}

	if client, err = qdrant.New(
		qdrant.WithURL(*url),
		qdrant.WithCollectionName(collection),
		qdrant.WithEmbedder(e),
		qdrant.WithAPIKey("gKzti5QyA5KeLQYQFLA1T6pT3GYE9pza"),
	); err != nil {
		log.Error(err)
		return nil
	}

	return &Qdrant{
		ctx:        ctx,
		client:     &client,
		embedder:   e,
		collection: collection,
		dimension:  dimension,
	}
}

/*
Initialize initializes the Qdrant client.
*/
func (q *Qdrant) Initialize() error {
	return nil
}

/*
Connect connects to the Qdrant client.
*/
func (q *Qdrant) Connect() error {
	return nil
}

/*
Use implements the Tool interface
*/
func (qdrant *Qdrant) Use(ctx context.Context, args map[string]any) string {
	switch qdrant.Operation {
	case "add":
		if docs, ok := args["documents"].([]string); ok {
			return qdrant.Add(docs, args["metadata"].(map[string]any))
		}

		return "Invalid documents format"
	case "query":
		if query, ok := args["query"].(string); ok {
			results, err := qdrant.Query(query)
			if err != nil {
				log.Error(err)
				return "Error querying Qdrant"
			}

			buf, err := json.Marshal(results)
			if err != nil {
				log.Error(err)
				return "Error marshalling results"
			}

			return string(buf)
		}

		return "Invalid query format"
	default:
		return "Unsupported operation"
	}
}

/*
GenerateSchema implements the Tool interface and renders the schema as a jsonschema string,
which can be injected into the prompt. It is used to explain to the agent how to use the tool.
*/
func (qdrant *Qdrant) GenerateSchema() string {
	schema := jsonschema.Reflect(&Qdrant{})
	buf, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Error(err)
		return "Error marshalling schema"
	}

	return string(buf)
}

/*
AddDocuments is a wrapper around the qdrant.Store.AddDocuments method.
*/
func (q *Qdrant) AddDocuments(docs []schema.Document) error {
	_, err := q.client.AddDocuments(q.ctx, docs)
	return err
}

/*
SimilaritySearch is a wrapper around the qdrant.Store.SimilaritySearch method.
*/
func (q *Qdrant) SimilaritySearch(query string, k int, opts ...vectorstores.Option) ([]schema.Document, error) {
	docs, err := q.client.SimilaritySearch(q.ctx, query, k, opts...)
	return docs, err
}

/*
Query is a wrapper around the qdrant.Store.SimilaritySearch method.
*/
type QdrantResult struct {
	Metadata map[string]any `json:"metadata"`
	Content  string         `json:"content"`
}

/*
Query is a wrapper around the qdrant.Store.SimilaritySearch method.
*/
func (q *Qdrant) Query(query string) ([]map[string]interface{}, error) {
	// Perform the similarity search with the options
	docs, err := q.client.SimilaritySearch(q.ctx, query, 1, vectorstores.WithScoreThreshold(0.7))
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for _, doc := range docs {
		results = append(results, map[string]interface{}{
			"metadata": doc.Metadata,
			"content":  doc.PageContent,
		})
	}

	return results, nil
}

/*
Add is a wrapper around the qdrant.Store.AddDocuments method.
*/
func (q *Qdrant) Add(docs []string, metadata map[string]any) string {
	for _, doc := range docs {
		if _, err := q.client.AddDocuments(q.ctx, []schema.Document{
			{
				PageContent: doc,
				Metadata:    metadata,
			},
		}); err != nil {
			log.Error(err)
		}
	}

	return "memory saved in vector store"
}

/*
createCollectionIfNotExists uses an HTTP PUT call to create a collection if it does not exist.
*/
func createCollectionIfNotExists(collection string, uri *url.URL, dimension uint64) error {
	var (
		response *client.Response
		err      error
	)

	// Add API key to request headers
	headers := map[string]string{
		"Content-Type": "application/json",
		"api-key":      "gKzti5QyA5KeLQYQFLA1T6pT3GYE9pza",
	}

	// First we do a GET call to check if the collection exists
	if response, err = client.Get(uri.String()+"/collections/"+collection, client.Config{
		Header: headers,
	}); err != nil {
		log.Error(err)
		return err
	}

	if response.StatusCode() == 404 {
		// Prepare the request body for creating a new collection
		requestBody := map[string]interface{}{
			"name": collection,
			"vectors": map[string]interface{}{
				"size":     dimension,
				"distance": "Cosine",
			},
		}

		response, err = client.Put(uri.String()+"/collections/"+collection, client.Config{
			Header: headers,
			Body:   requestBody,
		})

		if err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}
