# Server Generation

oapi-codegen can generate server-side handler code with a clean service interface pattern. This separates HTTP handling from business logic, making your code easier to test and maintain.

## Overview

The server generation feature creates:

1. **Service Interface** - A Go interface defining your business logic methods
2. **HTTP Adapter** - Generated code that handles HTTP parsing and calls your service
3. **Router Registration** - Framework-specific code to register routes
4. **Scaffold Files** - One-time generated files for your implementation (`service.go`, `middleware.go`)
5. **Server Main** (optional) - A runnable `main.go` with middleware setup

## Quick Start

Create a configuration file:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/doordash-oss/oapi-codegen-dd/HEAD/configuration-schema.json
package: api
output:
  directory: api
generate:
  handler:
    kind: chi  # or echo, gin, fiber, std-http, etc.
    middleware: {}
    server:
      directory: server
      handler-package: github.com/myorg/myapi/api
```

Run the generator:

```bash
go run github.com/doordash-oss/oapi-codegen-dd/v3/cmd/oapi-codegen -config cfg.yaml spec.yaml
```

This generates:

```
api/
├── gen.go          # Generated types, adapter, router (always regenerated)
├── service.go      # Your service implementation (scaffold, edit this)
└── middleware.go   # Custom middleware (scaffold, edit this)
server/
└── main.go         # Runnable server (scaffold, edit this)
```

## Supported Frameworks

| Framework | Kind | Path Params | Notes |
|-----------|------|-------------|-------|
| [chi](https://github.com/go-chi/chi) | `chi` | `chi.URLParam(r, "id")` | Lightweight, idiomatic |
| [Echo](https://github.com/labstack/echo) | `echo` | `c.Param("id")` | Feature-rich, middleware ecosystem |
| [Gin](https://github.com/gin-gonic/gin) | `gin` | `c.Param("id")` | High performance, popular |
| [Fiber](https://github.com/gofiber/fiber) | `fiber` | `c.Params("id")` | Express-inspired, fasthttp-based |
| [std-http](https://pkg.go.dev/net/http) | `std-http` | `r.PathValue("id")` | Go 1.22+ standard library |
| [Beego](https://github.com/beego/beego) | `beego` | `c.Ctx.Input.Param(":id")` | Full-stack framework |
| [go-zero](https://github.com/zeromicro/go-zero) | `go-zero` | `r.PathValue("id")` | Microservice framework |
| [Kratos](https://github.com/go-kratos/kratos) | `kratos` | `r.PathValue("id")` | Microservice framework |
| [Gorilla Mux](https://github.com/gorilla/mux) | `gorilla-mux` | `mux.Vars(r)["id"]` | Classic router |
| [GoFrame](https://github.com/gogf/gf) | `goframe` | `r.Get("id")` | Full-stack framework |
| [Hertz](https://github.com/cloudwego/hertz) | `hertz` | `c.Param("id")` | High-performance from ByteDance |
| [Iris](https://github.com/kataras/iris) | `iris` | `ctx.Params().Get("id")` | Feature-rich, MVC support |
| [fasthttp](https://github.com/valyala/fasthttp) | `fasthttp` | `ctx.UserValue("id")` | Zero-allocation HTTP |

## Architecture

The generated code follows a clean architecture pattern:

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP Request                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Router (chi/echo/gin/...)                │
│                    Routes registered by NewRouter()         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      HTTPAdapter                            │
│  • Parses path/query/body parameters                        │
│  • Validates request (optional)                             │
│  • Calls ServiceInterface method                            │
│  • Writes response                                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   ServiceInterface                          │
│  • Pure business logic                                      │
│  • No HTTP concerns                                         │
│  • Easy to test                                             │
└─────────────────────────────────────────────────────────────┘
```

## Configuration Reference

### `generate.handler.kind`

**Required.** The router framework to generate for.

```yaml
generate:
  handler:
    kind: chi
```

### `generate.handler.name`

