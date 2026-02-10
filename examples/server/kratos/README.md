# Kratos Server Example

This example demonstrates server code generation using [Kratos](https://github.com/go-kratos/kratos), a cloud-native Go microservices framework.

## Description

- Kratos uses gorilla/mux under the hood for HTTP routing
- Path parameters are extracted via `mux.Vars(r)["paramName"]`
- Routes are registered via `server.Route("/").GET("/path", handler)`
- Built-in middleware: recovery, logging, tracing, metrics, validation
- Server created with `http.NewServer(opts...)`

## Integrating with Existing Server

If you already have a Kratos HTTP server, register the generated routes:

```go
import handler "your/module/api"

svc := handler.NewService()
handler.RegisterRoutes(httpSrv, svc)
```

## Integrating with Kratos Project Structure

For projects using Kratos's standard layout (`kratos new`), the generated code maps as follows:

```
├── api/              # proto files (not needed for OpenAPI)
├── cmd/server/       # main.go, wire.go
├── internal/
│   ├── biz/          # business logic (domain layer)
│   ├── data/         # data access
│   ├── server/       # ← http.go - call RegisterRoutes() here
│   └── service/      # ← service.go (business logic)
```

Example integration in `internal/server/http.go`:

```go
import (
    "your/module/internal/handler"
    "your/module/internal/service"
    "github.com/go-kratos/kratos/v2/transport/http"
)

func NewHTTPServer(c *conf.Server, svc *service.YourService) *http.Server {
    srv := http.NewServer(
        http.Address(c.Http.Addr),
        http.Timeout(c.Http.Timeout.AsDuration()),
    )
    
    handler.RegisterRoutes(srv, svc)
    
    return srv
}
```

## Running the Server

```bash
go run ./server
```

The server starts on port 8080.

## API Endpoints

### Health Check

```bash
curl http://localhost:8080/health
```

### List Users

```bash
curl http://localhost:8080/users
```

With optional limit parameter:

```bash
curl "http://localhost:8080/users?limit=10"
```

### Create User

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name": "John Doe", "email": "john@example.com"}'
```

### Get User by ID

```bash
curl http://localhost:8080/users/123
```

### Delete User

```bash
curl -X DELETE http://localhost:8080/users/123
```

