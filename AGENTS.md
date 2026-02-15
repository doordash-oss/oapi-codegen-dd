## Building
- Use Makefile settings or commands for building
- All commands located in `cmd/`

## Testing
- All `pkg/runtime` functions should aim for 100% test coverage
- Run `go test -cover ./pkg/runtime/...` to check coverage

## Configuration
- Consult `examples/` for sample configurations for different use cases
- Running without config uses default settings:
  ```go run ./cmd/oapi-codegen <spec-path>```

### Key config options
- `output.use-single-file: true` - Generate all code in one file (default) vs multiple files
- `output.directory` - Output directory (when `use-single-file: false`, package name is appended as subdirectory)
- `generate.client: true` - Generate HTTP client code
- `generate.models: false` - Skip model generation (when models are in separate package)
- `generate.validation.skip: true` - Skip Validate() method generation
- `generate.validation.response: true` - Generate Validate() for response types (useful for contract testing)
- `generate.always-prefix-enum-values: true` - Prefix enum constants with type name (default)
- `generate.default-int-type: int64` - Use int64 instead of int for integer types
- `generate.handler.output.overwrite: true` - Force regeneration of scaffold files (service.go, middleware.go)
- `skip-prune: true` - Keep unused types (normally pruned)
- `error-mapping` - Map response types to implement error interface (key: type name, value: json path to message)
- `filter.include/exclude` - Filter paths, tags, operation-ids, extensions

### Handler/Server generation config
- `generate.handler.kind` - Router framework: `chi`, `echo`, `fiber`, `gin`, `std-http` (required)
- `generate.handler.name` - Service interface name (default: "Service")
- `generate.handler.models-package-alias` - Prefix for model types when models are in separate package
- `generate.handler.validation.request/response` - Enable request/response validation in handlers
- `generate.handler.output.directory/package` - Output for scaffold files (service.go, middleware.go)
- `generate.handler.middleware: {}` - Enable middleware.go generation
- `generate.handler.server` - Enable server/main.go generation with `directory`, `port`, `timeout`, `handler-package`

## Verifying changes
- After making changes to the code generator, ensure to run `make generate` which regenerates the code for all OpenAPI specs in `examples/`
- Verify that the generated code matches the expected output in `examples/`
- Run `make test` to ensure all unit tests pass
- Consider running integration tests in Connexions to verify end-to-end functionality:
  ```make test-integration```. That could take 5 minutes.

## Investigating integration test failures

### Running specific specs
- Re-run with `SPEC=<path-to-spec.yml> make test-integration` to focus on a specific spec
- Never run all integration tests at once - always use SPEC= to limit scope
- Example: `make test-integration SPEC=3.0/github.com/ghes-3.5.1.1.4.yml`
- **Never run oapi-codegen in the project root directory** - always use `/tmp` or run via `make test-integration`

### Common failure types and resolutions

1. **libopenapi limitation - JSON Pointer refs to paths**
   - Error: `component '#/paths/~1api~1v1~1...' does not exist in the specification`
   - Cause: Spec uses `$ref` pointing to path elements which libopenapi can't resolve
   - Resolution: Skip spec by prefixing filename with `-` (e.g., `mv spec.yml -spec.yml`)

2. **Missing schema/component reference**
   - Error: `component '#/components/schemas/SomeName' does not exist`
   - Cause: Spec references a schema that doesn't exist (broken spec)
   - Resolution: Remove the spec from testdata

3. **x- prefixed schema names**
   - Error: `undefined: XAny` or similar
   - Cause: Spec has schemas named with `x-` prefix (e.g., `x-any`) which libopenapi treats as extensions
   - Resolution: Skip spec with `-` prefix

4. **External file references**
   - Error: `unable to open the rolodex file` or `../some-file.yaml does not exist`
   - Cause: Spec references external files that aren't present
   - Resolution: Remove the spec from testdata

5. **Build failures (undefined types, syntax errors)**
   - Check the generated code at the debug path shown in error output
   - Look for patterns in the generated code that indicate the issue
   - Fix the generator code, then re-run the specific spec

### Workflow
1. Get the failing spec path from test output
2. Run: `go run ./cmd/oapi-codegen <spec-path>` to see generation errors
3. If generation succeeds, check the debug path for the generated code
4. For libopenapi limitations, skip with `-` prefix
5. For broken specs, remove from testdata
6. For generator bugs, fix and re-test

