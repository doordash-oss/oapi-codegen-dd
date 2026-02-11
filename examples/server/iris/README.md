# Iris Server Example

This example demonstrates server code generation using [Iris](https://github.com/kataras/iris), a fast, simple yet fully featured web framework for Go.

## Description

- Iris uses `iris.Handler` for middleware
- Path parameters are extracted via `ctx.Params().Get("paramName")`
- Iris has its own context type (`iris.Context`) with convenience methods
- Built-in middleware available: `recover.New()`, `requestid.New()`, `logger.New()`, etc.

## Integrating with Existing Server

If you already have an Iris application, register the generated routes:

```go
import handler "your/module/api"

svc := handler.NewService()
handler.NewRouter(app, svc)
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