Name of the generated service interface. Default: `"Service"`.

```yaml
generate:
  handler:
    kind: chi
    name: "UserAPI"  # Generates UserAPIInterface
```

### `generate.handler.models-package-alias`

When models are in a separate package, prefix types with this alias.

```yaml
generate:
  models: false  # Don't generate models here
  handler:
    kind: chi
    models-package-alias: types  # Use types.User instead of User
```

### `generate.handler.validation`

Enable request/response validation in handlers.

```yaml
generate:
  handler:
    kind: chi
    validation:
      request: true   # Validate incoming requests
      response: true  # Validate outgoing responses (for testing)
```

### `generate.handler.output`

Control where scaffold files are written.

```yaml
generate:
  handler:
    kind: chi
    output:
      directory: api      # Where to write service.go, middleware.go
      package: api        # Package name for scaffold files
      overwrite: true     # Force regenerate scaffold files
```

### `generate.handler.middleware`

Enable middleware scaffold generation. Set to `{}` to enable.

```yaml
generate:
  handler:
    kind: chi
    middleware: {}  # Generates middleware.go
```

### `generate.handler.server`

Generate a runnable server with middleware setup.

```yaml
generate:
  handler:
    kind: chi
    server:
      directory: server                              # Output directory
      port: 8080                                     # Server port
      timeout: 30                                    # Request timeout (seconds)
      handler-package: github.com/myorg/myapi/api   # Import path for handler
```

## Generated Code

### Service Interface

For each operation in your OpenAPI spec, a method is generated on the service interface:

```go
type ServiceInterface interface {
    // HealthCheck Health check endpoint
    HealthCheck(ctx context.Context) (*HealthCheckResponseData, error)

    // CreateUser Create a new user
    CreateUser(ctx context.Context, opts *CreateUserServiceRequestOptions) (*CreateUserResponseData, error)

    // GetUser Get a user by ID
    GetUser(ctx context.Context, opts *GetUserServiceRequestOptions) (*GetUserResponseData, error)
}
```

### Request Options

Operations with parameters receive a `*<Operation>ServiceRequestOptions` struct:

```go
type CreateUserServiceRequestOptions struct {
    RawRequest *http.Request      // Original HTTP request
    Body       *CreateUserRequest // Parsed request body
}

type GetUserServiceRequestOptions struct {
    RawRequest *http.Request   // Original HTTP request
    PathParams *GetUserPathParams // Path parameters
    Query      *GetUserQuery      // Query parameters
}
```

### Form-Encoded Requests

When your OpenAPI spec defines `application/x-www-form-urlencoded` as the request content type, 
the generated adapter automatically parses form data into your typed request body.

The adapter supports **deepObject encoding** as defined in OpenAPI 3.0, which allows nested objects and arrays in form data:

```
# Simple key-value pairs
name=John&age=30&active=true

# Nested objects (deepObject style)
address[city]=Berlin&address[country]=DE

# Arrays with indices
items[0]=first&items[1]=second

# Complex nested structures (e.g., Stripe API style)
flow_data[subscription][items][0][id]=si_123&flow_data[subscription][items][0][quantity]=2
```

**Type conversion** is handled automatically:

| Form Value | Converted To |
|------------|--------------|
| `true`, `false` | `bool` |
| `42`, `-10` | `int64` |
| `3.14`, `0.05` | `float64` |
| Other values | `string` |

**Conservative conversion** preserves string values that look like numbers but shouldn't be converted:

- Phone numbers starting with `+` (e.g., `+1234567890`)
- Values with leading zeros (e.g., `00123`)
- Values containing spaces or parentheses

This enables seamless integration with APIs like Stripe that use complex form-encoded request bodies.

### Response Data

Return a `*<Operation>ResponseData` from your service method:

