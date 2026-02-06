# `x-go-name`

Override the generated name of a field or a type.

## Overview

By default, `oapi-codegen` will attempt to generate the name of fields and types in as best a way it can.

However, sometimes, the name doesn't quite fit what your codebase standards are, or the intent of the field, so you can override it with `x-go-name`.

## Example

```yaml
--8<-- "extensions/xgoname/api.yaml"
```

## Generated Code

From here, we now get two different models:

```go
--8<-- "extensions/xgoname/gen.go:123:126"
```

```go
--8<-- "extensions/xgoname/gen.go:132:135"
```

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xgoname/){:target="_blank"}.

## Related Extensions

- [`x-go-type-name`](x-go-type-name.md) - Override the generated name of a type only (not fields)
- [`x-go-type`](x-go-type.md) - Override the generated type definition

