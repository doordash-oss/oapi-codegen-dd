# `x-oapi-codegen-extra-tags`

Generate arbitrary struct tags to fields.

## Overview

If you're making use of a field's struct tags to i.e. apply custom validation, 
decide whether something should be logged, etc, you can use `x-oapi-codegen-extra-tags` to set 
additional tags for your generated types.

## Example

```yaml
--8<-- "extensions/xoapicodegenextratags/api.yaml"
```

## Generated Code

From here, we now get two different models:

```go
--8<-- "extensions/xoapicodegenextratags/gen.go:10:13"
```

```go
--8<-- "extensions/xoapicodegenextratags/gen.go:19:22"
```

## Use Cases

Common use cases for extra tags include:

- **ORM mapping**: Add GORM, SQLBoiler, or other ORM tags
- **Logging control**: Mark fields as safe or unsafe to log
- **Serialization**: Add tags for other serialization formats (XML, YAML, etc.)
- **Custom metadata**: Any custom tags your application needs

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xoapicodegenextratags/){:target="_blank"}.

## Related Extensions

- [`x-go-json-ignore`](x-go-json-ignore.md) - Ignore fields when (un)marshaling JSON
- [`x-sensitive-data`](x-sensitive-data.md) - Automatically mask sensitive data

