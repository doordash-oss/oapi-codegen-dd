// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package codegen

import (
	"fmt"
	"time"
)

// Configuration defines code generation customizations.
// PackageName to generate the code under.
// CopyrightHeader is the header to add to the generated code. Use without //.
// SkipPrune indicates whether to skip pruning unused components on the generated code.
// Output specifies the output options for the generated code.
//
// Filter is the configuration for filtering the paths and operations to be parsed.
//
// AdditionalImports defines any additional Go imports to add to the generated code.
// ErrorMapping is the configuration for mapping the OpenAPI error responses to Go types.
//
//	The key is the spec error type name
//	and the value is the dotted json path to the string result.
//
// UserTemplates is the map of user-provided templates overriding the default ones.
// UserContext is the map of user-provided context values to be used in templates user overrides.
type Configuration struct {
	PackageName     string  `yaml:"package"`
	CopyrightHeader string  `yaml:"copyright-header"`
	SkipPrune       bool    `yaml:"skip-prune"`
	Output          *Output `yaml:"output"`

	Generate *GenerateOptions `yaml:"generate"`
	Filter   FilterConfig     `yaml:"filter,omitempty"`
	Overlay  *OverlayOptions  `yaml:"overlay,omitempty"`

	AdditionalImports []AdditionalImport `yaml:"additional-imports,omitempty"`
	ErrorMapping      map[string]string  `yaml:"error-mapping,omitempty"`
	Client            *Client            `yaml:"client,omitempty"`

	UserTemplates map[string]string `yaml:"user-templates,omitempty"`
	UserContext   map[string]any    `yaml:"user-context,omitempty"`
}

// Merge combines two configurations, with the receiver (o) taking priority.
// Empty fields in o are filled with values from other.
// This operation is not commutative: a.Merge(b) != b.Merge(a).
//
// Deprecated: Use WithDefaults() instead for clearer intent.
func (o Configuration) Merge(other Configuration) Configuration {
	return o.WithDefaults()
}

// WithDefaults fills empty fields in the configuration with sensible defaults.
// The receiver takes priority - only empty/nil fields are filled with defaults from NewDefaultConfiguration().
func (o Configuration) WithDefaults() Configuration {
	defaults := NewDefaultConfiguration()

	if o.PackageName == "" {
		o.PackageName = defaults.PackageName
	}
	if o.CopyrightHeader == "" {
		o.CopyrightHeader = defaults.CopyrightHeader
	}

	if o.Output != nil {
		if o.Output.Directory == "" {
			o.Output.Directory = defaults.Output.Directory
		}
		if o.Output.Filename == "" {
			o.Output.Filename = defaults.Output.Filename
		}
	} else {
		o.Output = defaults.Output
	}

	if o.Generate == nil {
		o.Generate = defaults.Generate
	} else {
		// Fill in missing Generate fields from defaults
		if o.Generate.DefaultIntType == "" {
			o.Generate.DefaultIntType = defaults.Generate.DefaultIntType
		}
		// Fill in Handler defaults if Handler is configured
		if o.Generate.Handler != nil {
			if o.Generate.Handler.Name == "" {
				o.Generate.Handler.Name = "Service"
			}
			if o.Generate.Handler.Kind == "" {
				o.Generate.Handler.Kind = HandlerKindChi
			}
			if o.Generate.Handler.MultipartMaxMemory == 0 {
				o.Generate.Handler.MultipartMaxMemory = 32
			}
		}
	}

	if o.Client == nil {
		o.Client = defaults.Client
	}
	if o.Client.Name == "" {
		o.Client.Name = defaults.Client.Name
	}
	if o.Client.Timeout == 0 {
		o.Client.Timeout = defaults.Client.Timeout
	}

	return o
}

