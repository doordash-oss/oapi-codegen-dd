# `x-go-type-skip-optional-pointer`

Do not add a pointer type for optional fields in structs.

## Overview

By default, `oapi-codegen` will generate a pointer for optional fields.

Using the `x-go-type-skip-optional-pointer` extension allows omitting that pointer.

## Example

```yaml
--8<-- "extensions/xgotypeskipoptionalpointer/api.yaml"
```

## Generated Code

From here, we now get two different models:

```go
--8<-- "extensions/xgotypeskipoptionalpointer/gen.go:10:13"
```

```go
--8<-- "extensions/xgotypeskipoptionalpointer/gen.go:19:22"
```

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xgotypeskipoptionalpointer/){:target="_blank"}.

## Related Extensions

- [`x-omitempty`](x-omitempty.md) - Force the presence of the JSON tag `omitempty` on a field

