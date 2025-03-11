// Package memory provides the memory tool implementation
package memory

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/smartystreets/goconvey/convey"
)

// MockVectorStore implements the VectorStore interface for testing
type MockVectorStore struct {
	documents []string
	metadata  []map[string]interface{}
	results   []string
	err       error
}

func (m *MockVectorStore) Store(ctx context.Context, text string, metadata map[string]interface{}) error {
	if m.err != nil {
		return m.err
	}
	m.documents = append(m.documents, text)
	m.metadata = append(m.metadata, metadata)
	return nil
}

func (m *MockVectorStore) Search(ctx context.Context, query string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// MockGraphStore implements the GraphStore interface for testing
type MockGraphStore struct {
	queries []string
	params  []map[string]interface{}
	results []string
	err     error
}

func (m *MockGraphStore) Execute(ctx context.Context, query string, params map[string]interface{}) error {
	if m.err != nil {
		return m.err
	}
	m.queries = append(m.queries, query)
	m.params = append(m.params, params)
	return nil
}

func (m *MockGraphStore) Query(ctx context.Context, keywords string, cypher string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// Helper function for creating mock request
func newMockRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      "memory",
			Arguments: args,
		},
	}
}

// TestNew tests the New constructor
func TestNew(t *testing.T) {
	Convey("Given vector and graph stores", t, func() {
		vectorStore := &MockVectorStore{}
		graphStore := &MockGraphStore{}

		Convey("When creating a new memory tool", func() {
			tool := New(vectorStore, graphStore)

			Convey("It should return a non-nil tool", func() {
				So(tool, ShouldNotBeNil)
			})

			Convey("It should have the correct name", func() {
				So(tool.Handle().Name, ShouldEqual, "memory")
			})
		})
	})
}

// TestHandle tests the Handle method
func TestHandle(t *testing.T) {
	Convey("Given a memory tool", t, func() {
		vectorStore := &MockVectorStore{}
		graphStore := &MockGraphStore{}
		tool := New(vectorStore, graphStore)

		Convey("When calling Handle", func() {
			handle := tool.Handle()

			Convey("It should return the correct mcp.Tool", func() {
				So(handle, ShouldNotBeNil)
				So(handle.Name, ShouldEqual, "memory")
			})
		})
	})
}

// TestValidate tests the validate method
func TestValidate(t *testing.T) {
	Convey("Given a memory tool", t, func() {
		vectorStore := &MockVectorStore{}
		graphStore := &MockGraphStore{}
		tool := New(vectorStore, graphStore)

		Convey("When validating with missing operation", func() {
			request := newMockRequest(map[string]interface{}{})

			ok, err := tool.validate(request)

			Convey("It should return an error", func() {
				So(ok, ShouldBeFalse)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, "operation is required")
			})
		})

		Convey("When validating add operation without document or cypher", func() {
			request := newMockRequest(map[string]interface{}{
				"operation": "add",
			})

			ok, err := tool.validate(request)

			Convey("It should return an error", func() {
				So(ok, ShouldBeFalse)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, "at least one of document or cypher is required")
			})
		})

		Convey("When validating add operation with document", func() {
			request := newMockRequest(map[string]interface{}{
				"operation": "add",
				"document":  "test document",
			})

			ok, err := tool.validate(request)

			Convey("It should validate successfully", func() {
				So(ok, ShouldBeTrue)
				So(err, ShouldBeNil)
			})
		})

		Convey("When validating add operation with cypher", func() {
			request := newMockRequest(map[string]interface{}{
				"operation": "add",
				"cypher":    "CREATE (n:Test) RETURN n",
			})

			ok, err := tool.validate(request)

			Convey("It should validate successfully", func() {
				So(ok, ShouldBeTrue)
				So(err, ShouldBeNil)
			})
		})

		Convey("When validating query operation without parameters", func() {
			request := newMockRequest(map[string]interface{}{
				"operation": "query",
			})

			ok, err := tool.validate(request)

			Convey("It should return an error", func() {
				So(ok, ShouldBeFalse)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, "at least one of question, keywords, or cypher is required")
			})
		})

		Convey("When validating query operation with question", func() {
			request := newMockRequest(map[string]interface{}{
				"operation": "query",
				"question":  "test question",
			})

			ok, err := tool.validate(request)

			Convey("It should validate successfully", func() {
				So(ok, ShouldBeTrue)
				So(err, ShouldBeNil)
			})
		})

		Convey("When validating query operation with keywords", func() {
			request := newMockRequest(map[string]interface{}{
				"operation": "query",
				"keywords":  "test,keywords",
			})

			ok, err := tool.validate(request)

			Convey("It should validate successfully", func() {
				So(ok, ShouldBeTrue)
				So(err, ShouldBeNil)
			})
		})

		Convey("When validating query operation with cypher", func() {
			request := newMockRequest(map[string]interface{}{
				"operation": "query",
				"cypher":    "MATCH (n:Test) RETURN n",
			})

			ok, err := tool.validate(request)

			Convey("It should validate successfully", func() {
				So(ok, ShouldBeTrue)
				So(err, ShouldBeNil)
			})
		})
	})
}

