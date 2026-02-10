# Hertz Server Example

This example demonstrates server generation for [Hertz (cloudwego/hertz)](https://github.com/cloudwego/hertz).

## Description

- Uses `github.com/cloudwego/hertz/pkg/app/server` for HTTP server
- Path parameters use `{param}` format (same as OpenAPI)
- Path params accessed via `c.Param("paramName")`
- Handler signature: `func(ctx context.Context, c *app.RequestContext)`
- High-performance framework from ByteDance with built-in recovery

## Running the Server

```bash
go run ./server
```

## API Endpoints

### Health Check
```bash
curl http://localhost:8080/health
```

### List Users
```bash
curl http://localhost:8080/users
```

### Create User
```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name": "John Doe", "email": "john@example.com"}'
```

### Get User
```bash
curl http://localhost:8080/users/123
```

### Delete User
```bash
curl -X DELETE http://localhost:8080/users/123
```

## Integrating with Existing Server

If you already have a Hertz server, you can integrate the generated handler:

```go
import (
    "github.com/cloudwego/hertz/pkg/app/server"
    handler "your-module/api"
)

func main() {
    h := server.Default()
    
    // Your existing routes
    h.GET("/existing", existingHandler)
    
    // Register generated API routes
    svc := handler.NewService()
    handler.NewRouter(h, svc)
    
    h.Spin()
}
```

Or with middleware:

```go
import handler "your-module/api"

func main() {
    h := server.Default()
    
    svc := handler.NewService()
    handler.NewRouter(h, svc,
        handler.WithMiddleware(handler.ExampleMiddleware()),
    )
    
    h.Spin()
}
```

