# GoFrame Server Example

This example demonstrates server generation for [GoFrame (gogf/gf)](https://github.com/gogf/gf).

## Description

- Uses `github.com/gogf/gf/v2/net/ghttp` for HTTP server
- Path parameters use `{param}` format (same as OpenAPI)
- Path params accessed via `r.Get("paramName").String()`
- Handler signature: `func(r *ghttp.Request)`
- Full-featured framework with built-in logging, tracing, and more

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

If you already have a GoFrame server, you can integrate the generated handler:

```go
import (
    "github.com/gogf/gf/v2/net/ghttp"
    handler "your-module/api"
)

func main() {
    s := ghttp.GetServer()
    
    // Your existing routes
    s.BindHandler("GET:/existing", existingHandler)
    
    // Register generated API routes
    svc := handler.NewService()
    handler.NewRouter(s, svc)
    
    s.SetPort(8080)
    s.Run()
}
```

Or with middleware:

```go
import handler "your-module/api"

func main() {
    s := ghttp.GetServer()
    
    svc := handler.NewService()
    handler.NewRouter(s, svc,
        handler.WithMiddleware(handler.ExampleMiddleware()),
    )
    
    s.SetPort(8080)
    s.Run()
}
```

