# Gorilla Mux Server Example

This example demonstrates server generation for [gorilla/mux](https://github.com/gorilla/mux).

## Description

- Uses `github.com/gorilla/mux` for routing
- Path parameters use `{param}` format (same as OpenAPI)
- Path params accessed via `mux.Vars(r)["paramName"]`
- Middleware signature: `func(http.Handler) http.Handler` (standard net/http)
- Supports regex path matching and subrouters

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

If you already have a gorilla/mux router, you can integrate the generated handler:

```go
import (
    "github.com/gorilla/mux"
    handler "your-module/api"
)

func main() {
    r := mux.NewRouter()
    
    // Your existing routes
    r.HandleFunc("/existing", existingHandler)
    
    // Mount generated API on a subrouter
    svc := handler.NewService()
    apiRouter := handler.NewRouter(svc)
    r.PathPrefix("/api").Handler(apiRouter)
    
    http.ListenAndServe(":8080", r)
}
```

Or use the generated router directly:

```go
import handler "your-module/api"

func main() {
    svc := handler.NewService()
    r := handler.NewRouter(svc,
        handler.WithMiddleware(handler.RecoveryMiddleware),
        handler.WithMiddleware(handler.LoggingMiddleware(log.Printf)),
    )
    
    http.ListenAndServe(":8080", r)
}
```

