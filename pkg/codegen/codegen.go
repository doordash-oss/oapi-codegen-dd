// Copyright 2019 DeepMap, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codegen

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"runtime/debug"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/tools/imports"
)

// ParseContext holds the OpenAPI models.
type ParseContext struct {
	Operations               []OperationDefinition
	TypeDefinitions          map[SpecLocation][]TypeDefinition
	Enums                    []EnumDefinition
	UnionTypes               []TypeDefinition
	AdditionalTypes          []TypeDefinition
	UnionWithAdditionalTypes []TypeDefinition
	Imports                  []string
}

// globalState stores all global state. Please don't put global state anywhere
// else so that we can easily track it.
var globalState struct {
	options       Configuration
	spec          *openapi3.T
	importMapping importMap
	// initialismsMap stores initialisms as "lower(initialism) -> initialism" map.
	// List of initialisms was taken from https://staticcheck.io/docs/configuration/options/#initialisms.
	initialismsMap map[string]string
}

// Generate uses the Go templating engine to generate all of our server wrappers from
// the descriptions we've built up above from the schema objects.
// opts defines
func Generate(spec *openapi3.T, opts Configuration) (string, error) {
	// This is global state
	globalState.options = opts
	globalState.spec = spec
	globalState.importMapping = constructImportMapping(opts.ImportMapping)

	filterOperationsByTag(spec, opts)
	filterOperationsByOperationID(spec, opts)
	if !opts.SkipPrune {
		pruneUnusedComponents(spec)
	}

	nameNormalizer = ToCamelCaseWithInitialisms

	globalState.initialismsMap = makeInitialismsMap(opts.AdditionalInitialisms)

	// This creates the golang templates text package
	t := template.New("oapi-codegen").Funcs(TemplateFunctions)
	// This parses all of our own template files into the template object
	// above
	err := LoadTemplates(templates, t)
	if err != nil {
		return "", fmt.Errorf("error parsing oapi-codegen templates: %w", err)
	}

	// load user-provided templates. Will Override built-in versions.
	for name, template := range opts.UserTemplates {
		utpl := t.New(name)

		txt, err := GetUserTemplateText(template)
		if err != nil {
			return "", fmt.Errorf("error loading user-provided template %q: %w", name, err)
		}

		_, err = utpl.Parse(txt)
		if err != nil {
			return "", fmt.Errorf("error parsing user-provided template %q: %w", name, err)
		}
	}

	ops, err := OperationDefinitions(spec, opts.InitialismOverrides)
	if err != nil {
		return "", fmt.Errorf("error creating operation definitions: %w", err)
	}

	xGoTypeImports, err := OperationImports(ops)
	if err != nil {
		return "", fmt.Errorf("error getting operation imports: %w", err)
	}

	var typeDefinitions, constantDefinitions string

	typeDefinitions, err = GenerateTypeDefinitions(t, spec, ops)
	if err != nil {
		return "", fmt.Errorf("error generating type definitions: %w", err)
	}

	imprts, err := GetTypeDefinitionsImports(spec)
	if err != nil {
		return "", fmt.Errorf("error getting type definition imports: %w", err)
	}
	MergeImports(xGoTypeImports, imprts)

	clientOut, err := GenerateClient(t, ops)
	if err != nil {
		return "", fmt.Errorf("error generating client: %w", err)
	}

	var clientWithResponsesOut string
	clientWithResponsesOut, err = GenerateClientWithResponses(t, ops)
	if err != nil {
		return "", fmt.Errorf("error generating client with responses: %w", err)
	}

	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	externalImports := append(globalState.importMapping.GoImports(), importMap(xGoTypeImports).GoImports()...)
	importsOut, err := GenerateImports(
		t,
		externalImports,
		opts.PackageName,
	)
	if err != nil {
		return "", fmt.Errorf("error generating imports: %w", err)
	}

	_, err = w.WriteString(importsOut)
	if err != nil {
		return "", fmt.Errorf("error writing imports: %w", err)
	}

	_, err = w.WriteString(constantDefinitions)
	if err != nil {
		return "", fmt.Errorf("error writing constants: %w", err)
	}

	_, err = w.WriteString(typeDefinitions)
	if err != nil {
		return "", fmt.Errorf("error writing type definitions: %w", err)
	}

	_, err = w.WriteString(clientOut)
	if err != nil {
		return "", fmt.Errorf("error writing client: %w", err)
	}
	_, err = w.WriteString(clientWithResponsesOut)
	if err != nil {
		return "", fmt.Errorf("error writing client: %w", err)
	}

	err = w.Flush()
	if err != nil {
		return "", fmt.Errorf("error flushing output buffer: %w", err)
	}

	// remove any byte-order-marks which break Go-Code
	goCode := sanitizeCode(buf.String())

	outBytes, err := imports.Process(opts.PackageName+".go", []byte(goCode), nil)
	if err != nil {
		return "", fmt.Errorf("error formatting Go code %s: %w", goCode, err)
	}
	return string(outBytes), nil
}

