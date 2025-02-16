package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"
)

type Memory struct {
	Documents []Document `json:"documents" jsonschema_description:"Unstructured text you want to remember"`
	Cypher    string     `json:"cypher" jsonschema_description:"Cypher query to created a memory graph"`
}

type Document struct {
	Text     string   `json:"text" jsonschema_description:"The text you want to remember"`
	Metadata Metadata `json:"metadata" jsonschema_description:"Metadata about the document"`
}

type Metadata struct {
	Source string `json:"source" jsonschema_description:"The source of the document"`
}

type MemoryQuery struct {
	SemanticSearch string   `json:"semantic_search" jsonschema_description:"Natural language search query for semantic similarity"`
	Keywords       []string `json:"keywords" jsonschema_description:"Specific keywords to look for in memories"`
	GraphQuery     string   `json:"graph_query" jsonschema_description:"Cypher query to find related memories through relationships"`
}

func GenerateSchema[T any]() interface{} {
	// Structured Outputs uses a subset of JSON schema
	// These flags are necessary to comply with the subset
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

var memorySchema = GenerateSchema[Memory]()
var memoryQuerySchema = GenerateSchema[MemoryQuery]()

func CreateMemories(response string) Memory {
	client := openai.NewClient()
	ctx := context.Background()

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        openai.F("biography"),
		Description: openai.F("Notable information about a person"),
		Schema:      openai.F(memorySchema),
		Strict:      openai.Bool(true),
	}

	// Query the Chat Completions API
	chat, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are an advanced memory agent. You are given a context which you will analyze and extract interesting information from it. Then you will create memories, either in document format, or in a graph format, whichever is more appropriate. Make sure to store memrories in a detailed and granular format, and to include as much information as possible."),
			openai.UserMessage(response),
		}),
		ResponseFormat: openai.F[openai.ChatCompletionNewParamsResponseFormatUnion](
			openai.ResponseFormatJSONSchemaParam{
				Type:       openai.F(openai.ResponseFormatJSONSchemaTypeJSONSchema),
				JSONSchema: openai.F(schemaParam),
			},
		),
		// Only certain models can perform structured outputs
		Model: openai.F(openai.ChatModelGPT4o2024_08_06),
	})

	if err != nil {
		panic(err.Error())
	}

	// The model responds with a JSON string, so parse it into a struct
	memory := Memory{}
	err = json.Unmarshal([]byte(chat.Choices[0].Message.Content), &memory)
	if err != nil {
		panic(err.Error())
	}

	return memory
}

func RetrieveMemories(query string) []Document {
	client := openai.NewClient()
	ctx := context.Background()

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        openai.F("memory_query"),
		Description: openai.F("Search parameters for memory retrieval"),
		Schema:      openai.F(memoryQuerySchema),
		Strict:      openai.Bool(true),
	}

	chat, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a memory retrieval agent. Analyze the query and create search parameters that will help find relevant memories. Generate both semantic search terms and specific keywords, plus a Cypher query to find related memories through relationships."),
			openai.UserMessage(query),
		}),
		ResponseFormat: openai.F[openai.ChatCompletionNewParamsResponseFormatUnion](
			openai.ResponseFormatJSONSchemaParam{
				Type:       openai.F(openai.ResponseFormatJSONSchemaTypeJSONSchema),
				JSONSchema: openai.F(schemaParam),
			},
		),
		Model: openai.F(openai.ChatModelGPT4o2024_08_06),
	})
	if err != nil {
		log.Error("Failed to analyze query:", err)
		return nil
	}

	// Parse the structured response
	var searchParams MemoryQuery
	err = json.Unmarshal([]byte(chat.Choices[0].Message.Content), &searchParams)
	if err != nil {
		log.Error("Failed to parse search parameters:", err)
		return nil
	}

	// Create a vector store query to find relevant memories
	qdrantClient := NewQdrant("memories", 1536)
	results, err := qdrantClient.Query(searchParams.SemanticSearch)
	if err != nil {
		log.Error("Failed to query vector store:", err)
		return nil
	}

	var documents []Document
	for _, result := range results {
		documents = append(documents, Document{
			Text: result["content"].(string),
			Metadata: Metadata{
				Source: "memory_store",
			},
		})
	}

	// Also query the graph database for related memories
	neo4jClient, err := NewNeo4j()
	if err != nil {
		log.Error("Failed to connect to Neo4j:", err)
		return documents
	}

	// Query both direct and indirect relationships
	cypher := `
	MATCH (n)-[r*1..2]-(m)
	WHERE n.content CONTAINS $query
	RETURN m.content as content, m.source as source
	LIMIT 5
	`
	graphResults, err := neo4jClient.Query(cypher)
	if err != nil {
		log.Error("Failed to query graph database:", err)
		return documents
	}

	for _, result := range graphResults {
		documents = append(documents, Document{
			Text: result["content"].(string),
			Metadata: Metadata{
				Source: result["source"].(string),
			},
		})
	}

	return documents
}

// StoreMemories stores new memories in both vector and graph stores
func StoreMemories(response string) error {
	// First, create structured memories using OpenAI
	memories := CreateMemories(response)

	// Store documents in vector store
	qdrantClient := NewQdrant("memories", 1536)
	var docs []string
	metadata := make(map[string]any)

	for _, doc := range memories.Documents {
		docs = append(docs, doc.Text)
		metadata["source"] = doc.Metadata.Source
	}

	qdrantClient.Add(docs, metadata)

	// Store relationships in graph database
	if memories.Cypher != "" {
		neo4jClient, err := NewNeo4j()
		if err != nil {
			return fmt.Errorf("failed to connect to Neo4j: %w", err)
		}

		err = neo4jClient.Write(memories.Cypher)
		if err != nil {
			return fmt.Errorf("failed to store graph relationships: %w", err)
		}
	}

	return nil
}

// FormatMemoriesAsContext formats retrieved memories into a context string
func FormatMemoriesAsContext(documents []Document) string {
	if len(documents) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Relevant memories from previous conversations:\n\n")

	for i, doc := range documents {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, doc.Text))
		if i < len(documents)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}
