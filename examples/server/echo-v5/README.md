# Echo v5 Server Example

This example demonstrates server code generation using [Echo v5](https://github.com/labstack/echo), a high-performance, minimalist Go web framework.

## Description

- Echo v5 uses `echo.MiddlewareFunc` for middleware (handler signature changed to `func(c *echo.Context) error`)
- Path parameters are extracted via `c.PathParam("paramName")`
- Echo v5 uses a pointer receiver `*echo.Context` instead of the v4 interface
- Built-in middleware available: `middleware.Recover()`, `middleware.Logger()`, `middleware.CORS()`, etc.
- Graceful shutdown is handled via `echo.StartConfig.Start(ctx, e)`

## Integrating with Existing Server

If you already have an Echo v5 instance, register the generated routes:

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
