# Migrating from v2

There are some incompatible changes that were introduced in v3 of the codegen.<br/>

## Extensions:

The following extensions are no longer supported:<br/>
- `x-order`<br/>
- `x-oapi-codegen-only-honour-go-name`

## User templates

HTTP path is not supported

## Custom name normalizer

Not supported

## Server code generation

Server code generation is now supported with a completely redesigned architecture. v3 uses a unified `generate.handler` configuration with a clean service interface pattern that separates HTTP concerns from business logic.

### Architecture comparison

| Aspect | v2 | v3 |
|--------|----|----|
| **Interface pattern** | `ServerInterface` with HTTP types in signature | `ServiceInterface` with typed request/response structs |
| **Handler signature** | `FindPets(w http.ResponseWriter, r *http.Request, params FindPetsParams)` | `FindPets(ctx context.Context, opts *FindPetsServiceRequestOptions) (*FindPetsResponseData, error)` |
| **Response handling** | Manual JSON encoding and status codes | Return typed response, adapter handles encoding |
| **Request parsing** | Parameters parsed, body manual | All parsing done by adapter |
| **Middleware** | Framework-specific, manual setup | Scaffold generated with examples |
| **Server main.go** | Not generated | Optional generation with full middleware stack |
| **Scaffold files** | Not generated | `service.go`, `middleware.go` generated once |

### Code comparison

=== "v2 Handler"

    ```go
    // v2: You implement ServerInterface with HTTP types
    type ServerInterface interface {
        // (GET /pets)
        FindPets(w http.ResponseWriter, r *http.Request, params FindPetsParams)
    }

    // Your implementation mixes HTTP and business logic
    func (s *Server) FindPets(w http.ResponseWriter, r *http.Request, params FindPetsParams) {
        pets, err := s.db.FindPets(params.Tags, params.Limit)
        if err != nil {
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
            return
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(pets)
    }
    ```

=== "v3 Service"

    ```go
    // v3: You implement ServiceInterface with typed structs
    type ServiceInterface interface {
        FindPets(ctx context.Context, opts *FindPetsServiceRequestOptions) (*FindPetsResponseData, error)
    }

    // Your implementation is pure business logic
    func (s *Service) FindPets(ctx context.Context, opts *FindPetsServiceRequestOptions) (*FindPetsResponseData, error) {
        pets, err := s.db.FindPets(opts.Query.Tags, opts.Query.Limit)
        if err != nil {
            return nil, err  // Adapter returns 500
        }
        return NewFindPetsResponseData(&FindPetsResponse200{
            Pets: pets,
        }), nil
    }
    ```

### Configuration migration

=== "v2 config"

    ```yaml
    package: api
    output: gen.go
    generate:
      chi-server: true
      models: true
    ```

=== "v3 config"

    ```yaml
    package: api
    output:
      directory: api
    generate:
      handler:
        kind: chi
        middleware: {}
        server:
          directory: server
          handler-package: github.com/myorg/myapi/api
    ```

### Framework support comparison

| Framework | v2 | v3 |
|-----------|----|----|
| [chi](https://github.com/go-chi/chi) | ✅ `chi-server` | ✅ `chi` |
| [Echo](https://github.com/labstack/echo) | ✅ `echo-server` | ✅ `echo` |
| [Gin](https://github.com/gin-gonic/gin) | ✅ `gin-server` | ✅ `gin` |
| [Fiber](https://github.com/gofiber/fiber) | ✅ `fiber-server` | ✅ `fiber` |
| [gorilla/mux](https://github.com/gorilla/mux) | ✅ `gorilla-server` | ✅ `gorilla-mux` |
| [std-http](https://pkg.go.dev/net/http) | ✅ `std-http-server` | ✅ `std-http` |
| [Iris](https://github.com/kataras/iris) | ✅ `iris-server` | ✅ `iris` |
| strict-server | ✅ `strict-server` | ❌ (service pattern is similar) |
| [Beego](https://github.com/beego/beego) | ❌ | ✅ `beego` |
| [go-zero](https://github.com/zeromicro/go-zero) | ❌ | ✅ `go-zero` |
| [Kratos](https://github.com/go-kratos/kratos) | ❌ | ✅ `kratos` |
| [GoFrame](https://github.com/gogf/gf) | ❌ | ✅ `goframe` |
| [Hertz](https://github.com/cloudwego/hertz) | ❌ | ✅ `hertz` |
| [fasthttp](https://github.com/valyala/fasthttp) | ❌ | ✅ `fasthttp` |

!!! note "About strict-server"
    v2's `strict-server` provided typed request/response objects similar to v3's service pattern. If you were using `strict-server`, the v3 service interface pattern should feel familiar, but with cleaner separation and scaffold generation.

See [Server Generation](server-generation.md) for complete documentation.

## Overlay support

v3 supports [OpenAPI Overlays](overlays.md), allowing you to modify specs without editing the original files:

```yaml
overlay:
  sources:
    - ./overlays/add-go-names.yaml
    - https://example.com/shared-overlay.yaml
```

See [Overlays documentation](overlays.md) for details.

## Configuration changes

```yaml
package: ✅
generate: ✅
    iris-server: ❌ not supported
    chi-server: ➡️ use generate.handler.kind: chi
    fiber-server: ➡️ use generate.handler.kind: fiber
    echo-server: ➡️ use generate.handler.kind: echo
    gin-server: ➡️ use generate.handler.kind: gin
    gorilla-server: ➡️ use generate.handler.kind: gorilla-mux
    std-http-server: ➡️ use generate.handler.kind: std-http
    strict-server: ❌ not supported
    client: ✅
      🆕🐣new properties:
        name: string
        timeout: time.duration
    models: ❌ always generated
    embedded-spec: ❌
    server-urls: ❌
  🆕🐣new properties:
    omit-description: bool
    default-int-type: "int64"
    handler:
      kind: string (chi, echo, gin, fiber, std-http, beego, go-zero, kratos, gorilla-mux, goframe, hertz, iris, fasthttp)
      name: string
      middleware: {}
      server:
        directory: string
        port: int
        timeout: int
        handler-package: string
compatibility: ❌
output-options: ➡️renamed to output
    skip-fmt: ➡️ moved to output.skip-fmt
    skip-prune: ➡ moved to config root
    include-tags: ➡ moved to filter include
    exclude-tags: ➡ moved to filter.exclude
    include-operation-ids: ➡ moved to filter.include
    exclude-operation-ids: ➡ moved to filter.exclude
    user-templates: ➡ moved to the config root
    exclude-schemas: ❌ moved to filter.exclude
    response-type-suffix: ❌
    client-type-name: ➡ moved to generate.client.name
    initialism-overrides: ❌
    additional-initialisms: ❌
    nullable-type: ❌
    disable-type-aliases-for-type: ❌
    name-normalizer: ❌
    overlay: ➡️ moved to config root as overlay.sources
    yaml-tags: ❌
    client-response-bytes-function: ❌
    prefer-skip-optional-pointer: ❌
    prefer-skip-optional-pointer-with-omitzero: ❌
    prefer-skip-optional-pointer-on-container-types: ❌

  🆕🐣new properties:
    use-single-file: bool
    directory: string
    filename: string
import-mapping: ❌
additional-imports: ✅
```
