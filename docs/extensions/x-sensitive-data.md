# `x-sensitive-data`

Automatically mask sensitive data in JSON output.

## Overview

The `x-sensitive-data` extension allows you to mark fields as containing sensitive information that should be automatically masked when marshaling to JSON. This is useful for preventing accidental logging or exposure of sensitive data like passwords, API keys, credit card numbers, etc.

## Masking Strategies

The extension supports several masking strategies:

- **`full`**: Replace the entire value with a fixed-length mask (`"********"`) to hide both content and length
- **`regex`**: Mask only parts of the value matching a regex pattern (keeps context visible)
- **`hash`**: Replace the value with a SHA256 hash (one-way, useful for verification)
- **`partial`**: Mask the middle part while keeping prefix/suffix visible (e.g., show last 4 digits of credit card)

## Example

```yaml
--8<-- "extensions/xsensitivedata/basic/api.yaml"
```

## Generated Code

This generates:

```go
--8<-- "extensions/xsensitivedata/basic/gen.go:16:23"
```

## Behavior

When marshaling to JSON:

- `email: "user@example.com"` becomes `email: "********"` (fixed length, hides original length)
- `ssn: "123-45-6789"` becomes `ssn: "***-**-****"` (digits masked, structure visible)
- `creditCard: "1234-5678-9012-3456"` becomes `creditCard: "********3456"` (last 4 visible)
- `apiKey: "my-secret-key"` becomes `apiKey: "325ededd6c3b9988f623c7f964abb9b016b76b0f8b3474df0f7d7c23b941381f"` (SHA256 hash)

## Partial Masking Options

- `keepPrefix`: Number of characters to keep at the start
- `keepSuffix`: Number of characters to keep at the end

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xsensitivedata/){:target="_blank"}.

## Related Extensions

- [`x-oapi-codegen-extra-tags`](x-oapi-codegen-extra-tags.md) - Generate arbitrary struct tags
- [`x-go-json-ignore`](x-go-json-ignore.md) - Ignore fields when (un)marshaling JSON

