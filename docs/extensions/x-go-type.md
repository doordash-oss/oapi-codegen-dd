# `x-go-type` and `x-go-type-import`

Override the generated type definition (and optionally, add an import from another package).

## Overview

Using the `x-go-type` (and optionally, `x-go-type-import` when you need to import another package) allows overriding the type that `oapi-codegen` determined the generated type should be.

## Example

```yaml
--8<-- "extensions/xgotype/api.yaml"
```

## Generated Code

From here, we now get two different models:

```go
--8<-- "extensions/xgotype/gen.go:11:14"
```

```go
--8<-- "extensions/xgotype/gen.go:20:23"
```

## Imports follow the types

A package is imported only when a generated type refers to it. `format: uuid` generates `uuid.UUID` from `github.com/google/uuid`, so overriding every uuid schema, for example with an overlay, drops that import and lets you point at another package such as the standard library one:

```yaml
x-go-type: uuid.UUID
x-go-type-import:
  path: uuid
```

Two things follow from this:

- An `x-go-type` that refers to a package needs its `x-go-type-import`. `x-go-type: uuid.UUID` alone only gets `github.com/google/uuid` while some other schema still generates `uuid.UUID`. Without that, goimports adds the standard library `uuid` instead. That builds on Go 1.27, with the standard library type rather than `google/uuid`, and fails on older Go with `package uuid is not in std`. The generator logs a warning that lists the schemas:

  ```
  WARN x-go-type: package uuid is only imported when a type uses github.com/google/uuid, so a schema that names it without an x-go-type-import may not build schemas=GetThings.Response.id hint="set x-go-type-import to the package you want, e.g. {path: github.com/google/uuid}"
  ```
- When overridden and plain uuid schemas share a file, both packages are called `uuid`. Give the override an alias, `x-go-type: stduuid.UUID` with `name: stduuid` next to `path: uuid` under `x-go-type-import`.

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xgotype/){:target="_blank"}.

## Related Extensions

- [`x-go-name`](x-go-name.md) - Override the generated name of a field or type
- [`x-go-type-name`](x-go-type-name.md) - Override the generated name of a type only

