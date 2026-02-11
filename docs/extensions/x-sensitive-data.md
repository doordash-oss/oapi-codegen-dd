# `x-sensitive-data`

Automatically mask sensitive data in structured logs.

## Overview

The `x-sensitive-data` extension allows you to mark fields as containing sensitive information that should be automatically masked when logging. This is useful for preventing accidental exposure of sensitive data like passwords, API keys, credit card numbers, etc. in logs.

The extension generates two methods:

- **`Masked()`** - Returns a copy of the struct with sensitive fields masked
- **`LogValue()`** - Implements Go's [`slog.LogValuer`](https://pkg.go.dev/log/slog#LogValuer) interface (calls `Masked()` internally)

This means:

- **JSON serialization stays raw** - `json.Marshal(user)` returns the actual values (needed for API calls)
- **Masked JSON** - `json.Marshal(user.Masked())` returns masked values
- **Structured logging is masked** - `slog.Info("user", "user", user)` automatically masks sensitive fields

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

This generates a struct with `Masked()` and `LogValue()` methods:

```go
--8<-- "extensions/xsensitivedata/basic/gen.go:14:77"
```

## Behavior

When logging with slog:

```go
user := User{
    ID:         1,
    Username:   "johndoe",
    Email:      Ptr("user@example.com"),
    Ssn:        Ptr("123-45-6789"),
    CreditCard: Ptr("1234-5678-9012-3456"),
    APIKey:     Ptr("my-secret-key"),
}

// Masked in logs
slog.Info("user created", "user", user)
// Output: user={id=1 username=johndoe email=******** ssn=***-**-**** creditCard=********3456 apiKey=325ededd...}

// Raw in JSON (for API calls)
json.Marshal(user)
// Output: {"id":1,"username":"johndoe","email":"user@example.com","ssn":"123-45-6789",...}
```

## Masked JSON Output

If you need masked JSON (e.g., for API responses that should hide sensitive data), use the `Masked()` method:

```go
// Get masked JSON
maskedJSON, _ := json.Marshal(user.Masked())
// Output: {"id":1,"username":"johndoe","email":"********","ssn":"***-**-****",...}
```

## Partial Masking Options

- `keepPrefix`: Number of characters to keep at the start
- `keepSuffix`: Number of characters to keep at the end

## Full Example

You can see this in more detail in [the example code](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples/extensions/xsensitivedata/){:target="_blank"}.

## Related Extensions

- [`x-oapi-codegen-extra-tags`](x-oapi-codegen-extra-tags.md) - Generate arbitrary struct tags
- [`x-go-json-ignore`](x-go-json-ignore.md) - Ignore fields when (un)marshaling JSON

