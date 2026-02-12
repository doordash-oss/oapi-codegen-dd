# x-mcp

The `x-mcp` extension controls MCP (Model Context Protocol) tool generation for individual operations.

## Usage

Apply to operations to customize or skip MCP tool generation:

```yaml
paths:
  /users:
    get:
      operationId: listUsers
      summary: List all users
      x-mcp:
        name: "list_users"
        description: "Retrieve all users from the system"
    
  /internal/metrics:
    get:
      operationId: getMetrics
      x-mcp:
        skip: true
```

## Properties

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `skip` | boolean | `false` | If `true`, exclude this operation from MCP generation |
| `name` | string | operationId | Override the MCP tool name |
| `description` | string | summary/description | Override the MCP tool description |

## Examples

### Skip an operation

```yaml
/admin/reset:
  post:
    operationId: resetDatabase
    x-mcp:
      skip: true  # Don't expose to AI assistants
```

### Custom tool name

```yaml
/users:
  get:
    operationId: listUsers
    x-mcp:
      name: "get_all_users"  # More descriptive for AI
```

### Custom description

```yaml
/users/{id}:
  delete:
    operationId: deleteUser
    x-mcp:
      description: "Permanently delete a user. This action cannot be undone."
```

## Interaction with default-skip

The `skip` property interacts with the `default-skip` configuration option:

| `default-skip` | `x-mcp.skip` | Result |
|----------------|--------------|--------|
| `false` (default) | not set | Included |
| `false` | `true` | Skipped |
| `false` | `false` | Included |
| `true` | not set | Skipped |
| `true` | `true` | Skipped |
| `true` | `false` | Included |

Example with `default-skip: true`:

```yaml
# cfg.yaml
generate:
  mcp-server:
    default-skip: true

# spec.yaml
paths:
  /users:
    get:
      operationId: listUsers
      x-mcp:
        skip: false  # Explicitly include this operation
```

## Notes

- By default, operations without `x-mcp` are included (unless `default-skip: true`)
- The `name` defaults to the `operationId`
- The `description` defaults to the operation's `summary` or `description`
- This extension only affects MCP server generation (`generate.mcp-server`)

## See Also

- [MCP Server Generation](../mcp-server.md)

