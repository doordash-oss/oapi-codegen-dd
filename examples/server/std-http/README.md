# Standard Library HTTP Server Example

This example demonstrates server code generation using Go's [standard library net/http](https://pkg.go.dev/net/http) package.

## Description

- Uses standard `http.Handler` and `http.HandlerFunc` patterns
- Path parameters are extracted using Go 1.22+ `http.Request.PathValue()` method
- No external dependencies required
- Middleware uses the standard `func(http.Handler) http.Handler` pattern
- Compatible with any `net/http` compatible middleware

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

