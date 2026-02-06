# `x-deprecated-reason`

Add a GoDoc deprecation warning to a type.

## Overview

When an OpenAPI type is deprecated, a deprecation warning can be added in the GoDoc using `x-deprecated-reason`.

!!! note
    The `x-deprecated-reason` extension only takes effect when `deprecated: true` is also set on the field or type.

## Example

```yaml
--8<-- "extensions/xdeprecatedreason/api.yaml"
```

## Generated Code

From here, we now get two different models:

```go
--8<-- "extensions/xdeprecatedreason/gen.go:10:13"
```

```go
--8<-- "extensions/xdeprecatedreason/gen.go:19:23"
```

Notice that because we've not set `deprecated: true` to the `id` field, it doesn't generate a deprecation warning.

## Use Cases

This extension is useful for:

- Providing clear migration guidance when deprecating fields
- Documenting why a field is deprecated
- Suggesting alternatives to deprecated fields
- Maintaining API documentation in code

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xdeprecatedreason/){:target="_blank"}.

## Related Extensions

- [`x-go-name`](x-go-name.md) - Override the generated name of a field or type

