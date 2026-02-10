# Gin Server Example

This example demonstrates server code generation using [Gin](https://github.com/gin-gonic/gin), a high-performance HTTP web framework.

## Description

- Gin uses `gin.HandlerFunc` for middleware
- Path parameters are extracted via `c.Param("paramName")`
- Gin has its own context type (`*gin.Context`) with convenience methods
- Built-in middleware available: `gin.Recovery()`, `gin.Logger()`, etc.
- Set `gin.SetMode(gin.ReleaseMode)` for production

## Integrating with Existing Server

If you already have a Gin engine, register the generated routes:

```go
import handler "your/module/api"

svc := handler.NewService()
handler.NewRouter(r, svc)
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

