# `x-enum-names`

Override generated variable names for enum constants.

## Overview

When consuming an enum value from an external system, the name may not produce a nice variable name. Using the `x-enum-names` extension allows overriding the name of the generated variable names.

## Example

```yaml
--8<-- "extensions/xenumnames/api.yaml"
```

## Generated Code

From here, we now get two different forms of the same enum definition.

```go
--8<-- "extensions/xenumnames/gen.go:12:17"
```

```go
--8<-- "extensions/xenumnames/gen.go:29:34"
```

## Use Cases

This extension is particularly useful when:

- External API uses abbreviated or cryptic enum values
- You want more descriptive constant names in your Go code
- Enum values contain special characters or don't follow Go naming conventions
- You need to maintain backwards compatibility while improving code readability

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xenumnames/){:target="_blank"}.

## Related Extensions

- [`x-go-name`](x-go-name.md) - Override the generated name of a field or type
- [`x-go-type-name`](x-go-type-name.md) - Override the generated name of a type