### Creating reproducible test cases
When debugging complex issues, create a minimal reproducible example:
1. Create a new directory in `examples/` (e.g., `examples/issue-123/`)
2. Add a simplified spec that reproduces the issue
3. Add a `cfg.yaml` with the relevant configuration
4. Create a `gen_test.go` to verify the generated code compiles:
   ```go
   package example_test

   import (
       "testing"
       _ "github.com/doordash-oss/oapi-codegen-dd/v3/examples/issue-123"
   )

   func TestCompiles(t *testing.T) {}
   ```
5. Run `go generate ./examples/issue-123/...` to generate code
6. Run `go test ./examples/issue-123/...` to verify it compiles
7. After fixing, ask the user whether to keep the example or remove it

### File organization
- `testdata/specs` - Specs being tested (if missing: run `make fetch-specs` to download)
- Never put generated files in project root - use `/tmp` for testing
- Never build binaries in project root - use `go run ./cmd/oapi-codegen` instead of `go build`

## Code Generation Architecture

### Two-pass schema processing
The generator uses a two-pass approach for component schemas:
1. **Pass 1 (preRegisterSchemaNames)**: Registers all schema names and refs in TypeTracker BEFORE processing. This ensures cross-references can find the correct (potentially renamed) type names.
2. **Pass 2 (generateSchemaDefinitions)**: Generates full type definitions using pre-registered names.

This is critical for handling `x-go-name` extensions and name conflicts correctly.

### TypeTracker
Central registry for managing type names and references:
- `registerName(name)` - Reserve a type name
- `registerRef(ref, name)` - Map a $ref to a type name
- `LookupByRef(ref)` - Find type name for a $ref
- `LookupByName(name)` - Find TypeDefinition by name
- `generateUniqueName(name)` - Generate unique name if conflict exists (appends numbers)

### Circular reference handling
- `ParseOptions.visited` map tracks visited schema paths to prevent infinite recursion
- When a circular reference is detected, the generator returns a reference to the already-registered type
- Pre-registration in pass 1 ensures the type name exists before it's referenced

### Enum generation
- Enums are generated for schemas with `enum` values
- Only comparable types can have enum constants (primitives like string, int, bool)
- Non-comparable types (time.Time, uuid.UUID, arrays, structs) skip enum constant generation
- Use `isComparableType()` to check if a type can be used as an enum constant
- `nonConstantTypes` map lists Go types that cannot be constants

