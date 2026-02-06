# oapi-codegen Documentation

Welcome to the oapi-codegen documentation! This tool helps you generate Go code from OpenAPI 3.x specifications, reducing boilerplate and letting you focus on your business logic.


## Getting Started

```bash
# Install
go install github.com/doordash-oss/oapi-codegen-dd/v3/cmd/oapi-codegen@latest

# Generate code from the Petstore example
oapi-codegen https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.yaml > petstore.go
```

## Best Practices

!!! tip "Define schemas in `#/components/schemas`"

    Try to define as much as possible within the `#/components/schemas` object, as `oapi-codegen`
    will generate all the types here with clear, predictable names.

    While we can generate types from inline definitions (e.g., in a path's response type),
    the generated type names may be less intuitive since they're derived from the path and operation.

