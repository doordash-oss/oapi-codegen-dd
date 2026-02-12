package main

import (
	"context"
	"log"

	"github.com/doordash-oss/oapi-codegen-dd/v3/examples/mcp/gen"
	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/mark3labs/mcp-go/server"
)

// To run it this example:
// cd examples
// npx @modelcontextprotocol/inspector go run ./mcp/server/
func main() {
	// Create mock client with sample data
	client := &mockClient{
		users: []gen.User{
			{ID: "1", Name: "Alice", Email: "alice@example.com"},
			{ID: "2", Name: "Bob", Email: "bob@example.com"},
		},
	}

	// Create the MCP server
	s := server.NewMCPServer(
		"User API MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register all MCP tools
	gen.NewMCPTools(s, gen.WithClient(client))

	// Start the MCP server over stdio
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// mockClient implements gen.ClientInterface with in-memory data
type mockClient struct {
	users []gen.User
}

func (m *mockClient) HealthCheck(ctx context.Context, reqEditors ...runtime.RequestEditorFn) (*gen.HealthCheckResponse, error) {
	return &gen.HealthCheckResponse{Status: "ok"}, nil
}

func (m *mockClient) ListUsers(ctx context.Context, opts *gen.ListUsersRequestOptions, reqEditors ...runtime.RequestEditorFn) (*gen.ListUsersResponse, error) {
	result := gen.ListUsersResponse(m.users)
	return &result, nil
}

func (m *mockClient) CreateUser(ctx context.Context, opts *gen.CreateUserRequestOptions, reqEditors ...runtime.RequestEditorFn) (*gen.CreateUserResponse, error) {
	user := gen.User{
		ID:    "new-id",
		Name:  opts.Body.Name,
		Email: opts.Body.Email,
	}
	m.users = append(m.users, user)
	return &gen.CreateUserResponse{ID: user.ID, Name: user.Name, Email: user.Email}, nil
}

func (m *mockClient) GetUser(ctx context.Context, opts *gen.GetUserRequestOptions, reqEditors ...runtime.RequestEditorFn) (*gen.GetUserResponse, error) {
	for _, u := range m.users {
		if u.ID == opts.PathParams.ID {
			return &gen.GetUserResponse{ID: u.ID, Name: u.Name, Email: u.Email}, nil
		}
	}
	return &gen.GetUserResponse{ID: opts.PathParams.ID, Name: "Unknown", Email: "unknown@example.com"}, nil
}

func (m *mockClient) DeleteUser(ctx context.Context, opts *gen.DeleteUserRequestOptions, reqEditors ...runtime.RequestEditorFn) (*struct{}, error) {
	return nil, nil
}

func (m *mockClient) GetMetrics(ctx context.Context, reqEditors ...runtime.RequestEditorFn) (*gen.GetMetricsResponse, error) {
	return nil, nil
}
