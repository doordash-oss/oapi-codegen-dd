package codegen

import (
	"errors"
	"fmt"
	"sort"
)

const (
	// TODO: check if needed
	disableTypeAliasesForArray = true
)

// Configuration defines code generation customizations.
// PackageName to generate the code under.
// SkipPrune indicates whether to skip pruning unused components on the generated code.
//
// OutputOptions are used to modify the output code in some way.
// ImportMapping specifies the golang package path for each external reference
// AdditionalImports defines any additional Go imports to add to the generated code
// ErrorMapping is the configuration for mapping the OpenAPI error responses to Go types.
//
//	The key is the spec error type name
//	and the value is the dotted json path to the string result.
type Configuration struct {
	PackageName string `yaml:"package"`
	SkipPrune   bool   `yaml:"skip-prune,omitempty"`

	Filter        FilterConfig      `yaml:"filter,omitempty"`
	UserTemplates map[string]string `yaml:"user-templates,omitempty"`

	InitialismOverrides bool `yaml:"initialism-overrides,omitempty"`

	DisableTypeAliasesForType []string `yaml:"disable-type-aliases-for-type"`
	NameNormalizer            string   `yaml:"name-normalizer,omitempty"`

	ImportMapping     map[string]string  `yaml:"import-mapping,omitempty"`
	AdditionalImports []AdditionalImport `yaml:"additional-imports,omitempty"`
	ErrorMapping      map[string]string  `yaml:"error-mapping,omitempty"`
}

// Validate checks whether Configuration represent a valid configuration
func (o Configuration) Validate() error {
	if o.PackageName == "" {
		return errors.New("package name must be specified")
	}

	return nil
}

type AdditionalImport struct {
	Alias   string `yaml:"alias,omitempty"`
	Package string `yaml:"package"`
}

// FilterConfig is the configuration for filtering the paths and operations to be parsed.
type FilterConfig struct {
	Include FilterParamsConfig
	Exclude FilterParamsConfig
}

// FilterParamsConfig is the configuration for filtering the paths to be parsed.
type FilterParamsConfig struct {
	Paths        []string
	Tags         []string
	OperationIDs []string
}

// NewDefaultConfiguration creates a new default Configuration.
func NewDefaultConfiguration() *Configuration {
	return &Configuration{
		PackageName: "gen",
	}
}

type keyValue[K, V any] struct {
	key   K
	value V
}

func newImportMap(importMapping map[string]string) importMap {
	var (
		pathToName = map[string]string{}
		result     = importMap{}
	)

	var packagePaths []string
	for _, packageName := range importMapping {
		packagePaths = append(packagePaths, packageName)
	}
	sort.Strings(packagePaths)

	for _, packagePath := range packagePaths {
		if _, ok := pathToName[packagePath]; !ok && packagePath != importMappingCurrentPackage {
			pathToName[packagePath] = fmt.Sprintf("externalRef%d", len(pathToName))
		}
	}

	for specPath, packagePath := range importMapping {
		result[specPath] = goImport{Name: pathToName[packagePath], Path: packagePath}
	}
	return result
}
