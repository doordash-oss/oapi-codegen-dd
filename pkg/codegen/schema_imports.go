package codegen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
)

// goImport represents a go package to be imported in the generated code
type goImport struct {
	Name string // package name
	Path string // package path
}

// String returns a go import statement
func (gi goImport) String() string {
	if gi.Name != "" {
		return fmt.Sprintf("%s %q", gi.Name, gi.Path)
	}
	return fmt.Sprintf("%q", gi.Path)
}

// importMap maps external OpenAPI specifications files/urls to external go packages
type importMap map[string]goImport

// importMappingCurrentPackage allows an Import Mapping to map to the current package, rather than an external package.
// This allows users to split their OpenAPI specification across multiple files, but keep them in the same package, which can reduce a bit of the overhead for users.
// We use `-` to indicate that this is a bit of a special case
const importMappingCurrentPackage = "-"

// GoImports returns a slice of go import statements
func (im importMap) GoImports() []string {
	goImports := make([]string, 0, len(im))
	for _, v := range im {
		if v.Path == importMappingCurrentPackage {
			continue
		}
		goImports = append(goImports, v.String())
	}
	return goImports
}

func collectSchemaImports(s GoSchema) (map[string]goImport, error) {
	res := map[string]goImport{}

	for _, p := range s.Properties {
		imprts, err := getOpenAPISchemaImports(p.Schema.OpenAPISchema)
		if err != nil {
			return nil, err
		}
		mergeImports(res, imprts)
	}

	imprts, err := getOpenAPISchemaImports(s.OpenAPISchema)
	if err != nil {
		return nil, err
	}
	mergeImports(res, imprts)
	return res, nil
}

func getOpenAPISchemaImports(schema *base.Schema) (map[string]goImport, error) {
	res := map[string]goImport{}

	if schema == nil || (schema.ParentProxy != nil && schema.ParentProxy.IsReference()) {
		return nil, nil
	}

	if gi, err := parseGoImportExtension(schema); err != nil {
		return nil, err
	} else {
		if gi != nil {
			res[gi.String()] = *gi
		}
	}

	t := schema.Type
	if slices.Contains(t, "object") {
		for _, v := range schema.Properties.FromOldest() {
			imprts, err := getOpenAPISchemaImports(v.Schema())
			if err != nil {
				return nil, err
			}
			mergeImports(res, imprts)
		}
	} else if slices.Contains(t, "array") {
		if schema.Items == nil {
			return nil, nil
		}
		if schema.Items.IsA() && schema.Items.A != nil {
			imprts, err := getOpenAPISchemaImports(schema.Items.A.Schema())
			if err != nil {
				return nil, err
			}
			mergeImports(res, imprts)
		}
	}

	return res, nil
}

func parseGoImportExtension(v *base.Schema) (*goImport, error) {
	if v == nil || v.Extensions == nil {
		return nil, nil
	}

	if v.Extensions.Value(extPropGoImport) == nil || v.Extensions.Value(extPropGoType) == nil {
		return nil, nil
	}

	importI := map[string]any{}
	goTypeImportExt := v.Extensions.Value(extPropGoImport)
	// TODO: check if this is correct
	if err := goTypeImportExt.Decode(&importI); err != nil {
		return nil, err
	}

	gi := goImport{}
	// replicate the case-insensitive field mapping json.Unmarshal would do
	for k, v := range importI {
		if strings.EqualFold(k, "name") {
			if vs, ok := v.(string); ok {
				gi.Name = vs
			} else {
				return nil, fmt.Errorf("failed to convert type: %T", v)
			}
		} else if strings.EqualFold(k, "path") {
			if vs, ok := v.(string); ok {
				gi.Path = vs
			} else {
				return nil, fmt.Errorf("failed to convert type: %T", v)
			}
		}
	}

	return &gi, nil
}

func mergeImports(dst, src map[string]goImport) {
	for k, v := range src {
		dst[k] = v
	}
}
