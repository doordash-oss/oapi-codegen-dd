# OpenAPI Extensions

As well as the core OpenAPI support, we also support the following OpenAPI extensions, as denoted by the [OpenAPI Specification Extensions](https://spec.openapis.org/oas/v3.0.3#specification-extensions).

## Available Extensions

| Extension | Description | Example |
|-----------|-------------|---------|
| [`x-go-type`](extensions/x-go-type.md) / [`x-go-type-import`](extensions/x-go-type.md) | Override the generated type definition (and optionally, add an import from another package) | [View Example](extensions/x-go-type.md) |
| [`x-go-type-skip-optional-pointer`](extensions/x-go-type-skip-optional-pointer.md) | Do not add a pointer type for optional fields in structs | [View Example](extensions/x-go-type-skip-optional-pointer.md) |
| [`x-go-name`](extensions/x-go-name.md) | Override the generated name of a field or a type | [View Example](extensions/x-go-name.md) |
| [`x-go-type-name`](extensions/x-go-type-name.md) | Override the generated name of a type | [View Example](extensions/x-go-type-name.md) |
| [`x-oapi-codegen-only-honour-go-name`](extensions/x-oapi-codegen-only-honour-go-name.md) | Prevent automatic capitalization of field names (for unexported fields) | [View Example](extensions/x-oapi-codegen-only-honour-go-name.md) |
| [`x-omitempty`](extensions/x-omitempty.md) | Force the presence of the JSON tag `omitempty` on a field | [View Example](extensions/x-omitempty.md) |
| [`x-go-json-ignore`](extensions/x-go-json-ignore.md) | When (un)marshaling JSON, ignore field(s) | [View Example](extensions/x-go-json-ignore.md) |
| [`x-oapi-codegen-extra-tags`](extensions/x-oapi-codegen-extra-tags.md) | Generate arbitrary struct tags to fields | [View Example](extensions/x-oapi-codegen-extra-tags.md) |
| [`x-sensitive-data`](extensions/x-sensitive-data.md) | Automatically mask sensitive data in JSON output | [View Example](extensions/x-sensitive-data.md) |
| [`x-enum-names`](extensions/x-enum-names.md) | Override generated variable names for enum constants | [View Example](extensions/x-enum-names.md) |
| [`x-deprecated-reason`](extensions/x-deprecated-reason.md) | Add a GoDoc deprecation warning to a type | [View Example](extensions/x-deprecated-reason.md) |

## Quick Examples

### Type Override with Import

```yaml
properties:
  id:
    type: string
    x-go-type: uuid.UUID
    x-go-type-import:
      path: github.com/google/uuid
```

### Custom Struct Tags

```yaml
properties:
  email:
    type: string
    x-oapi-codegen-extra-tags:
      validate: "required,email"
      safe-to-log: "false"
```

### Sensitive Data Masking

```yaml
properties:
  password:
    type: string
    x-sensitive-data:
      mask: full
```

For detailed documentation on each extension, click on the extension name in the table above.

