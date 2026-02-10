# Fiber Server Example

This example demonstrates server code generation using [Fiber](https://github.com/gofiber/fiber), an Express-inspired web framework built on Fasthttp.

## Description

- Fiber uses `fiber.Handler` for middleware
- Path parameters are extracted via `c.Params("paramName")`
- Fiber is built on Fasthttp, not `net/http` - uses its own request/response types
- The adapter converts between Fiber's context and standard `http.Request`/`http.ResponseWriter`
- Built-in middleware available: `recover.New()`, `logger.New()`, `cors.New()`, etc.

## Integrating with Existing Server

If you already have a Fiber app, register the generated routes:

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

