# `x-oapi-codegen-only-honour-go-name`

Prevent automatic capitalization of field names, allowing creation of unexported (lowercase) fields.

## Overview

By default, `oapi-codegen` automatically capitalizes field names to make them exported (public) in Go. When you use `x-go-name` to specify a custom field name, it still gets capitalized via the `schemaNameToTypeName()` transformation.

The `x-oapi-codegen-only-honour-go-name` extension bypasses this automatic capitalization, using the exact name specified in `x-go-name` without any transformations. This allows you to create unexported (lowercase) fields in generated structs.

!!! warning
    This is an advanced extension. Unexported fields cannot be accessed from other packages and will not be marshaled/unmarshaled by default JSON encoders.

## Example

```yaml
--8<-- "extensions/xoapicodegenonlyhonourgoname/api.yaml"
```

## Generated Code

From here, we get a struct with an unexported field:

```go
--8<-- "extensions/xoapicodegenonlyhonourgoname/gen.go:10:14"
```

Notice that `accountIdentifier` starts with a lowercase letter, making it unexported (private to the package).

## How It Works

Without `x-oapi-codegen-only-honour-go-name: true`:
- `x-go-name: accountIdentifier` → gets capitalized → `AccountIdentifier` (exported)

With `x-oapi-codegen-only-honour-go-name: true`:
- `x-go-name: accountIdentifier` → used as-is → `accountIdentifier` (unexported)

## Use Cases

This extension is useful when you need:

- **Internal-only fields**: Fields that should only be accessible within the same package
- **Implementation details**: Fields that are part of internal logic but not part of the public API
- **Custom marshaling**: Fields that require custom JSON marshaling logic (hence `json:"-"`)
- **Package encapsulation**: Enforcing strict boundaries between packages

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xoapicodegenonlyhonourgoname/){:target="_blank"}.

## Related Extensions

- [`x-go-name`](x-go-name.md) - Override the generated name of a field or type (this extension modifies its behavior)
- [`x-go-json-ignore`](x-go-json-ignore.md) - Ignore fields when (un)marshaling JSON (commonly used together)
- [`x-oapi-codegen-extra-tags`](x-oapi-codegen-extra-tags.md) - Generate arbitrary struct tags (often used with `json:"-"`)

