# Beego Server Example

This example demonstrates server code generation using [Beego](https://github.com/beego/beego), a full-featured MVC framework for Go.

## Description

- Beego uses `FilterFunc` for middleware via `InsertFilter`
- Recovery is enabled via `web.BConfig.RecoverPanic = true`
- Routes are registered on `*web.ControllerRegister` (accessed via `app.Handlers`)
- Path parameters are extracted from beego's context and copied to the standard `http.Request`

## Integrating with Existing Server

If you already have a Beego server, register the generated routes:

```go
import handler "your/module/api"

svc := handler.NewService()
handler.RegisterRoutes(app.Handlers, svc)
```

## Integrating with Beego Project Structure

For projects using Beego's standard MVC layout (`bee new`), the generated code maps as follows:

```
├── conf/
│   └── app.conf
├── controllers/      # ← gen.go (adapter, routes)
├── models/           # ← generated request/response types (in gen.go)
├── routers/
│   └── router.go     # ← call RegisterRoutes() here
├── services/         # ← service.go (business logic) - create this
└── main.go
```

Example integration in `routers/router.go`:

```go
import (
    "your/module/controllers"
    "your/module/services"
    beego "github.com/beego/beego/v2/server/web"
)

func init() {
    svc := services.NewService()
    controllers.RegisterRoutes(beego.BeeApp.Handlers, svc)
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

