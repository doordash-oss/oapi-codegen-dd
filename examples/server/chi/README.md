# Chi Server Example

This example demonstrates server code generation using [Chi](https://github.com/go-chi/chi), a lightweight, idiomatic HTTP router for Go.

## Description

- Chi uses standard `http.Handler` middleware pattern
- Path parameters are extracted via `chi.URLParam(r, "paramName")`
- Chi is 100% compatible with `net/http` - no custom context types
- Middleware is added via `router.Use()`

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
