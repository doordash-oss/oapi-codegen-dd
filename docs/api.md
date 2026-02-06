# Programmatic API

This guide shows how to use oapi-codegen as a library in your Go code, rather than as a command-line tool.

## Overview

The oapi-codegen package provides a programmatic API for generating Go code from OpenAPI specifications. This is useful when you need to:

- Integrate code generation into your build pipeline
- Generate code dynamically at runtime
- Access type definitions and operations programmatically
- Build custom tooling on top of oapi-codegen

## Quick Start

### Simple Code Generation

The simplest way to generate code is using the `codegen.Generate()` function:

```go title="examples/api/simple-generate/main.go"
--8<-- "api/simple-generate/main.go"
```

### Advanced: Two-Step Generation

For more control, use the two-step approach with `CreateParseContext()` and `NewParser()`:

```go title="examples/api/advanced-two-step/main.go"
--8<-- "api/advanced-two-step/main.go"
```

## Configuration

### Loading Configuration from YAML

```go title="examples/api/load-config/main.go"
--8<-- "api/load-config/main.go"
```

See the [Configuration](configuration.md) page for all available options.

## Accessing Type Definitions

The `ParseContext` provides access to all generated type definitions and operations:

```go title="examples/api/access-types/main.go"
--8<-- "api/access-types/main.go"
```

## TypeDefinition Structure

Each `TypeDefinition` describes a Go type in the generated code:

```go
type TypeDefinition struct {
    Name             string        // Go type name (e.g., "User")
    JsonName         string        // JSON field name (e.g., "user")
    Schema           GoSchema      // Schema object with type information
    SpecLocation     SpecLocation  // Where in spec this was defined
    NeedsMarshaler   bool          // Whether custom marshaler needed
    HasSensitiveData bool          // Whether has sensitive properties
}
```

### TypeDefinition Methods

```go
// Check if type is an alias
if td.IsAlias() {
    fmt.Printf("%s is an alias\n", td.Name)
}

// Check if type is optional
if td.IsOptional() {
    fmt.Printf("%s is optional\n", td.Name)
}

// Generate error response code (for response types)
if td.SpecLocation == codegen.SpecLocationResponse {
    errorCode := td.GetErrorResponse()
    fmt.Printf("Error response code: %s\n", errorCode)
}
```

### SpecLocation Constants

Types are organized by where they appear in the OpenAPI spec:

```go
const (
    SpecLocationPath     SpecLocation = "path"      // Path parameters
    SpecLocationQuery    SpecLocation = "query"     // Query parameters
    SpecLocationHeader   SpecLocation = "header"    // Header parameters
    SpecLocationBody     SpecLocation = "body"      // Request body
    SpecLocationResponse SpecLocation = "response"  // Response types
    SpecLocationSchema   SpecLocation = "schema"    // Component schemas
    SpecLocationUnion    SpecLocation = "union"     // Union types
)
```

## Template Functions

The `codegen.TemplateFunctions` provides built-in template functions that can be extended with custom functions:

```go
import (
    "text/template"
    "github.com/doordash-oss/oapi-codegen-dd/v3/pkg/codegen"
)

// Get built-in template functions
funcMap := codegen.TemplateFunctions

// Add custom functions
funcMap["myCustomFunc"] = func(s string) string {
    return "custom: " + s
}

// Use in template
tmpl := template.New("custom").Funcs(funcMap)
tmpl.Parse("{{ myCustomFunc .Name }}")
```

## Complete Example

Here's a complete example showing how to use the API in a real project:

```go title="examples/api/complete-example/main.go"
--8<-- "api/complete-example/main.go"
```

## See Also

- [Configuration](configuration.md) - Complete configuration reference
- [Union Types](union-types.md) - Working with union types
- [Extensions](extensions.md) - OpenAPI extensions reference

