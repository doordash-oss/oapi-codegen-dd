# Additional Properties

OpenAPI's `additionalProperties` keyword allows schemas to accept fields that aren't explicitly defined in the schema. This is useful for creating flexible APIs that can handle dynamic or user-defined fields.

## Overview

When you use `additionalProperties` in your OpenAPI spec, oapi-codegen generates Go code that can handle both:
- **Defined properties**: Fields explicitly listed in the schema's `properties` section
- **Additional properties**: Any extra fields not in the schema definition

## Basic Usage

### Type Aliases for Maps

When a schema consists **only** of `additionalProperties` (no explicit properties), oapi-codegen generates a type alias to a Go map:

```yaml
--8<-- "additional-properties/ex1/api.yaml:21:24"
```

Generates:

```go
--8<-- "additional-properties/ex1/gen.go:20:20"
```

### Explicit additionalProperties: true

Using `additionalProperties: true` creates a map that accepts any value type:

```yaml
--8<-- "additional-properties/ex1/api.yaml:16:19"
```

Generates:

```go
--8<-- "additional-properties/ex1/gen.go:18:18"
```

### Typed Additional Properties

You can specify the type of additional property values:

```yaml
--8<-- "additional-properties/ex1/api.yaml:11:14"
```

Generates:

```go
--8<-- "additional-properties/ex1/gen.go:16:16"
```

## Mixed Properties and Additional Properties

When a schema has **both** explicit properties and `additionalProperties`, oapi-codegen generates a struct with:
- Regular fields for defined properties
- An `AdditionalProperties` field (with `json:"-"` tag)
- `Get()` and `Set()` methods for accessing additional properties
- Custom `MarshalJSON()` and `UnmarshalJSON()` methods

Example schema:

```yaml
--8<-- "additional-properties/ex1/api.yaml:41:47"
```

Generated code:

```go
--8<-- "additional-properties/ex1/gen.go:61:81"
```

The custom JSON marshaling ensures that additional properties are serialized alongside defined properties in the JSON output.

## Advanced Scenarios

### Nested Additional Properties

You can nest `additionalProperties` to create multi-level maps:

```yaml
metadata:
  type: object
  additionalProperties:
    additionalProperties:
      additionalProperties:
        $ref: "#/components/schemas/Metadata"
```

Generates:

```go
Metadata map[string]map[string]map[string]Metadata
```

[View nested example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/additional-properties/nested/){:target="_blank"}

### Self-Referencing Schemas

Schemas can reference themselves via `additionalProperties`:

```yaml
AggregatedResult:
  type: object
  properties:
    totalClicks:
      type: integer
    hourlyBreakDown:
      type: object
      additionalProperties:
        $ref: '#/components/schemas/AggregatedResult'
```

Generates:

```go
type AggregatedResult struct {
    TotalClicks     *int                        `json:"totalClicks,omitempty"`
    HourlyBreakDown map[string]AggregatedResult `json:"hourlyBreakDown,omitempty"`
}
```

[View self-reference example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/additional-properties/self-reference/){:target="_blank"}

## Validation

For map types, oapi-codegen generates `Validate()` methods that validate each value in the map:

```go
--8<-- "additional-properties/ex1/gen.go:22:35"
```

## Complete Examples

For comprehensive examples including:
- Property count constraints (`minProperties`, `maxProperties`)
- String length constraints in map values
- Required fields in additional property values
- `oneOf` in additional properties
- And more...

[View complete examples](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/additional-properties/){:target="_blank"}

