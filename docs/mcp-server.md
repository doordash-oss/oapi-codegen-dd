# MCP Server Generation

oapi-codegen can generate [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) servers from your OpenAPI spec. MCP is Anthropic's open standard for connecting AI assistants to external tools and data sources.

## Overview

The MCP server generation feature creates:

1. **ClientInterface** - Interface for the API client (allows mocking)
2. **MCPTools** - Registers tools with the MCP server
3. **Tool handlers** - One handler per API operation

Each API operation becomes an MCP tool that AI assistants like Claude can call.

## Quick Start

Create a configuration file:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/doordash-oss/oapi-codegen-dd/HEAD/configuration-schema.json
package: api
generate:
  client: true
  mcp-server: {}
```

Run the generator:

```bash
go run github.com/doordash-oss/oapi-codegen-dd/v3/cmd/oapi-codegen -config cfg.yaml spec.yaml
```

Create a main.go to run the server:

```go
package main

import (
    "log"
    
    "github.com/mark3labs/mcp-go/server"
    "myproject/api"
)

func main() {
    // Create your client implementation
    client, _ := api.NewDefaultClient("http://localhost:8080")
    
    // Create MCP server
    s := server.NewMCPServer("My API", "1.0.0", server.WithToolCapabilities(true))
    
    // Register tools
    api.NewMCPTools(s, api.WithClient(client))
    
    // Start over stdio
    if err := server.ServeStdio(s); err != nil {
        log.Fatal(err)
    }
}
```

## Testing with MCP Inspector

```bash
# Build the server
go build -o mcp-server ./cmd

# Run with inspector
npx @modelcontextprotocol/inspector ./mcp-server
```

## Claude Desktop Integration

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "my-api": {
      "command": "/path/to/mcp-server"
    }
  }
}
```

Restart Claude Desktop and your API tools will be available.

## Configuration Options

### default-skip

By default, all operations are included in MCP generation. Use `default-skip: true` to invert this behavior:

```yaml
generate:
  client: true
  mcp-server:
    default-skip: true  # Skip all operations by default
```

| `default-skip` | Behavior |
|----------------|----------|
| `false` (default) | Include all operations unless `x-mcp.skip: true` |
| `true` | Skip all operations unless `x-mcp.skip: false` |

This is useful when you only want to expose a few operations as MCP tools:

```yaml
# cfg.yaml
generate:
  mcp-server:
    default-skip: true

# spec.yaml - only listUsers will be an MCP tool
paths:
  /users:
    get:
      operationId: listUsers
      x-mcp:
        skip: false  # Explicitly include
  /users/{id}:
    delete:
      operationId: deleteUser
      # No x-mcp - skipped due to default-skip: true
```

## x-mcp Extension

Control MCP tool generation per operation using the `x-mcp` extension:

```yaml
paths:
  /users:
    get:
      operationId: listUsers
      x-mcp:
        name: "list_users"           # Override tool name (default: operationId)
        description: "Get all users" # Override tool description

  /internal/metrics:
    get:
      operationId: getMetrics
      x-mcp:
        skip: true  # Exclude from MCP generation
```

### Extension Properties

| Property | Type | Description |
|----------|------|-------------|
| `skip` | boolean | If `true`, exclude this operation from MCP generation. If `false`, include it (useful with `default-skip: true`) |
| `name` | string | Override the tool name (default: operationId) |
| `description` | string | Override the tool description (default: operation summary/description) |

## Architecture

```
┌─────────────────┐     stdio      ┌─────────────────┐
│  AI Assistant   │◄──────────────►│   MCP Server    │
│  (Claude, etc)  │   JSON-RPC     │  (generated)    │
└─────────────────┘                └────────┬────────┘
                                            │
                                            ▼
                                   ┌─────────────────┐
                                   │  ClientInterface │
                                   │  (your impl)    │
                                   └────────┬────────┘
                                            │
                                            ▼
                                   ┌─────────────────┐
                                   │   Your API      │
                                   │   (HTTP/etc)    │
                                   └─────────────────┘
```

The MCP server acts as a bridge between AI assistants and your API. You provide a `ClientInterface` implementation - either the generated HTTP client pointing to a real server, or a mock for testing.

## Example

See the [examples/mcp](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/mcp) directory for a complete working example with tests.

