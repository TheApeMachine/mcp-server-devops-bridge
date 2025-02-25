package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Global driver instances
var (
	neo4jDriver  neo4j.DriverWithContext
	qdrantClient QdrantClient
)

// QdrantClient interface for vector operations
type QdrantClient interface {
	Add(docs []string, metadata map[string]interface{}) error
	Search(ctx context.Context, collection string, vector []float32, limit int) ([]map[string]interface{}, error)
}

// CodeContext represents a piece of code with its context and relationships
type CodeContext struct {
	// Unique identifier
	ID string `json:"id"`

	// Vector representation of code
	CodeEmbedding []float32 `json:"code_embedding"`

	// Metadata
	Metadata struct {
		Language     string    `json:"language"`
		Framework    string    `json:"framework"`
		Dependencies []string  `json:"dependencies"`
		Author       string    `json:"author"`
		Timestamp    time.Time `json:"timestamp"`
		Path         string    `json:"path"`
	} `json:"metadata"`

	// Relationships
	Relations struct {
		Dependencies []string `json:"dependencies"` // Other code files this depends on
		UsedBy       []string `json:"used_by"`      // Code files that use this
		RelatedDocs  []string `json:"related_docs"` // Related documentation
		Issues       []string `json:"issues"`       // Related issues/tickets
		PRs          []string `json:"prs"`          // Related pull requests
	} `json:"relations"`

	// Analysis results
	Analysis struct {
		Complexity     float32            `json:"complexity"`
		Coverage       float32            `json:"coverage"`
		BugProbability float32            `json:"bug_probability"`
		Performance    map[string]float32 `json:"performance"`
		Security       map[string]string  `json:"security"`
	} `json:"analysis"`
}

// NewCodeContext creates a new CodeContext instance
func NewCodeContext(path string) *CodeContext {
	cc := &CodeContext{
		ID: uuid.New().String(),
	}
	cc.Metadata.Path = path
	cc.Metadata.Timestamp = time.Now()
	return cc
}

// Store saves the CodeContext to both Qdrant and Neo4j
func (c *CodeContext) Store(ctx context.Context) error {
	// Store vector representation in Qdrant
	metadata := map[string]interface{}{
		"id":            c.ID,
		"language":      c.Metadata.Language,
		"framework":     c.Metadata.Framework,
		"dependencies":  c.Metadata.Dependencies,
		"author":        c.Metadata.Author,
		"timestamp":     c.Metadata.Timestamp,
		"path":          c.Metadata.Path,
		"analysis":      c.Analysis,
		"relationships": c.Relations,
	}

	// Convert float32 slice to string representation for storage
	embeddingStr := fmt.Sprintf("%v", c.CodeEmbedding)
	docs := []string{embeddingStr}
	err := qdrantClient.Add(docs, metadata)
	if err != nil {
		return fmt.Errorf("failed to store in Qdrant: %v", err)
	}

	// Store relationships in Neo4j
	session := neo4jDriver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	query := `
	MERGE (code:CodeFile {id: $id, path: $path})
	SET code += $metadata
	WITH code
	UNWIND $dependencies as dep
	MERGE (depCode:CodeFile {path: dep})
	MERGE (code)-[:DEPENDS_ON]->(depCode)
	WITH code
	UNWIND $usedBy as user
	MERGE (userCode:CodeFile {path: user})
	MERGE (userCode)-[:USES]->(code)
	WITH code
	UNWIND $docs as doc
	MERGE (docNode:Documentation {path: doc})
	MERGE (code)-[:HAS_DOCUMENTATION]->(docNode)
	WITH code
	UNWIND $issues as issue
	MERGE (issueNode:Issue {id: issue})
	MERGE (code)-[:HAS_ISSUE]->(issueNode)
	WITH code
	UNWIND $prs as pr
	MERGE (prNode:PullRequest {id: pr})
	MERGE (code)-[:HAS_PR]->(prNode)
	`

	_, err = session.Run(ctx, query, map[string]interface{}{
		"id":           c.ID,
		"path":         c.Metadata.Path,
		"metadata":     c.Metadata,
		"dependencies": c.Relations.Dependencies,
		"usedBy":       c.Relations.UsedBy,
		"docs":         c.Relations.RelatedDocs,
		"issues":       c.Relations.Issues,
		"prs":          c.Relations.PRs,
	})

	if err != nil {
		return fmt.Errorf("failed to store in Neo4j: %v", err)
	}

	return nil
}

