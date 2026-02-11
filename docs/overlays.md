# OpenAPI Overlays

OpenAPI Overlays allow you to modify an OpenAPI specification without editing the original file. This is useful for:

- Adding Go-specific extensions (`x-go-name`, `x-go-type`) to third-party specs
- Removing internal endpoints before generating client code
- Customizing descriptions or adding documentation
- Applying organization-wide conventions to multiple specs

oapi-codegen uses [libopenapi's overlay support](https://pb33f.io/libopenapi/overlays/) which implements the [OpenAPI Overlay Specification 1.0.0](https://github.com/OAI/Overlay-Specification).

## Basic Usage

Configure overlays in your `cfg.yml`:

```yaml
package: api
overlay:
  sources:
    - ./overlays/add-go-names.yml
    - https://example.com/shared-overlay.yml
generate:
  client: true
```

Overlays are applied in order before filtering and pruning.

## Overlay File Format

An overlay file has three main sections:

```yaml
overlay: 1.0.0
info:
  title: My Overlay
  version: 1.0.0
actions:
  - target: <JSONPath expression>
    update: <object to merge>
  - target: <JSONPath expression>
    remove: true
```

### Actions

| Action | Description |
|--------|-------------|
| `update` | Merge the provided object into the target |
| `remove` | Remove the target from the document |

## Examples

### Add x-go-name Extensions

Add Go-friendly names to schemas:

```yaml
overlay: 1.0.0
info:
  title: Add Go Extensions
  version: 1.0.0
actions:
  - target: $.components.schemas.user
    update:
      x-go-name: User
  - target: $.components.schemas.user.properties.user_id
    update:
      x-go-name: ID
```

### Remove Internal Endpoints

Remove paths that shouldn't be in the generated client:

```yaml
overlay: 1.0.0
info:
  title: Remove Internal Endpoints
  version: 1.0.0
actions:
  - target: $.paths['/internal/health']
    remove: true
  - target: $.paths['/internal/metrics']
    remove: true
```

!!! note "JSONPath Syntax"
    Paths containing special characters like `/` must use bracket notation: `$.paths['/users/{id}']` not `$.paths./users/{id}`.

### Add Custom Extensions

Add organization-specific extensions:

```yaml
overlay: 1.0.0
info:
  title: Add Custom Extensions
  version: 1.0.0
actions:
  - target: $.components.schemas.Order
    update:
      x-oapi-codegen-extra-tags:
        db: orders
        json: order
```

### Modify Descriptions

Update or add descriptions:

```yaml
overlay: 1.0.0
info:
  title: Improve Documentation
  version: 1.0.0
actions:
  - target: $.info
    update:
      description: "Updated API description for internal use"
  - target: $.paths['/users'].get
    update:
      description: "List all users. Requires admin permissions."
```

## Multiple Overlays

You can apply multiple overlays in sequence. They are applied in the order specified:

```yaml
overlay:
  sources:
    - ./overlays/base-extensions.yml      # Applied first
    - ./overlays/remove-internal.yml      # Applied second
    - https://cdn.example.com/shared.yml  # Applied third
```

This is useful for:

- Separating concerns (extensions vs. removals)
- Sharing common overlays across projects
- Environment-specific modifications

## URL Sources

Overlay sources can be URLs, allowing you to share overlays across teams:

```yaml
overlay:
  sources:
    - https://raw.githubusercontent.com/myorg/api-standards/main/go-extensions.yml
```

## Processing Order

Overlays are applied early in the processing pipeline:

1. **Load spec** - Parse the OpenAPI document
2. **Apply overlays** - Modify the document with overlay actions
3. **Filter** - Apply include/exclude filters
4. **Prune** - Remove unused schemas
5. **Generate** - Generate Go code

This means overlays can add extensions that affect code generation, or remove paths before filtering is applied.

## JSONPath Reference

Overlays use JSONPath expressions to target elements. Common patterns:

| Target | JSONPath |
|--------|----------|
| Root info | `$.info` |
| All paths | `$.paths` |
| Specific path | `$.paths['/users']` |
| Path with variable | `$.paths['/users/{id}']` |
| Operation | `$.paths['/users'].get` |
| Schema | `$.components.schemas.User` |
| Schema property | `$.components.schemas.User.properties.email` |
| All schemas | `$.components.schemas.*` |

See the [JSONPath specification](https://goessner.net/articles/JsonPath/) for full syntax.

## See Also

- [Configuration Reference](configuration.md#overlay) - Full overlay configuration options
- [libopenapi Overlays](https://pb33f.io/libopenapi/overlays/) - Underlying implementation details
- [OpenAPI Overlay Specification](https://github.com/OAI/Overlay-Specification) - Official specification
