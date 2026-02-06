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

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xgotype/){:target="_blank"}.

## Related Extensions

- [`x-go-name`](x-go-name.md) - Override the generated name of a field or type
- [`x-go-type-name`](x-go-type-name.md) - Override the generated name of a type only