// OverwriteWith overwrites fields in the configuration with non-empty values from other.
// The parameter takes priority - non-empty fields from other overwrite the receiver.
func (o Configuration) OverwriteWith(other Configuration) Configuration {
	// Overwrite simple string fields
	if other.PackageName != "" {
		o.PackageName = other.PackageName
	}
	if other.CopyrightHeader != "" {
		o.CopyrightHeader = other.CopyrightHeader
	}

	// Overwrite SkipPrune
	if other.SkipPrune {
		o.SkipPrune = other.SkipPrune
	}

	// Overwrite Output
	if other.Output != nil {
		if o.Output == nil {
			o.Output = other.Output
		} else {
			if other.Output.Directory != "" {
				o.Output.Directory = other.Output.Directory
			}
			if other.Output.Filename != "" {
				o.Output.Filename = other.Output.Filename
			}
			if other.Output.UseSingleFile {
				o.Output.UseSingleFile = other.Output.UseSingleFile
			}
		}
	}

	// Overwrite Generate options
	if other.Generate != nil {
		if o.Generate == nil {
			o.Generate = other.Generate
		} else {
			if other.Generate.Client {
				o.Generate.Client = other.Generate.Client
			}
			if other.Generate.OmitDescription {
				o.Generate.OmitDescription = other.Generate.OmitDescription
			}
			if other.Generate.DefaultIntType != "" {
				o.Generate.DefaultIntType = other.Generate.DefaultIntType
			}
			if other.Generate.AlwaysPrefixEnumValues {
				o.Generate.AlwaysPrefixEnumValues = other.Generate.AlwaysPrefixEnumValues
			}
			// Overwrite Validation options
			if other.Generate.Validation.Skip {
				o.Generate.Validation.Skip = other.Generate.Validation.Skip
			}
			if other.Generate.Validation.Simple {
				o.Generate.Validation.Simple = other.Generate.Validation.Simple
			}
			if other.Generate.Validation.Response {
				o.Generate.Validation.Response = other.Generate.Validation.Response
			}

			// Overwrite Handler options
			if other.Generate.Handler != nil {
				if o.Generate.Handler == nil {
					o.Generate.Handler = other.Generate.Handler
				} else {
					if other.Generate.Handler.Name != "" {
						o.Generate.Handler.Name = other.Generate.Handler.Name
					}
					if other.Generate.Handler.Kind != "" {
						o.Generate.Handler.Kind = other.Generate.Handler.Kind
					}
					if other.Generate.Handler.Validation.Request {
						o.Generate.Handler.Validation.Request = other.Generate.Handler.Validation.Request
					}
					if other.Generate.Handler.Validation.Response {
						o.Generate.Handler.Validation.Response = other.Generate.Handler.Validation.Response
					}
				}
			}
		}
	}

	// Overwrite Client
	if other.Client != nil {
		if o.Client == nil {
			o.Client = other.Client
		} else {
			if other.Client.Name != "" {
				o.Client.Name = other.Client.Name
			}
			if other.Client.Timeout != 0 {
				o.Client.Timeout = other.Client.Timeout
			}
		}
	}

	// Overwrite Filter
	if !other.Filter.IsEmpty() {
		o.Filter = other.Filter
	}

	// Overwrite AdditionalImports
	if len(other.AdditionalImports) > 0 {
		o.AdditionalImports = other.AdditionalImports
	}

	// Overwrite ErrorMapping
	if len(other.ErrorMapping) > 0 {
		o.ErrorMapping = other.ErrorMapping
	}

	// Overwrite UserTemplates
	if len(other.UserTemplates) > 0 {
		o.UserTemplates = other.UserTemplates
	}

	// Overwrite UserContext
	if len(other.UserContext) > 0 {
		o.UserContext = other.UserContext
	}

	return o
}

type AdditionalImport struct {
	Alias   string `yaml:"alias,omitempty"`
	Package string `yaml:"package"`
}

// FilterConfig is the configuration for filtering the paths and operations to be parsed.
type FilterConfig struct {
	Include FilterParamsConfig `yaml:"include"`
	Exclude FilterParamsConfig `yaml:"exclude"`
}

// IsEmpty returns true if the filter is empty.
func (o FilterConfig) IsEmpty() bool {
	return o.Include.IsEmpty() && o.Exclude.IsEmpty()
}

// FilterParamsConfig is the configuration for filtering the paths to be parsed.
type FilterParamsConfig struct {
	Paths            []string            `yaml:"paths"`
	Tags             []string            `yaml:"tags"`
	OperationIDs     []string            `yaml:"operation-ids"`
	SchemaProperties map[string][]string `yaml:"schema-properties"`
	Extensions       []string            `yaml:"extensions"`
}

// IsEmpty returns true if the filter is empty.
func (o FilterParamsConfig) IsEmpty() bool {
	return len(o.Paths) == 0 &&
		len(o.Tags) == 0 &&
		len(o.OperationIDs) == 0 &&
		len(o.SchemaProperties) == 0 &&
		len(o.Extensions) == 0
}

