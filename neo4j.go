package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/invopop/jsonschema"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

/*
Neo4j is a wrapper around the Neo4j database that turns it into a tool,
usable by the agent.
*/
type Neo4j struct {
	client    neo4j.DriverWithContext
	Operation string
	Cypher    string
}

/*
GenerateSchema implements the Tool interface and renders the schema as a jsonschema string,
which can be injected into the prompt. It is used to explain to the agent how to use the tool.
*/
func (neo4j *Neo4j) GenerateSchema() string {
	schema := jsonschema.Reflect(&Neo4j{})
	buf, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		log.Error(err)
		return "Error marshalling schema"
	}

	return string(buf)
}

/*
NewNeo4j creates a new Neo4j client.
*/
func NewNeo4j() (*Neo4j, error) {
	ctx := context.Background()

	client, err := neo4j.NewDriverWithContext("neo4j://localhost:7687", neo4j.BasicAuth("neo4j", "securepassword", ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j client: %w", err)
	}

	if err := client.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("failed to verify Neo4j connectivity: %w", err)
	}

	return &Neo4j{client: client}, nil
}

/*
Initialize initializes the Neo4j client.
*/
func (n *Neo4j) Initialize() error {
	if n.client == nil {
		return fmt.Errorf("Neo4j client is not initialized")
	}
	ctx := context.Background()
	return n.client.VerifyConnectivity(ctx)
}

func (n *Neo4j) Connect() error {
	if n.client == nil {
		return fmt.Errorf("Neo4j client is not initialized")
	}
	return nil
}

/*
Query executes a Cypher query on the Neo4j database and returns the results.
*/
func (n *Neo4j) Query(query string) ([]map[string]interface{}, error) {
	if n.client == nil {
		return nil, fmt.Errorf("Neo4j client is not initialized")
	}

	ctx := context.Background()
	session := n.client.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var records []map[string]interface{}
	for result.Next(ctx) {
		record := result.Record()
		if node, ok := record.Values[0].(neo4j.Node); ok {
			records = append(records, node.Props)
		}
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("error processing results: %w", err)
	}

	return records, nil
}

/*
Write executes a Cypher write query on the Neo4j database.
*/
func (n *Neo4j) Write(query string) error {
	if n.client == nil {
		return fmt.Errorf("Neo4j client is not initialized")
	}

	ctx := context.Background()
	session := n.client.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to execute write query: %w", err)
	}

	if err := result.Err(); err != nil {
		return fmt.Errorf("error processing write results: %w", err)
	}

	return nil
}

/*
Close closes the Neo4j client connection.
*/
func (n *Neo4j) Close() error {
	if n.client == nil {
		return fmt.Errorf("Neo4j client is not initialized")
	}
	ctx := context.Background()
	return n.client.Close(ctx)
}

/*
Use implements the Tool interface and is used to execute the tool.
*/
func (neo4j *Neo4j) Use(ctx context.Context, args map[string]any) string {
	if neo4j.client == nil {
		return "Neo4j client is not initialized"
	}

	switch neo4j.Operation {
	case "query":
		cypher, ok := args["cypher"].(string)
		if !ok {
			return "Missing or invalid cypher query"
		}
		records, err := neo4j.Query(cypher)
		if err != nil {
			return fmt.Sprintf("Query failed: %v", err)
		}
		result, err := json.Marshal(records)
		if err != nil {
			return fmt.Sprintf("Failed to marshal results: %v", err)
		}
		return string(result)

	case "write":
		query, ok := args["query"].(string)
		if !ok {
			return "Missing or invalid write query"
		}
		if err := neo4j.Write(query); err != nil {
			return fmt.Sprintf("Write failed: %v", err)
		}
		return "Write operation successful"

	default:
		return "Unsupported operation"
	}
}