```go
func (s *Service) GetUser(ctx context.Context, opts *GetUserServiceRequestOptions) (*GetUserResponseData, error) {
    user, err := s.db.FindUser(opts.PathParams.Id)
    if err != nil {
        return nil, err
    }
    if user == nil {
        return NewGetUserResponseData(&GetUserResponse404{
            Code:    "not_found",
            Message: "User not found",
        }), nil
    }
    return NewGetUserResponseData(&GetUserResponse200{
        Id:    user.ID,
        Name:  user.Name,
        Email: user.Email,
    }), nil
}
```

You can also set custom headers and status codes:

```go
resp := NewGetUserResponseData(&GetUserResponse200{...})
resp.Status = 200
resp.Headers = http.Header{
    "X-Custom-Header": []string{"value"},
}
return resp, nil
```

## Integrating with Existing Applications

### Adding to an Existing Router

Each framework has a `NewRouter` function to register routes:

=== "chi"

    ```go
    import (
        "github.com/go-chi/chi/v5"
        handler "your-module/api"
    )

    func main() {
        r := chi.NewRouter()

        // Your existing routes
        r.Get("/existing", existingHandler)

        // Register generated API routes
        svc := handler.NewService()
        handler.NewRouter(r, svc)

        http.ListenAndServe(":8080", r)
    }
    ```

=== "echo"

    ```go
    import (
        "github.com/labstack/echo/v4"
        handler "your-module/api"
    )

    func main() {
        e := echo.New()

        // Your existing routes
        e.GET("/existing", existingHandler)

        // Register generated API routes
        svc := handler.NewService()
        handler.NewRouter(e, svc)

        e.Start(":8080")
    }
    ```

=== "gin"

    ```go
    import (
        "github.com/gin-gonic/gin"
        handler "your-module/api"
    )

    func main() {
        r := gin.Default()

        // Your existing routes
        r.GET("/existing", existingHandler)

        // Register generated API routes
        svc := handler.NewService()
        handler.NewRouter(r, svc)

        r.Run(":8080")
    }
    ```

### Adding Middleware

Use `WithMiddleware` to add framework-specific middleware:

```go
svc := handler.NewService()
handler.NewRouter(r, svc,
    handler.WithMiddleware(handler.ExampleMiddleware()),
    handler.WithMiddleware(loggingMiddleware),
)
```

## Testing

The generated code is designed for easy testing. Use the `Handler()` function (available for frameworks with custom signatures) or create a test server:

```go
func TestGetUser(t *testing.T) {
    // Create a mock service
    svc := &MockService{
        GetUserFunc: func(ctx context.Context, opts *api.GetUserServiceRequestOptions) (*api.GetUserResponseData, error) {
            return api.NewGetUserResponseData(&api.GetUserResponse200{
                Id:    opts.PathParams.Id,
                Name:  "Test User",
                Email: "test@example.com",
            }), nil
        },
    }

    // Create handler
    handler := api.Handler(svc)  // or api.NewRouter(chi.NewRouter(), svc)

    // Test with httptest
    req := httptest.NewRequest("GET", "/users/123", nil)
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    assert.Equal(t, 200, rec.Code)
}
```

## Error Handling

The generated code includes a flexible error handling system that separates error classification from error response formatting.

### Error Types

The `HTTPAdapter` handles four types of errors:

| Error Kind | Description | Default Status |
|------------|-------------|----------------|
| `OapiErrorKindParse` | Parameter parsing errors (invalid path/query/header) | 400 |
| `OapiErrorKindDecode` | Request body decoding errors (invalid JSON, form data) | 400 |
| `OapiErrorKindValidation` | Request validation errors (failed schema validation) | 400 |
| `OapiErrorKindService` | Service/business logic errors from your implementation | 500 (or typed) |

### Default Behavior

The `OapiDefaultErrorHandler` respects the `Accept` header:

- **JSON** (`application/json`, `*/*`, or empty): Returns JSON response
- **Other**: Returns plain text

