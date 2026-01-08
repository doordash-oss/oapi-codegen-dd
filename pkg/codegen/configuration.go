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

import "time"

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
		if o.Generate == nil {
			o.Generate = &GenerateOptions{
				Client:                 defaults.Generate.Client,
				AlwaysPrefixEnumValues: defaults.Generate.AlwaysPrefixEnumValues,
				DefaultIntType:         defaults.Generate.DefaultIntType,
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

	// OmitDescription specifies whether to omit schema description from the spec in the generated code. Defaults to false.
	OmitDescription bool `yaml:"omit-description"`

	// DefaultIntType specifies the default integer type to use. Defaults to "int".
	DefaultIntType string `yaml:"default-int-type"`

	// AlwaysPrefixEnumValues specifies whether to always prefix enum values with the schema name. Defaults to true.
	AlwaysPrefixEnumValues bool `yaml:"always-prefix-enum-values"`

	// ResponseValidators specifies whether to generate Validate() methods for response types.
	// Useful for contract testing to ensure responses match the OpenAPI spec. Defaults to false.
	ResponseValidators bool `yaml:"response-validators"`
}

type Output struct {
	UseSingleFile bool   `yaml:"use-single-file"`
	Directory     string `yaml:"directory"`
	Filename      string `yaml:"filename"`
}

type Client struct {
	Name    string        `yaml:"name"`
	Timeout time.Duration `yaml:"timeout"`
}

// NewDefaultConfiguration creates a new default Configuration.
func NewDefaultConfiguration() Configuration {
	return Configuration{
		PackageName:     "gen",
		CopyrightHeader: "Code generated by oapi-codegen. DO NOT EDIT.",
		Generate: &GenerateOptions{
			DefaultIntType:         "int",
			AlwaysPrefixEnumValues: true,
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
