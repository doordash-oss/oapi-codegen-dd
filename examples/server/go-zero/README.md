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

// Create your service implementation
svc := handler.NewService()

// Register routes with your existing server
handler.RegisterRoutes(server, svc)
```

## Running the Server

```bash
go run ./server/main.go
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

