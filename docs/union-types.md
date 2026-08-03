# Union Types

Union types in OpenAPI allow schemas to accept multiple different types or structures. The `oapi-codegen` generator handles `allOf`, `anyOf`, `oneOf`, and `if`/`then`/`else` with intelligent type generation based on the number and nature of the variants.

## Overview

- **`allOf`**: Merges all schemas into a single struct with all fields combined
- **`anyOf`**: Can match any of the specified schemas
- **`oneOf`**: Must match exactly one of the specified schemas
- **`if`/`then`/`else`**: Conditional schemas (OpenAPI 3.1) - uses `runtime.Conditional[T, E]`

The generator applies smart optimizations based on the union structure:

1. **Single element or nullable unions** → Type bubbles up directly (no wrapper)
2. **Two-element unions** → Uses `runtime.Either[A, B]` pattern
3. **Three+ element unions** → Uses `json.RawMessage` with accessor methods

---

## `allOf` Merges All Schemas

When using `allOf`, all schemas are merged into a single struct containing all fields from all referenced schemas.

### OpenAPI Spec

```yaml
--8<-- "union/allof/api.yaml:19:28"
```

### Generated Go Code

```go
--8<-- "union/allof/gen.go:32:36"
```

All fields from `Identity` (Issuer), `Verification` (Verifier), and the inline schema (Same) are merged into a single struct.

[View the complete example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/union/allof/){:target="_blank"}

---

## Bubble-Up for Single Element or Nullable Unions

When a union contains only one non-null element, the type simplifies directly to that element without creating a wrapper type.

### Single Element Union

**OpenAPI Spec:**

```yaml
--8<-- "union/anyof-single/api.yaml:31:36"
```

**Generated Go Code:**

```go
--8<-- "union/anyof-single/gen.go:14:18"
```

No `Order_Client_AnyOf` wrapper is created—the type bubbles up to `*Identity` directly.

### Nullable Union (anyOf/oneOf with null)

**OpenAPI Spec:**

```yaml
--8<-- "union/nullable-union/api.yaml:36:44"
```

**Generated Go Code:**

```go
--8<-- "union/nullable-union/gen.go:16:22"
```

Nullable unions become simple pointer types—no union wrapper types are created.

[View single element example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/union/anyof-single/){:target="_blank"}

[View nullable union example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/union/nullable-union/){:target="_blank"}

---

## Two-Element Unions Use `runtime.Either`

When a union has exactly two non-null elements, the generator creates a type using `runtime.Either[A, B]`.

### OpenAPI Spec

```yaml
--8<-- "union/anyof/api.yaml:19:28"
```

### Generated Go Code

```go
--8<-- "union/anyof/gen.go:101:117"
```

The `runtime.Either` type provides `IsA()` and `IsB()` methods to check which variant is present.

[View the complete example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/union/anyof/){:target="_blank"}

---

## Three+ Element Unions

For unions with three or more elements, the generator uses `json.RawMessage` storage with accessor methods for each type.

### OpenAPI Spec (OpenAPI 3.1)

```yaml
--8<-- "union/types/api.yaml:19:30"
```

### Generated Go Code

```go
--8<-- "union/types/gen.go:87:89"
```

Accessor methods for each type:

```go
--8<-- "union/types/gen.go:98:131"
```

Each variant gets three methods:
- `As*()` - Retrieve the value as the specific type
- `AsValidated*()` - Retrieve and validate the value
- `From*()` - Set the value as the specific type

[View the complete example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/union/types/){:target="_blank"}

---

## Complex Unions (allOf + anyOf + oneOf)

You can combine `allOf` with `anyOf` and `oneOf` in the same schema. The generator merges the `allOf` fields into the parent struct and injects the union type fields with `json:"-"` tags.

### Union Field Injection

When a schema combines `allOf` with `anyOf` or `oneOf`, the union fields are injected into the parent struct with `json:"-"` tags to prevent direct JSON marshaling. The parent struct handles marshaling through custom `MarshalJSON` and `UnmarshalJSON` methods.

### OpenAPI Spec

```yaml
--8<-- "union/allof-anyof-oneof/api.yaml:26:46"
```

### Generated Go Code

```go
--8<-- "union/allof-anyof-oneof/gen.go:61:66"
```

Notice the `json:"-"` tags on the union fields. These fields are not directly marshaled to JSON.

### Custom Marshaling

The generator creates custom `MarshalJSON` and `UnmarshalJSON` methods that merge all parts together:

```go
--8<-- "union/allof-anyof-oneof/gen.go:96:123"
```

The `runtime.JSONMerge` function combines all the JSON parts into a single object, ensuring that the base fields and union fields are properly merged.

[View complex union example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/union/allof-anyof-oneof/){:target="_blank"}

---

## Conditional Schemas (`if`/`then`/`else`)

OpenAPI 3.1 supports JSON Schema conditional keywords. The generator
treats `then` and `else` as structural branches. The `if` schema is
ignored - it's a validation predicate (e.g. "if kind equals typeA")
with no structural properties. `if` is also a reserved keyword in Go.

### Both Branches - `runtime.Conditional`

When both `then` and `else` are present, the generator creates a
`runtime.Conditional[T, E]` wrapper with named variant types. Unlike
`runtime.Either` (`.A`/`.B`), `Conditional` uses `.Then`/`.Else`
fields and `.IsThen()`/`.IsElse()` methods.

**OpenAPI Spec:**

```yaml
--8<-- "ifthenelse/basic/api.yaml:20:42"
```

**Generated Go Code:**

```go
--8<-- "ifthenelse/basic/gen.go:16:19"
```

The variant types use `_Then` and `_Else` suffixes:

```go
--8<-- "ifthenelse/basic/gen.go:86:98"
```

Usage:

```go
if resource.Resource_IfThenElse.IsThen() {
    fmt.Println(resource.Resource_IfThenElse.Then.FieldA)
}
if resource.Resource_IfThenElse.IsElse() {
    fmt.Println(resource.Resource_IfThenElse.Else.FieldB)
}
```

[View basic example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/ifthenelse/basic/){:target="_blank"}

### Single Branch - Flat Merge

When only `then` or only `else` is present, the branch properties
are flat-merged into the parent struct. No wrapper type is created.

**OpenAPI Spec:**

```yaml
--8<-- "ifthenelse/then-only/api.yaml:20:35"
```

**Generated Go Code:**

```go
--8<-- "ifthenelse/then-only/gen.go:12:16"
```

The `timeout` and `retries` fields from the `then` branch appear
directly on the `Config` struct.

[View then-only example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/ifthenelse/then-only/){:target="_blank"}

### With `$ref` Branches

When `then`/`else` reference component schemas, the generated
`Conditional` uses the referenced type names directly:

```go
--8<-- "ifthenelse/with-refs/gen.go:98:100"
```

[View with-refs example](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/ifthenelse/with-refs/){:target="_blank"}