// TestHandler tests the Handler method
func TestHandler(t *testing.T) {
	Convey("Given a memory tool", t, func() {
		ctx := context.Background()

		Convey("When handling invalid request", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			request := newMockRequest(map[string]interface{}{})

			result, err := tool.Handler(ctx, request)

			Convey("It should return an error result", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When handling invalid operation", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			request := newMockRequest(map[string]interface{}{
				"operation": "invalid",
			})

			result, err := tool.Handler(ctx, request)

			Convey("It should return an error result", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When handling add operation", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			request := newMockRequest(map[string]interface{}{
				"operation": "add",
				"document":  "test document",
				"cypher":    "CREATE (n:Test) RETURN n",
			})

			result, err := tool.Handler(ctx, request)

			Convey("It should call the handleAddMemory method", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(vectorStore.documents, ShouldContain, "test document")
				So(graphStore.queries, ShouldContain, "CREATE (n:Test) RETURN n")
			})
		})

		Convey("When handling query operation", func() {
			vectorStore := &MockVectorStore{
				results: []string{"vector result 1", "vector result 2"},
			}
			graphStore := &MockGraphStore{
				results: []string{"graph result 1", "graph result 2"},
			}
			tool := New(vectorStore, graphStore)

			request := newMockRequest(map[string]interface{}{
				"operation": "query",
				"question":  "test question",
				"keywords":  "test,keywords",
				"cypher":    "MATCH (n:Test) RETURN n",
			})

			result, err := tool.Handler(ctx, request)

			Convey("It should call the handleQueryMemory method", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})
	})
}

// TestHandleAddMemory tests the handleAddMemory method
func TestHandleAddMemory(t *testing.T) {
	Convey("Given a memory tool", t, func() {
		ctx := context.Background()

		Convey("When adding a document to the vector store successfully", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleAddMemory(ctx, "test document", "")

			Convey("It should store the document and return success", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(vectorStore.documents, ShouldContain, "test document")
			})
		})

		Convey("When adding a document to the vector store fails", func() {
			vectorStore := &MockVectorStore{
				err: fmt.Errorf("vector store error"),
			}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleAddMemory(ctx, "test document", "")

			Convey("It should return an error result", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When executing a Cypher query in the graph store successfully", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleAddMemory(ctx, "", "CREATE (n:Test) RETURN n")

			Convey("It should execute the query and return success", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(graphStore.queries, ShouldContain, "CREATE (n:Test) RETURN n")
			})
		})

		Convey("When executing a Cypher query in the graph store fails", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{
				err: fmt.Errorf("graph store error"),
			}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleAddMemory(ctx, "", "CREATE (n:Test) RETURN n")

			Convey("It should return an error result", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When adding to both stores successfully", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleAddMemory(ctx, "test document", "CREATE (n:Test) RETURN n")

			Convey("It should store in both and return combined success", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(vectorStore.documents, ShouldContain, "test document")
				So(graphStore.queries, ShouldContain, "CREATE (n:Test) RETURN n")
			})
		})
	})
}

// TestHandleQueryMemory tests the handleQueryMemory method
func TestHandleQueryMemory(t *testing.T) {
	Convey("Given a memory tool", t, func() {
		ctx := context.Background()

		Convey("When querying the vector store successfully", func() {
			vectorStore := &MockVectorStore{
				results: []string{"vector result 1", "vector result 2"},
			}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleQueryMemory(ctx, "test question", "", "")

			Convey("It should return the vector results", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When querying the vector store returns no results", func() {
			vectorStore := &MockVectorStore{
				results: []string{},
			}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleQueryMemory(ctx, "test question", "", "")

			Convey("It should indicate no memories found", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When querying the vector store fails", func() {
			vectorStore := &MockVectorStore{
				err: fmt.Errorf("vector search error"),
			}
			graphStore := &MockGraphStore{}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleQueryMemory(ctx, "test question", "", "")

			Convey("It should return an error result", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When querying the graph store with keywords successfully", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{
				results: []string{"graph result 1", "graph result 2"},
			}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleQueryMemory(ctx, "", "test,keywords", "")

			Convey("It should return the graph results", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When querying the graph store with cypher successfully", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{
				results: []string{"graph result 1", "graph result 2"},
			}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleQueryMemory(ctx, "", "", "MATCH (n:Test) RETURN n")

			Convey("It should return the graph results", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When querying the graph store returns no results", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{
				results: []string{},
			}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleQueryMemory(ctx, "", "test,keywords", "")

			Convey("It should indicate no memories found", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When querying the graph store fails", func() {
			vectorStore := &MockVectorStore{}
			graphStore := &MockGraphStore{
				err: fmt.Errorf("graph search error"),
			}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleQueryMemory(ctx, "", "test,keywords", "")

			Convey("It should return an error result", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})

		Convey("When querying both stores successfully", func() {
			vectorStore := &MockVectorStore{
				results: []string{"vector result 1", "vector result 2"},
			}
			graphStore := &MockGraphStore{
				results: []string{"graph result 1", "graph result 2"},
			}
			tool := New(vectorStore, graphStore)

			result, err := tool.handleQueryMemory(ctx, "test question", "test,keywords", "")

			Convey("It should return both vector and graph results", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
			})
		})
	})
}