type GenerateOptions struct {
	// Client specifies whether to generate a client. Defaults to false.
	Client bool `yaml:"client"`

	// Models specifies whether to generate model types. Defaults to true.
	// Set to false when models are generated in a separate package.
	Models *bool `yaml:"models,omitempty"`

	// Handler specifies options for handler/server code generation.
	// If nil, no handler code is generated.
	Handler *HandlerOptions `yaml:"handler,omitempty"`

	// MCPServer specifies options for MCP (Model Context Protocol) server generation.
	// If set, generates MCP tools that wrap the generated client for AI assistant integration.
	// Requires client generation to be enabled.
	MCPServer *MCPServerOptions `yaml:"mcp-server,omitempty"`

	// OmitDescription specifies whether to omit schema description from the spec in the generated code. Defaults to false.
	OmitDescription bool `yaml:"omit-description"`

	// DefaultIntType specifies the default integer type to use. Defaults to "int".
	DefaultIntType string `yaml:"default-int-type"`

	// AlwaysPrefixEnumValues specifies whether to always prefix enum values with the schema name. Defaults to true.
	AlwaysPrefixEnumValues bool `yaml:"always-prefix-enum-values"`

	// Validation specifies options for Validate() method generation.
	Validation ValidationOptions `yaml:"validation"`
}

type ValidationOptions struct {
	// Skip specifies whether to skip Validation method generation. Defaults to false.
	Skip bool `yaml:"skip"`

	// Simple specifies whether to use the simple validation approach. Defaults to false.
	// Simple validation uses validate.Struct() for all types, whereas complex validation generates custom Validate() methods.
	Simple bool `yaml:"simple"`

	// Response specifies whether to generate Validate() methods for response types.
	// Useful for contract testing to ensure responses match the OpenAPI spec. Defaults to false.
	Response bool `yaml:"response"`
}

type Output struct {
	UseSingleFile bool   `yaml:"use-single-file"`
	Directory     string `yaml:"directory"`
	Filename      string `yaml:"filename"`
}

// OverlayOptions specifies OpenAPI Overlay files to apply to the spec before generation.
// See https://spec.openapis.org/overlay/v1.0.0.html for the Overlay specification.
type OverlayOptions struct {
	// Sources is a list of overlay files to apply to the OpenAPI spec.
	// Each source can be a file path or URL. Overlays are applied in order.
	Sources []string `yaml:"sources"`
}

type Client struct {
	Name    string        `yaml:"name"`
	Timeout time.Duration `yaml:"timeout"`
}

// HandlerKind specifies the router/framework to generate handler code for.
type HandlerKind string

const (
	HandlerKindBeego      HandlerKind = "beego"
	HandlerKindChi        HandlerKind = "chi"
	HandlerKindEcho       HandlerKind = "echo"
	HandlerKindFastHTTP   HandlerKind = "fasthttp"
	HandlerKindFiber      HandlerKind = "fiber"
	HandlerKindGin        HandlerKind = "gin"
	HandlerKindGoFrame    HandlerKind = "goframe"
	HandlerKindGoZero     HandlerKind = "go-zero"
	HandlerKindGorillaMux HandlerKind = "gorilla-mux"
	HandlerKindHertz      HandlerKind = "hertz"
	HandlerKindIris       HandlerKind = "iris"
	HandlerKindKratos     HandlerKind = "kratos"
	HandlerKindStdHTTP    HandlerKind = "std-http"
)

// IsValid returns true if the handler kind is a supported value.
func (k HandlerKind) IsValid() bool {
	switch k {
	case HandlerKindBeego, HandlerKindChi, HandlerKindEcho, HandlerKindFastHTTP, HandlerKindFiber, HandlerKindGin, HandlerKindGoFrame, HandlerKindGoZero, HandlerKindGorillaMux, HandlerKindHertz, HandlerKindIris, HandlerKindKratos, HandlerKindStdHTTP:
		return true
	default:
		return false
	}
}

// HandlerOptions specifies options for handler/server code generation.
type HandlerOptions struct {
	// Name is the name of the service interface. Defaults to "Service".
	Name string `yaml:"name"`

	// Kind specifies the router/framework to generate for. Defaults to "chi".
	// Supported values: "chi"
	Kind HandlerKind `yaml:"kind"`

	// Validation specifies options for request/response validation in handlers.
	Validation HandlerValidation `yaml:"validation"`

	// ModelsPackageAlias is the package alias to prefix model types with.
	// Used when models are generated separately (generate.models: false).
	// Example: "types" will generate "types.User" instead of "User".
	ModelsPackageAlias string `yaml:"models-package-alias"`

	// MultipartMaxMemory is the maximum memory in MB for multipart form parsing.
	// Defaults to 32MB (matching Go stdlib). Files exceeding this are stored in temp files.
	MultipartMaxMemory int `yaml:"multipart-max-memory"`

	// Output specifies output for scaffolded handler files (service.go, middleware.go).
	// Falls back to root output if nil.
	Output *ScaffoldOutput `yaml:"output"`

	// Middleware specifies options for generating middleware.go.
	// If nil, no middleware is generated.
	Middleware *MiddlewareOptions `yaml:"middleware"`

	// Server specifies options for generating a runnable server main.go.
	// If nil, no server is generated.
	Server *ServerOptions `yaml:"server"`
}

