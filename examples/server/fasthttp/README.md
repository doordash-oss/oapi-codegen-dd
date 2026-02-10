# fasthttp Server Example

This example demonstrates server generation for [fasthttp](https://github.com/valyala/fasthttp).

## Description

- Uses `github.com/fasthttp/router` for routing
- Path parameters use `{param}` format (same as OpenAPI)
- Middleware signature: `func(fasthttp.RequestHandler) fasthttp.RequestHandler`
- Uses `fasthttpadaptor` to convert net/http handlers to fasthttp handlers
- High-performance alternative to net/http

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

If you already have a `fasthttp` server, you can integrate the generated handler:

```go
import (
    "github.com/valyala/fasthttp"
    handler "your-module/api"
)

func main() {
    svc := handler.NewService()
    
    // Get the handler directly
    h := handler.Handler(svc,
        handler.WithMiddleware(handler.RecoveryMiddleware()),
    )
    
    // Use with fasthttp
    fasthttp.ListenAndServe(":8080", h)
}
```

Or get the router for more control:

```go
import (
    handler "your-module/api"
)

func main() {
    svc := handler.NewService()
    
    // Get the router
    r := handler.NewRouter(svc)
    
    // Add more routes
    r.GET("/custom", customHandler)
    
    fasthttp.ListenAndServe(":8080", r.Handler)
}
```

