package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mcpClient struct {
	cmd    *exec.Cmd
	stdin  *json.Encoder
	stdout *bufio.Reader
	nextID int
}

func startMCPServer(t *testing.T) *mcpClient {
	// Build the server binary
	binPath := t.TempDir() + "/mcp-server"
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = "."
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build failed: %s", out)

	// Start the server
	cmd := exec.Command(binPath)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = os.Stderr

	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	client := &mcpClient{
		cmd:    cmd,
		stdin:  json.NewEncoder(stdin),
		stdout: bufio.NewReader(stdout),
		nextID: 1,
	}

	// Initialize the MCP connection
	client.initialize(t)

	return client
}

func (c *mcpClient) initialize(t *testing.T) {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}
	c.nextID++

	require.NoError(t, c.stdin.Encode(req))

	line, err := c.stdout.ReadBytes('\n')
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(line, &resp))
	require.Contains(t, resp, "result")
}

func (c *mcpClient) callTool(t *testing.T, name string, args map[string]any) map[string]any {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	c.nextID++

	require.NoError(t, c.stdin.Encode(req))

	line, err := c.stdout.ReadBytes('\n')
	require.NoError(t, err)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(line, &resp))
	return resp
}

func TestListUsers(t *testing.T) {
	client := startMCPServer(t)

	resp := client.callTool(t, "list_users", map[string]any{})

	result := resp["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	text := content["text"].(string)

	var users []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &users))

	assert.Len(t, users, 2)
	assert.Equal(t, "Alice", users[0]["name"])
	assert.Equal(t, "Bob", users[1]["name"])
}

func TestGetUser(t *testing.T) {
	client := startMCPServer(t)

	resp := client.callTool(t, "GetUser", map[string]any{"id": "1"})

	result := resp["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	text := content["text"].(string)

	var user map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &user))

	assert.Equal(t, "1", user["id"])
	assert.Equal(t, "Alice", user["name"])
}

func TestCreateUser(t *testing.T) {
	client := startMCPServer(t)

	resp := client.callTool(t, "CreateUser", map[string]any{
		"body": map[string]any{
			"name":  "Charlie",
			"email": "charlie@example.com",
		},
	})

	result := resp["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	text := content["text"].(string)

	var user map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &user))

	assert.Equal(t, "new-id", user["id"])
	assert.Equal(t, "Charlie", user["name"])
	assert.Equal(t, "charlie@example.com", user["email"])
}

func TestHealthCheck(t *testing.T) {
	client := startMCPServer(t)

	resp := client.callTool(t, "HealthCheck", map[string]any{})

	result := resp["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	text := content["text"].(string)

	assert.Contains(t, text, "ok")
}
