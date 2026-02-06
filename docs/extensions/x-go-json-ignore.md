# `x-go-json-ignore`

When (un)marshaling JSON, ignore field(s).

## Overview

By default, `oapi-codegen` will generate `json:"..."` struct tags for all fields in a struct, so JSON (un)marshaling works.

However, sometimes, you want to omit fields, which can be done with the `x-go-json-ignore` extension.

## Example

```yaml
--8<-- "extensions/xgojsonignore/api.yaml"
```

## Generated Code

From here, we now get two different models:

```go
--8<-- "extensions/xgojsonignore/gen.go:10:13"

--8<-- "extensions/xgojsonignore/gen.go:33:36"

--8<-- "extensions/xgojsonignore/gen.go:38:41"

--8<-- "extensions/xgojsonignore/gen.go:61:64"
```

Notice that the `ComplexField` is still generated in full, but the type will then be ignored with JSON marshalling (`json:"-"`).

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xgojsonignore/){:target="_blank"}.

## Related Extensions

- [`x-omitempty`](x-omitempty.md) - Force the presence of the JSON tag `omitempty` on a field
- [`x-oapi-codegen-extra-tags`](x-oapi-codegen-extra-tags.md) - Generate arbitrary struct tags

