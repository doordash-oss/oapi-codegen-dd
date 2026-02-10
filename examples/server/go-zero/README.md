# go-zero Server Example

This example demonstrates server code generation using [go-zero](https://github.com/zeromicro/go-zero), a cloud-native Go microservices framework.

## Description

- go-zero uses `rest.Middleware` (`func(next http.HandlerFunc) http.HandlerFunc`) for middleware
- Path parameters are extracted via `pathvar.Vars(r)["paramName"]`
- Routes are registered via `server.AddRoutes([]rest.Route{...})`
- Built-in features: recovery, logging, timeout, circuit breaker, rate limiting, load shedding
- Server created with `rest.MustNewServer(rest.RestConf{...})`

## Integrating with Existing Server

If you already have a go-zero server, register the generated routes:

```go
import handler "your/module/api"

svc := handler.NewService()
handler.RegisterRoutes(server, svc)
```

## Integrating with go-zero Project Structure

For projects using go-zero's standard layout (`goctl api new`), the generated code maps as follows:

```
├── internal
│   ├── handler/          # ← gen.go (adapter, routes)
│   ├── logic/            # ← service.go (business logic)
│   ├── svc/              # ← pass dependencies to NewService()
│   └── types/            # ← generated request/response types (in gen.go)
```

Example integration in `greet.go`:

```go
import (
    "your/module/internal/handler"
    "your/module/internal/svc"
)

func main() {
    // ... server setup ...

    ctx := svc.NewServiceContext(c)
    svc := handler.NewServiceWithContext(ctx) // implement this wrapper
    handler.RegisterRoutes(server, svc)

    server.Start()
}
```

The `ServiceInterface` can wrap your `svc.ServiceContext` to access config, redis, etc.

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

