package memory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	sdk "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jStore implements the GraphStore interface for Neo4j
type Neo4jStore struct {
	client sdk.DriverWithContext
	dbName string
}

// NewNeo4jStore creates a new Neo4j graph store
func NewNeo4jStore(url, username, password, dbName string) (*Neo4jStore, error) {
	// If parameters are empty, try environment variables
	if url == "" {
		url = os.Getenv("NEO4J_URL")
	}

	if username == "" {
		username = os.Getenv("NEO4J_USERNAME")
		// Fallback to NEO4J_USER if NEO4J_USERNAME not set
		if username == "" {
			username = os.Getenv("NEO4J_USER")
		}
	}

	if password == "" {
		password = os.Getenv("NEO4J_PASSWORD")
	}

	if dbName == "" {
		dbName = "neo4j" // Default database name
	}

	driver, err := sdk.NewDriverWithContext(
		url,
		sdk.BasicAuth(
			username,
			password,
			"",
		),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Neo4j: %w", err)
	}

	return &Neo4jStore{
		client: driver,
		dbName: dbName,
	}, nil
}

// Execute runs a Cypher query with parameters
func (store *Neo4jStore) Execute(ctx context.Context, query string, params map[string]any) error {
	session := store.client.NewSession(ctx, sdk.SessionConfig{
		DatabaseName: store.dbName,
		AccessMode:   sdk.AccessModeWrite,
	})
	defer session.Close(ctx)

	_, err := session.Run(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to execute Cypher query: %w", err)
	}

	return nil
}

// Query searches the graph database with keywords or a custom Cypher query
func (store *Neo4jStore) Query(ctx context.Context, keywords string, cypher string) ([]string, error) {
	var results []string
	session := store.client.NewSession(ctx, sdk.SessionConfig{
		DatabaseName: store.dbName,
		AccessMode:   sdk.AccessModeRead,
	})
	defer session.Close(ctx)

	// Handle keyword-based search
	if keywords != "" {
		for _, keyword := range strings.Split(keywords, ",") {
			keyword = strings.TrimSpace(keyword)
			if keyword == "" {
				continue
			}

			// Simple query to find relationships based on keyword
			result, err := session.Run(
				ctx,
				`
				MATCH p=(a)-[r]->(b)
				WHERE a.name CONTAINS $term OR b.name CONTAINS $term
				RETURN a.name as source, labels(a)[0] as sourceLabel, 
					type(r) as relationship, 
					b.name as target, labels(b)[0] as targetLabel
				LIMIT 20
				`,
				map[string]any{
					"term": keyword,
				},
			)

			if err != nil {
				return results, fmt.Errorf("failed to query with keyword '%s': %w", keyword, err)
			}

			// Collect relationships
			var relationshipFound bool
			for result.Next(ctx) {
				relationshipFound = true
				record := result.Record()
				asmap := record.AsMap()

				relationship := fmt.Sprintf("%v:%v -[%v]-> %v:%v",
					asmap["sourceLabel"],
					asmap["source"],
					asmap["relationship"],
					asmap["targetLabel"],
					asmap["target"],
				)

				results = append(results, relationship)
			}

			if err := result.Err(); err != nil {
				return results, fmt.Errorf("error processing keyword results: %w", err)
			}

			if !relationshipFound {
				results = append(results, fmt.Sprintf("No relationships found for: %s", keyword))
			}
		}
	}

	// Handle custom Cypher query if provided
	if cypher != "" {
		result, err := session.Run(ctx, cypher, nil)
		if err != nil {
			return results, fmt.Errorf("failed to execute custom Cypher query: %w", err)
		}

		// Format the results as strings
		for result.Next(ctx) {
			record := result.Record()
			results = append(results, fmt.Sprintf("%v", record.AsMap()))
		}

		if err := result.Err(); err != nil {
			return results, fmt.Errorf("error processing Cypher query results: %w", err)
		}
	}

	return results, nil
}

// Close releases all resources held by the driver
func (store *Neo4jStore) Close(ctx context.Context) error {
	return store.client.Close(ctx)
}