```go
// JSON error response for parse/decode/validation errors
{
    "error": "invalid parameter \"id\": strconv.Atoi: parsing \"abc\": invalid syntax"
}
```

Service errors are JSON-encoded directly, so the response structure matches your error type's JSON tags.

### Custom Error Handler

Implement the `OapiErrorHandler` interface to customize error responses, add logging, or collect metrics:

```go
type OapiErrorHandler interface {
    HandleError(w http.ResponseWriter, r *http.Request, statusCode int, err error)
}
```

The `err` parameter is either:

- **`OapiHandlerError`** - A generic handler error (parse, decode, validation) with context fields
- **Typed error** - An error type from your OpenAPI spec (when `error-mapping` is configured)

```go
type OapiHandlerError struct {
    Kind          OapiErrorKind  // Type of error (Parse, Decode, Validation, Service)
    OperationID   string         // OpenAPI operation ID (e.g., "GetUser", "CreateOrder")
    Message       string         // Error message
    ParamName     string         // Parameter name (for parse errors)
    ParamLocation string         // Parameter location: "path", "query", "header" (for parse errors)
}
```

Example custom handler with logging:

```go
type LoggingErrorHandler struct {
    logger *slog.Logger
}

func (h *LoggingErrorHandler) HandleError(w http.ResponseWriter, r *http.Request, statusCode int, err error) {
    // Check if it's a generic handler error
    if handlerErr, ok := err.(api.OapiHandlerError); ok {
        h.logger.Error("request error",
            "operation", handlerErr.OperationID,
            "kind", handlerErr.Kind,
            "error", handlerErr.Message,
            "status", statusCode,
            "param", handlerErr.ParamName,
            "param_location", handlerErr.ParamLocation,
        )
    } else {
        // Typed error from OpenAPI spec or service error
        h.logger.Error("service error",
            "error", err.Error(),
            "status", statusCode,
        )
    }

    // Write response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)

    if handlerErr, ok := err.(api.OapiHandlerError); ok {
        json.NewEncoder(w).Encode(map[string]string{
            "error": handlerErr.Message,
        })
    } else {
        // Typed error - encode directly to match API contract
        json.NewEncoder(w).Encode(err)
    }
}
```

Use your custom handler with `WithErrorHandler`:

```go
svc := api.NewService()
handler := api.NewRouter(svc,
    api.WithErrorHandler(&LoggingErrorHandler{logger: slog.Default()}),
)
```

### Typed Error Responses

When your OpenAPI spec defines error response types and you configure `error-mapping`, the generator creates typed errors with constructors:

```yaml
# cfg.yaml
error-mapping:
  InvalidRequestError: error.message
```

This generates:

```go
func NewInvalidRequestError(message string) InvalidRequestError {
    return InvalidRequestError{ErrorData: &ErrorData{Message: message}}
}
```

Use in your service implementation:

```go
func (s *Service) CreateUser(ctx context.Context, opts *CreateUserOpts) (*CreateUserResponse, error) {
    if opts.Body.Email == "" {
        return nil, NewInvalidRequestError("email is required")
    }
    // ...
}
```

The `HTTPAdapter` automatically detects typed errors and uses the appropriate status code from your OpenAPI spec.

## Examples

Complete examples for each framework are available in the repository:

- [chi](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/chi)
- [echo](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/echo)
- [gin](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/gin)
- [fiber](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/fiber)
- [std-http](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/std-http)
- [beego](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/beego)
- [go-zero](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/go-zero)
- [kratos](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/kratos)
- [gorilla-mux](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/gorilla-mux)
- [goframe](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/goframe)
- [hertz](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/hertz)
- [iris](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/iris)
- [fasthttp](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/server/fasthttp)

Each example includes:

- `cfg.yml` - Configuration file
- `api/gen.go` - Generated handler code
- `api/service.go` - Service implementation scaffold
- `api/middleware.go` - Middleware scaffold
- `server/main.go` - Runnable server
- `README.md` - Framework-specific documentation