// ResolveScaffoldOutput returns the output config for scaffold files (service.go, middleware.go).
// Uses handler.output if set, otherwise falls back to root output.
func (o HandlerOptions) ResolveScaffoldOutput(rootOutput *Output) ScaffoldOutput {
	if o.Output != nil {
		return *o.Output
	}
	return ScaffoldOutput{
		Directory: rootOutput.Directory,
		Package:   "", // will use root package name
	}
}

// ResolveServerOutput returns the output config for server/main.go.
// Uses server.directory if set, otherwise defaults to "server".
// Package is always "main" for server.
func (o HandlerOptions) ResolveServerOutput() ScaffoldOutput {
	dir := "server"
	if o.Server != nil && o.Server.Directory != "" {
		dir = o.Server.Directory
	}
	return ScaffoldOutput{
		Directory: dir,
		Package:   "main",
	}
}

// Validate returns an error if the handler options are invalid.
func (o HandlerOptions) Validate() error {
	if o.Kind == "" {
		return ErrHandlerKindRequired
	}
	if !o.Kind.IsValid() {
		return fmt.Errorf("%w: %q", ErrHandlerKindUnsupported, o.Kind)
	}
	return nil
}

// HandlerValidation specifies validation options for handlers.
type HandlerValidation struct {
	// Request enables validation of incoming requests. Defaults to false.
	Request bool `yaml:"request"`

	// Response enables validation of outgoing responses. Defaults to false.
	// Useful for contract testing.
	Response bool `yaml:"response"`
}

// MiddlewareOptions specifies options for generating middleware.go.
// Currently empty but allows for future extensibility.
type MiddlewareOptions struct {
}

// MCPServerOptions specifies options for MCP (Model Context Protocol) server generation.
// MCP servers expose API operations as tools that AI assistants (Claude, Cursor, etc.) can invoke.
// The generated code wraps the generated client to make real API calls.
type MCPServerOptions struct {
	// DefaultSkip specifies whether operations are skipped by default.
	// If true, operations are excluded unless x-mcp.skip is explicitly false.
	// If false (default), operations are included unless x-mcp.skip is true.
	DefaultSkip bool `yaml:"default-skip"`
}

// ScaffoldOutput specifies output options for scaffolded files.
// Scaffold files are always generated as separate files (not merged).
type ScaffoldOutput struct {
	// Directory is the output directory, relative to the spec/config file location.
	Directory string `yaml:"directory"`

	// Package is the package name for the generated file.
	Package string `yaml:"package"`

	// Overwrite forces regeneration of scaffold-once files (e.g., service.go, middleware.go).
	// Normally these files are only generated if they don't exist. Defaults to false.
	Overwrite bool `yaml:"overwrite"`
}

// ServerOptions specifies options for generating a runnable server main.go.
type ServerOptions struct {
	// Directory is the output directory for server/main.go.
	// Defaults to "server".
	Directory string `yaml:"directory"`

	// Port is the port the server listens on. Defaults to 8080.
	Port int `yaml:"port"`

	// Timeout is the request timeout in seconds. Defaults to 30.
	Timeout int `yaml:"timeout"`

	// HandlerPackage is the full import path of the handler package.
	// Required when server generation is enabled.
	HandlerPackage string `yaml:"handler-package"`
}

// WithDefaults returns a copy of ServerOptions with default values applied.
func (o ServerOptions) WithDefaults() ServerOptions {
	if o.Directory == "" {
		o.Directory = "server"
	}
	if o.Port == 0 {
		o.Port = 8080
	}
	if o.Timeout == 0 {
		o.Timeout = 30
	}
	return o
}

// Validate returns an error if the server options are invalid.
func (o ServerOptions) Validate() error {
	if o.HandlerPackage == "" {
		return ErrServerHandlerPackageRequired
	}
	return nil
}

// NewDefaultConfiguration creates a new default Configuration.
func NewDefaultConfiguration() Configuration {
	return Configuration{
		PackageName:     "gen",
		CopyrightHeader: "Code generated by oapi-codegen. DO NOT EDIT.",
		Generate: &GenerateOptions{
			DefaultIntType: "int",
			Validation:     ValidationOptions{},
		},
		Output: &Output{
			Directory:     ".",
			UseSingleFile: true,
			Filename:      "gen.go",
		},
		Client: &Client{
			Name:    "Client",
			Timeout: 3 * time.Second,
		},
	}
}

type keyValue[K, V any] struct {
	key   K
	value V
}
