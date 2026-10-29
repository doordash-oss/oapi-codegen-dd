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
	"go/ast"
	"go/parser"
	"log/slog"
	"maps"
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

// templateHardcodedImports lists import paths that are already present in header.tmpl.
// GoImports() filters these out to avoid duplicate imports in generated code.
var templateHardcodedImports = map[string]bool{
	"bytes":           true,
	"compress/gzip":   true,
	"context":         true,
	"encoding/base64": true,
	"encoding/json":   true,
	"encoding/xml":    true,
	"errors":          true,
	"fmt":             true,
	"io":              true,
	"os":              true,
	"mime":            true,
	"mime/multipart":  true,
	"net/http":        true,
	"net/url":         true,
	"path":            true,
	"strings":         true,
	"time":            true,
	"log/slog":        true,
	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime": true,
	"github.com/google/go-querystring/query":                 true,
	"github.com/go-playground/validator/v10":                 true,
}

// builtinTypeImports lists the packages behind the Go types generated for built-in formats, by the name
// the generated code uses for them. header.tmpl leaves them out, a type that uses one adds its import.
var builtinTypeImports = map[string]goImport{
	"uuid": {Path: "github.com/google/uuid"},
}

// maxListedSchemas caps how many schemas a warning lists, an overlay can match hundreds.
const maxListedSchemas = 5

// GoImports returns a slice of go import statements
func (im importMap) GoImports() []string {
	goImports := make([]string, 0, len(im))
	for _, v := range im {
		if v.Path == importMappingCurrentPackage {
			continue
		}
		if v.Name == "" && templateHardcodedImports[v.Path] {
			continue
		}
		goImports = append(goImports, v.String())
	}
	return goImports
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

// goTypePackages returns the names of the packages a Go type expression refers to, e.g. uuid for []uuid.UUID.
// An expression that does not parse refers to none.
func goTypePackages(goType string) []string {
	expr, err := parser.ParseExpr(goType)
	if err != nil {
		return nil
	}

	var res []string
	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && !slices.Contains(res, pkg.Name) {
			res = append(res, pkg.Name)
		}
		return true
	})
	return res
}

// isImported reports whether an import, or an additional-imports entry, binds the package gi to name.
func isImported(imports importMap, additional []AdditionalImport, name string, gi goImport) bool {
	for _, v := range imports {
		if v.Path == gi.Path && (v.Name == "" || v.Name == name) {
			return true
		}
	}

	for _, a := range additional {
		if a.Package == gi.Path && (a.Alias == "" || a.Alias == name) {
			return true
		}
	}
	return false
}

// warnUnimportedGoTypes reports the x-go-type values that name a built-in package without an x-go-type-import
// while nothing else imports it. header.tmpl no longer imports these packages for every file, so such a
// schema builds only if goimports happens to find a package of that name.
func warnUnimportedGoTypes(imports importMap, additional []AdditionalImport, unimported map[string][]string) {
	for _, name := range slices.Sorted(maps.Keys(unimported)) {
		builtin, ok := builtinTypeImports[name]
		if !ok || isImported(imports, additional, name, builtin) {
			continue
		}

		schemas := slices.Compact(slices.Sorted(slices.Values(unimported[name])))
		slog.Warn(fmt.Sprintf("x-go-type: package %s is only imported when a type uses %s, so a schema that names it without an x-go-type-import may not build", name, builtin.Path),
			"schemas", listSchemas(schemas),
			"hint", fmt.Sprintf("set x-go-type-import to the package you want, e.g. {path: %s}", builtin.Path))
	}
}

func listSchemas(schemas []string) string {
	if len(schemas) <= maxListedSchemas {
		return strings.Join(schemas, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(schemas[:maxListedSchemas], ", "), len(schemas)-maxListedSchemas)
}
