# Echo Server Example

This example demonstrates server code generation using [Echo](https://github.com/labstack/echo), a high-performance, minimalist Go web framework.

## Description

- Echo uses `echo.MiddlewareFunc` for middleware
- Path parameters are extracted via `c.Param("paramName")`
- Echo has its own context type (`echo.Context`) with convenience methods
- Built-in middleware available: `middleware.Recover()`, `middleware.Logger()`, `middleware.CORS()`, etc.

## Integrating with Existing Server

If you already have an Echo instance, register the generated routes:

```go
import handler "your/module/api"

svc := handler.NewService()
handler.NewRouter(e, svc)
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