### Response type handling
- Response types can be configured to implement the `error` interface via `output.response-type-suffix` and error mapping
- When a response type has error mapping, it cannot be an alias (aliases don't support methods)
- Response schemas are processed similarly to component schemas but with different SpecLocation

### Union types (oneOf/anyOf)
- Union types are generated as structs with pointer fields for each variant
- Use `ContainsUnions()` method on schemas to check if they contain union elements
- Union types get custom JSON marshaling/unmarshaling to handle the polymorphism

### Name conflict resolution
- When multiple schemas would generate the same Go type name, the generator appends numbers (e.g., `Foo`, `Foo2`, `Foo3`)
- This happens at registration time via `generateUniqueName()`
- Conflicts can arise from: same names in different paths, `x-go-name` collisions, inline type extraction

## Adding a New Server Framework

When adding support for a new router/framework (e.g., `mux`), follow these steps:

### 1. Add HandlerKind constant
In `pkg/codegen/configuration.go`:
- Add new constant: `HandlerKindMux HandlerKind = "mux"` (keep constants in alphabetical order)
- Update `IsValid()` switch to include the new kind

In `configuration-schema.json`:
- Add the new kind to the `enum` array in `HandlerOptions.kind` (keep in alphabetical order)

### 2. Create framework-specific templates
Create directory `pkg/codegen/templates/handler/<framework>/` with:
- `handler.tmpl` - **Required**. Main handler template that includes shared templates via `{{template "..."}}`. See existing frameworks for pattern.
- `server.tmpl` - **Required**. Custom server setup using the framework's native server/middleware. Must wire up all 5 middleware types (recovery, request-id, logging, CORS, timeout) plus custom middleware from scaffold.
- `middleware.tmpl` - Optional. Override if framework needs custom middleware pattern (e.g., Echo has custom one)

Templates use `{{define "handler-<framework>"}}` blocks. The shared templates in `pkg/codegen/templates/handler/` provide common functionality:
- `adapter.tmpl` - Request/response adapter
- `router.tmpl` - Router registration
- `service.tmpl` - Service interface scaffold
- `service-options.tmpl` - Service request options
- `response-data.tmpl` - Response data types
- `middleware.tmpl` - Default middleware scaffold
- `server.tmpl` - Default server main.go

### Server template middleware requirements
Each framework's `server.tmpl` must demonstrate proper middleware setup:
1. **Recovery** - Panic recovery middleware
2. **Request ID** - Add unique request ID to each request
3. **Logging** - Log request method, path, status
4. **CORS** - Cross-origin resource sharing
5. **Timeout** - Request timeout handling
6. **Custom middleware** - Wire up `handler.ExampleMiddleware()` when `$config.Generate.Handler.Middleware` is set

Use the framework's native middleware where available. If the framework provides built-in middleware for 4+ of the above (e.g., Kratos has recovery, logging, tracing, metrics), you must still generate at least one example middleware in `middleware.tmpl` to demonstrate the pattern, and wire it up in `server.tmpl`. See `echo/server.tmpl` for reference.

### 3. Add standalone example
Create `examples/server/<framework>/` with:
- `cfg.yml` - Config with `generate.handler.kind: <framework>` and `generate.handler.output.overwrite: true`
- `generate.go` - `//go:generate` directive
- `README.md` - Documentation for the example (see below)
- Generated files will be created in `api/` and `server/` subdirectories

**Important**: Standalone examples must have `generate.handler.output.overwrite: true` so scaffold files (service.go, middleware.go) are regenerated when the API spec changes.

#### README.md requirements
Each server example must have a README.md with:
- Framework name and link to the framework's GitHub repository
- Description section with framework-specific notes (middleware pattern, path params, context type, etc.)
- Shell block showing how to start the server
- Separate curl examples for each of the 5 API endpoints:
  - `GET /health` - Health check
  - `GET /users` - List users
  - `POST /users` - Create user
  - `GET /users/{id}` - Get user by ID
  - `DELETE /users/{id}` - Delete user

### 4. Add test case
Create `examples/server/test/<framework>/testcase/` with:
- `cfg.yml` - Config for test case
- `generate.go` - Generate directive that copies shared service.go.src:
  ```go
  //go:generate cp ../../testcase/gen.go ./gen.go
  //go:generate cp ../../testcase/service.go.src ./service.go
  //go:generate go run github.com/doordash-oss/oapi-codegen-dd/v3/cmd/oapi-codegen -config cfg.yml ../../../api.yml
  ```

### 5. Update test file
In `examples/server/test/server_test.go`:
- Add import for new testcase package
- Add new framework to the `frameworks` slice in test functions

### 6. Regenerate and test
```bash
# Only regenerate the specific examples you're working on, NOT all examples
cd examples/server/<framework> && go generate ./...
cd examples/server/test/<framework> && go generate ./...
cd examples && go build ./...    # Verify builds
cd examples && go test ./server/test/...     # Run tests including new framework
make lint                        # Check for lint issues
```

**Important**: Do NOT run `make generate` when adding a new framework - only regenerate the specific examples you're working on.

## Documentation

Documentation is in `docs/` using MkDocs. Key files:

- `docs/mcp-server.md` - MCP server generation guide
- `docs/server-generation.md` - HTTP server generation guide
- `docs/extensions/x-*.md` - Extension documentation
- `mkdocs.yml` - Navigation and config

### Adding documentation
1. Create markdown file in `docs/`
2. Add to `nav:` section in `mkdocs.yml`
3. For extensions, create in `docs/extensions/` and add under Extensions nav

### Code snippets in docs
Documentation uses MkDocs snippets to include code from example files. The format is:
```
--8<-- "path/to/file.go:start_line:end_line"
```
For example: `--8<-- "extensions/xgotype/gen.go:11:14"` includes lines 11-14 from that file.

**After regenerating examples**, verify that line number references in docs are still correct:
```bash
grep -rn '\-\-8<\-\-' docs/ | grep -E ':[0-9]+:[0-9]+'
```
Then check each referenced file to ensure the line ranges still point to the expected code.

### Local preview
```bash
pip install mkdocs-material
mkdocs serve
```