func GenerateTypeDefinitions(t *template.Template, swagger *openapi3.T, ops []OperationDefinition) (string, error) {
	var allTypes []TypeDefinition
	if swagger.Components != nil {
		schemaTypes, err := GenerateTypesForSchemas(swagger.Components.Schemas)
		if err != nil {
			return "", fmt.Errorf("error generating Go types for component schemas: %w", err)
		}

		paramTypes, err := GenerateTypesForParameters(swagger.Components.Parameters)
		if err != nil {
			return "", fmt.Errorf("error generating Go types for component parameters: %w", err)
		}
		allTypes = append(schemaTypes, paramTypes...)

		responseTypes, err := GenerateTypesForResponses(swagger.Components.Responses)
		if err != nil {
			return "", fmt.Errorf("error generating Go types for component responses: %w", err)
		}
		allTypes = append(allTypes, responseTypes...)

		bodyTypes, err := GenerateTypesForRequestBodies(swagger.Components.RequestBodies)
		if err != nil {
			return "", fmt.Errorf("error generating Go types for component request bodies: %w", err)
		}
		allTypes = append(allTypes, bodyTypes...)
	}

	// Go through all operations, and add their types to allTypes, so that we can
	// scan all of them for enums. Operation definitions are handled differently
	// from the rest, so let's keep track of enumTypes separately, which will contain
	// all types needed to be scanned for enums, which includes those within operations.
	enumTypes := allTypes
	for _, op := range ops {
		enumTypes = append(enumTypes, op.TypeDefinitions...)
	}

	operationsOut, err := GenerateTypesForOperations(t, ops)
	if err != nil {
		return "", fmt.Errorf("error generating Go types for component request bodies: %w", err)
	}

	enumsOut, err := GenerateEnums(t, enumTypes)
	if err != nil {
		return "", fmt.Errorf("error generating code for type enums: %w", err)
	}

	typesOut, err := GenerateTypes(t, allTypes)
	if err != nil {
		return "", fmt.Errorf("error generating code for type definitions: %w", err)
	}

	allOfBoilerplate, err := GenerateAdditionalPropertyBoilerplate(t, allTypes)
	if err != nil {
		return "", fmt.Errorf("error generating allOf boilerplate: %w", err)
	}

	unionBoilerplate, err := GenerateUnionBoilerplate(t, allTypes)
	if err != nil {
		return "", fmt.Errorf("error generating union boilerplate: %w", err)
	}

	unionAndAdditionalBoilerplate, err := GenerateUnionAndAdditionalProopertiesBoilerplate(t, allTypes)
	if err != nil {
		return "", fmt.Errorf("error generating boilerplate for union types with additionalProperties: %w", err)
	}

	typeDefinitions := strings.Join([]string{enumsOut, typesOut, operationsOut, allOfBoilerplate, unionBoilerplate, unionAndAdditionalBoilerplate}, "")
	return typeDefinitions, nil
}

// GenerateTypes passes a bunch of types to the template engine, and buffers
// its output into a string.
func GenerateTypes(t *template.Template, types []TypeDefinition) (string, error) {
	ts, err := checkDuplicates(types)
	if err != nil {
		return "", err
	}

	context := struct {
		Types []TypeDefinition
	}{
		Types: ts,
	}

	return GenerateTemplates([]string{"typedef.tmpl"}, t, context)
}

func GenerateEnums(t *template.Template, types []TypeDefinition) (string, error) {
	enums := []EnumDefinition{}

	// Keep track of which enums we've generated
	m := map[string]bool{}

	// These are all types defined globally
	for _, tp := range types {
		if found := m[tp.TypeName]; found {
			continue
		}

		m[tp.TypeName] = true

		if len(tp.Schema.EnumValues) > 0 {
			wrapper := ""
			if tp.Schema.GoType == "string" {
				wrapper = `"`
			}
			enums = append(enums, EnumDefinition{
				Schema:         tp.Schema,
				TypeName:       tp.TypeName,
				ValueWrapper:   wrapper,
				PrefixTypeName: true,
			})
		}
	}

	// Now, go through all the enums, and figure out if we have conflicts with
	// any others.
	for i := range enums {
		// Look through all other enums not compared so far. Make sure we don't
		// compare against self.
		e1 := enums[i]
		for j := i + 1; j < len(enums); j++ {
			e2 := enums[j]

			for e1key := range e1.GetValues() {
				_, found := e2.GetValues()[e1key]
				if found {
					e1.PrefixTypeName = true
					e2.PrefixTypeName = true
					enums[i] = e1
					enums[j] = e2
					break
				}
			}
		}

		// now see if this enum conflicts with any global type names.
		for _, tp := range types {
			// Skip over enums, since we've handled those above.
			if len(tp.Schema.EnumValues) > 0 {
				continue
			}
			_, found := e1.Schema.EnumValues[tp.TypeName]
			if found {
				e1.PrefixTypeName = true
				enums[i] = e1
			}
		}

		// Another edge case is that an enum value can conflict with its own
		// type name.
		_, found := e1.GetValues()[e1.TypeName]
		if found {
			e1.PrefixTypeName = true
			enums[i] = e1
		}
	}

	// Now see if enums conflict with any non-enum typenames

	return GenerateTemplates([]string{"constants.tmpl"}, t, Constants{EnumDefinitions: enums})
}

