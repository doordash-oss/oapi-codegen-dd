# Validation

oapi-codegen generates `Validate()` methods for your types based on OpenAPI schema constraints. 
This uses the [go-playground/validator](https://github.com/go-playground/validator) library under the hood.

## Configuration Options

```yaml
generate:
  validation:
    skip: false      # Set to true to skip Validate() method generation
    simple: false    # Set to true to use simple validate.Struct() for all types
    response: false  # Set to true to generate Validate() for response types
```

### `skip`

When `true`, no `Validate()` methods are generated. Use this if you don't need validation or want to implement your own.

### `simple`

When `true`, all struct types use simple `validate.Struct()` validation instead of custom validation logic. This produces cleaner code but doesn't support advanced features like union type validation.

### `response`

When `true`, generates `Validate()` methods for response types. Useful for contract testing to ensure API responses match the OpenAPI spec.

## Supported OpenAPI Constraints

The following OpenAPI constraints are translated to validation tags:

| OpenAPI Constraint | Validation Tag | Applies To |
|--------------------|----------------|------------|
| `required` | `required` | All types |
| `nullable: true` | `omitempty` | All types |
| `minimum` | `gte=N` | integers, numbers |
| `maximum` | `lte=N` | integers, numbers |
| `exclusiveMinimum` | `gt=N` | integers, numbers |
| `exclusiveMaximum` | `lt=N` | integers, numbers |
| `minLength` | `min=N` | strings, arrays |
| `maxLength` | `max=N` | strings, arrays |
| `minItems` | `min=N` | arrays |
| `maxItems` | `max=N` | arrays |
| `enum` | custom switch | string, integer enums |

## Generated Code Examples

### Simple Struct Validation

For simple structs without unions or nested validators:

```go
--8<-- "validation/nested/gen.go:106:112"
```

### Enum Validation

Enum types get custom switch-based validation:

```go
--8<-- "validation/enums/gen.go:12:28"
```

### Complex Validation with Nested Types

For structs with nested types that implement `Validator`:

```go
--8<-- "validation/enums/gen.go:69:100"
```

## Runtime Helpers

### Validator Interface

All generated types with validation implement this interface:

```go
type Validator interface {
    Validate() error
}
```

### ValidationErrors

A slice type that collects multiple validation errors with field paths:

```go
var errors runtime.ValidationErrors
errors = errors.Append("FieldName", err)
```

### ConvertValidatorError

Converts go-playground/validator errors to `ValidationErrors`:

```go
return runtime.ConvertValidatorError(typesValidator.Struct(s))
```

## Usage

### Validating Request Bodies

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    var body CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    if err := body.Validate(); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Process valid request...
}
```

### Contract Testing (Response Validation)

Enable response validation in config:

```yaml
generate:
  validation:
    response: true
```

Then validate responses in tests:

```go
func TestAPIResponse(t *testing.T) {
    resp, err := client.GetUser(ctx, userID)
    require.NoError(t, err)

    // Validate response matches OpenAPI spec
    if err := resp.Validate(); err != nil {
        t.Errorf("Response validation failed: %v", err)
    }
}
```
