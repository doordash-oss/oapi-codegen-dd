# `x-go-type-name`

Override the generated name of a type.

## Overview

!!! note
    Notice that this is subtly different to the `x-go-name`, which also applies to _fields_ within `struct`s.

By default, `oapi-codegen` will attempt to generate the name of types in as best a way it can.

However, sometimes, the name doesn't quite fit what your codebase standards are, or the intent of the field, so you can override it with `x-go-type-name`.

## Example

```yaml
--8<-- "extensions/xgotypename/api.yaml"
```

## Generated Code

From here, we now get two different models and a type alias:

```go
--8<-- "extensions/xgotypename/gen.go:10:13"
```

```go
--8<-- "extensions/xgotypename/gen.go:19:19"
```

```go
--8<-- "extensions/xgotypename/gen.go:21:24"
```

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xgotypename/){:target="_blank"}.

## Related Extensions

- [`x-go-name`](x-go-name.md) - Override the generated name of a field or type
- [`x-go-type`](x-go-type.md) - Override the generated type definition