// GenerateImports generates our import statements and package definition.
func GenerateImports(t *template.Template, externalImports []string, packageName string) (string, error) {
	// Read build version for incorporating into generated files
	// Unit tests have ok=false, so we'll just use "unknown" for the
	// version if we can't read this.

	modulePath := "unknown module path"
	moduleVersion := "unknown version"
	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Path != "" {
			modulePath = bi.Main.Path
		}
		if bi.Main.Version != "" {
			moduleVersion = bi.Main.Version
		}
	}

	context := struct {
		ExternalImports   []string
		PackageName       string
		ModuleName        string
		Version           string
		AdditionalImports []AdditionalImport
	}{
		ExternalImports:   externalImports,
		PackageName:       packageName,
		ModuleName:        modulePath,
		Version:           moduleVersion,
		AdditionalImports: globalState.options.AdditionalImports,
	}

	return GenerateTemplates([]string{"imports.tmpl"}, t, context)
}

// GenerateAdditionalPropertyBoilerplate generates all the glue code which provides
// the API for interacting with additional properties and JSON-ification
func GenerateAdditionalPropertyBoilerplate(t *template.Template, typeDefs []TypeDefinition) (string, error) {
	var filteredTypes []TypeDefinition

	m := map[string]bool{}

	for _, t := range typeDefs {
		if found := m[t.TypeName]; found {
			continue
		}

		m[t.TypeName] = true

		if t.Schema.HasAdditionalProperties {
			filteredTypes = append(filteredTypes, t)
		}
	}

	context := struct {
		Types []TypeDefinition
	}{
		Types: filteredTypes,
	}

	return GenerateTemplates([]string{"additional-properties.tmpl"}, t, context)
}

func GenerateUnionBoilerplate(t *template.Template, typeDefs []TypeDefinition) (string, error) {
	var filteredTypes []TypeDefinition
	for _, t := range typeDefs {
		if len(t.Schema.UnionElements) != 0 {
			filteredTypes = append(filteredTypes, t)
		}
	}

	if len(filteredTypes) == 0 {
		return "", nil
	}

	context := struct {
		Types []TypeDefinition
	}{
		Types: filteredTypes,
	}

	return GenerateTemplates([]string{"union.tmpl"}, t, context)
}

func GenerateUnionAndAdditionalProopertiesBoilerplate(t *template.Template, typeDefs []TypeDefinition) (string, error) {
	var filteredTypes []TypeDefinition
	for _, t := range typeDefs {
		if len(t.Schema.UnionElements) != 0 && t.Schema.HasAdditionalProperties {
			filteredTypes = append(filteredTypes, t)
		}
	}

	if len(filteredTypes) == 0 {
		return "", nil
	}
	context := struct {
		Types []TypeDefinition
	}{
		Types: filteredTypes,
	}

	return GenerateTemplates([]string{"union-and-additional-properties.tmpl"}, t, context)
}

// GetUserTemplateText attempts to retrieve the template text from a passed string or file..
func GetUserTemplateText(inputData string) (template string, err error) {
	// if the input data is more than one line, assume its a template and return that data.
	if strings.Contains(inputData, "\n") {
		return inputData, nil
	}

	// load data from file
	data, err := os.ReadFile(inputData)
	// return data if found and loaded
	if err == nil {
		return string(data), nil
	}

	// check for non "not found" errors
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to open file %s: %w", inputData, err)
	}

	return string(data), nil
}

// LoadTemplates loads all of our template files into a text/template. The
// path of template is relative to the templates directory.
func LoadTemplates(src embed.FS, t *template.Template) error {
	return fs.WalkDir(src, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking directory %s: %w", path, err)
		}
		if d.IsDir() {
			return nil
		}

		buf, err := src.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file '%s': %w", path, err)
		}

		templateName := strings.TrimPrefix(path, "templates/")
		tmpl := t.New(templateName)
		_, err = tmpl.Parse(string(buf))
		if err != nil {
			return fmt.Errorf("parsing template '%s': %w", path, err)
		}
		return nil
	})
}
