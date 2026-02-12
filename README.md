<!-- --8<-- [start:docs] -->
# `oapi-codegen`

> **Battle-tested**: This generator is continuously tested against 2,000+ real-world OpenAPI specs, successfully generating and compiling over 20 million lines of Go code. Handles complex specs with circular references, deep nesting, and union types.

Using `oapi-codegen` allows you to reduce the boilerplate required to create or integrate with
services based on [OpenAPI 3.x](https://github.com/OAI/OpenAPI-Specification/blob/main/versions/3.0.0.md), and instead focus on writing your business logic, and working
on the real value-add for your organization.

## Features

### Code Generation
- **Idiomatic Go code** - Clean, readable generated code that follows Go conventions
- **OpenAPI 3.x support** - Comprehensive support for OpenAPI 3.0 and 3.1 specifications
- **Flexible output** - Single or multiple file output with configurable structure
- **Smart pruning** - Automatically removes unused types by default (configurable)

### Type System
- **Union types** - Full support for `oneOf`, `anyOf`, and `allOf` with intelligent type merging and `runtime.Either[A, B]` for two-element unions
- **Additional properties** - Handle dynamic fields with `map[string]interface{}` or custom types
- **Validation** - Built-in validation using [go-playground/validator](https://github.com/go-playground/validator) with `Validate()` methods on generated types
- **Custom extensions** - for fine-grained control over code generation

### Client Generation
- **HTTP client generation** - Generate type-safe HTTP clients with customizable timeout and request editors
- **Custom client types** - Wrap generated clients with your own types for additional functionality
- **Error mapping** - Map response types to implement the `error` interface automatically

### Server Generation
- **Complete server scaffolding** - Generate service interfaces, HTTP adapters, routers, and server main.go
- **13 framework support** - Chi, Echo, Gin, Fiber, std-http, Beego, go-zero, Kratos, GoFrame, Hertz, gorilla-mux, fasthttp, Iris
- **Clean architecture** - Service interface pattern separates business logic from HTTP handling
- **Request/response validation** - Optional validation in generated handlers

### MCP Server Generation
- **[MCP (Model Context Protocol)](https://modelcontextprotocol.io/)** - Generate MCP servers for AI assistant integration
- **Tool generation** - Each API operation becomes an MCP tool that AI assistants can call
- **x-mcp extension** - Control tool names, descriptions, and skip operations
- **Works with Claude Desktop, Cursor, and other MCP clients**

### Configuration & Filtering
- **YAML-based configuration** with JSON schema validation
- **Flexible filtering** - Include/exclude by paths, tags, operation IDs, schema properties, or extensions
- **Transitive pruning** - Automatically remove schemas that are only referenced by filtered-out properties
- **[OpenAPI Overlays](https://doordash-oss.github.io/oapi-codegen-dd/overlays/)** - Modify specs without editing originals (add extensions, remove paths)

### Programmatic Access
- **Runtime package** - Public API for working with generated types
- **TypeTracker** - Internal API for managing type definitions programmatically (for advanced use cases)

## Quick Start
```bash
# Install
go install github.com/doordash-oss/oapi-codegen-dd/v3/cmd/oapi-codegen@latest

# Generate code from the Petstore example
oapi-codegen https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.yaml > petstore.go
```

## Examples
The [examples directory](https://github.com/doordash-oss/oapi-codegen-dd/tree/main/examples) contains useful examples of how to use `oapi-codegen`.

## Why v3?

This project is a fork of [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) v2, fully reworked to address limitations in the original.

### Compatibility

Tested against [2,137 real-world OpenAPI 3.0 specs](https://github.com/cubahno/specs):

| Version | Passed | Failed | Pass Rate |
|---------|--------|--------|-----------|
| **v3** | 2,137 | 0 | **100%** |
| v2 | 1,159 | 978 | 54.2% |

### Key Improvements

| Category | v2 | v3 |
|----------|----|----|
| **OpenAPI Support** | 3.0 only | 3.0, 3.1, 3.2 |
| **Parser** | `kin-openapi` | `libopenapi` |
| **Circular References** | Limited | Full support |
| **Union Types (oneOf/anyOf)** | Basic | Full with optimizations |
| **Name Conflicts** | Manual fix required | Automatic resolution |
| **Validation** | None | `Validate()` methods |
| **Server Scaffold** | Interface only, manual boilerplate | Full typed solution (service, middleware, main.go) |
| **Filtering** | Tags, operation IDs | + Paths, extensions, schema properties |
| **Overlays** | Single | Multiple, applied in order |
| **Output** | Single file | Single or multiple files |
| **Templates** | Monolithic | Composable with `{{define}}` blocks |
| **Programmatic API** | Limited | Full (`ParseContext`, `TypeTracker`) |
| **Server Frameworks** | 7 (Chi, Echo, Fiber, Gin, Gorilla, Iris, std-http) | 13 (+ Beego, go-zero, Kratos, GoFrame, Hertz, fasthttp) |
| **MCP Server** | None | Full (AI assistant integration) |

### Migration

If you're migrating from v2, please refer to the [migration guide](https://doordash-oss.github.io/oapi-codegen-dd/migrate-from-v2/).
<!-- --8<-- [end:docs] -->

## License
This project is licensed under the Apache License 2.0.  
See [LICENSE.txt](LICENSE.txt) for details.

## Notices
See [NOTICE.txt](NOTICE.txt) for third-party components and attributions.

## Contributor License Agreement (CLA)
Contributions to this project require agreeing to the DoorDash Contributor License Agreement.  
See [CONTRIBUTOR_LICENSE_AGREEMENT.txt](CONTRIBUTOR_LICENSE_AGREEMENT.txt).