// QuerySimilarCode finds similar code contexts based on vector similarity
func QuerySimilarCode(ctx context.Context, codeContext *CodeContext, limit int) ([]*CodeContext, error) {
	// Search Qdrant for similar vectors
	searchResults, err := qdrantClient.Search(ctx, "code_contexts", codeContext.CodeEmbedding, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search Qdrant: %v", err)
	}

	// Get IDs from search results
	var ids []string
	for _, result := range searchResults {
		ids = append(ids, result["id"].(string))
	}

	// Enrich with Neo4j relationship data
	session := neo4jDriver.NewSession(ctx, neo4j.SessionConfig{})
	defer session.Close(ctx)

	query := `
	MATCH (code:CodeFile)
	WHERE code.id IN $ids
	OPTIONAL MATCH (code)-[:DEPENDS_ON]->(dep)
	OPTIONAL MATCH (code)<-[:USES]-(usedBy)
	OPTIONAL MATCH (code)-[:HAS_DOCUMENTATION]->(doc)
	OPTIONAL MATCH (code)-[:HAS_ISSUE]->(issue)
	OPTIONAL MATCH (code)-[:HAS_PR]->(pr)
	RETURN code,
		collect(distinct dep.path) as dependencies,
		collect(distinct usedBy.path) as usedBy,
		collect(distinct doc.path) as docs,
		collect(distinct issue.id) as issues,
		collect(distinct pr.id) as prs
	`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"ids": ids,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query Neo4j: %v", err)
	}

	var contexts []*CodeContext
	for result.Next(ctx) {
		record := result.Record()
		codeNode := record.Values[0].(neo4j.Node)

		cc := &CodeContext{
			ID: codeNode.Props["id"].(string),
		}

		// Populate metadata
		cc.Metadata.Path = codeNode.Props["path"].(string)
		cc.Metadata.Language = codeNode.Props["language"].(string)
		cc.Metadata.Framework = codeNode.Props["framework"].(string)
		cc.Metadata.Author = codeNode.Props["author"].(string)

		// Populate relationships
		cc.Relations.Dependencies = toStringSlice(record.Values[1])
		cc.Relations.UsedBy = toStringSlice(record.Values[2])
		cc.Relations.RelatedDocs = toStringSlice(record.Values[3])
		cc.Relations.Issues = toStringSlice(record.Values[4])
		cc.Relations.PRs = toStringSlice(record.Values[5])

		contexts = append(contexts, cc)
	}

	return contexts, nil
}

// Helper function to convert interface{} slice to string slice
func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	interfaceSlice := v.([]interface{})
	result := make([]string, len(interfaceSlice))
	for i, v := range interfaceSlice {
		result[i] = v.(string)
	}
	return result
}

// Add tools for code context operations
func addCodeContextTools(s *server.MCPServer) {
	// Tool to store code context
	storeCodeContextTool := mcp.NewTool("store_code_context",
		mcp.WithDescription("Store code context information in both vector and graph stores"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the code file"),
		),
		mcp.WithString("language",
			mcp.Required(),
			mcp.Description("Programming language of the code"),
		),
		mcp.WithString("framework",
			mcp.Description("Framework used, if any"),
		),
		mcp.WithString("dependencies",
			mcp.Description("List of dependencies"),
		),
	)

	s.AddTool(storeCodeContextTool, handleStoreCodeContext)

	// Tool to query similar code
	querySimilarCodeTool := mcp.NewTool("query_similar_code",
		mcp.WithDescription("Find similar code based on context and relationships"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the code file to find similar contexts for"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of similar contexts to return"),
		),
	)

	s.AddTool(querySimilarCodeTool, handleQuerySimilarCode)
}

func handleStoreCodeContext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := request.Params.Arguments["path"].(string)
	language := request.Params.Arguments["language"].(string)
	framework := request.Params.Arguments["framework"].(string)
	dependencies := request.Params.Arguments["dependencies"].([]string)

	codeContext := NewCodeContext(path)
	codeContext.Metadata.Language = language
	codeContext.Metadata.Framework = framework
	codeContext.Metadata.Dependencies = dependencies

	// TODO: Generate code embeddings using an appropriate model
	// For now, we'll use a placeholder
	codeContext.CodeEmbedding = make([]float32, 384) // Example dimension

	if err := codeContext.Store(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to store code context: %v", err)), nil
	}

	return mcp.NewToolResultText("Successfully stored code context"), nil
}

func handleQuerySimilarCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := request.Params.Arguments["path"].(string)
	limit := 10
	if l, ok := request.Params.Arguments["limit"].(float64); ok {
		limit = int(l)
	}

	codeContext := NewCodeContext(path)
	// TODO: Generate code embeddings for the query context

	similar, err := QuerySimilarCode(ctx, codeContext, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to query similar code: %v", err)), nil
	}

	// Format results
	var result string
	for _, ctx := range similar {
		result += fmt.Sprintf("Path: %s\nLanguage: %s\nFramework: %s\nDependencies: %v\n\n",
			ctx.Metadata.Path,
			ctx.Metadata.Language,
			ctx.Metadata.Framework,
			ctx.Metadata.Dependencies)
	}

	return mcp.NewToolResultText(result), nil
}
