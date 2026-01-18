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
	"strings"
	"unicode"

	"github.com/iancoleman/strcase"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type RequestParametersDefinition struct {
	Name     string
	Encoding map[string]ParameterEncoding
	Params   []ParameterDefinition
	TypeDef  TypeDefinition
}

// ParameterEncoding describes the encoding options for a request body.
// @see https://spec.openapis.org/oas/v3.1.0#style-examples
type ParameterEncoding struct {
	Style         string
	Explode       *bool
	Required      bool
	AllowReserved bool
}

// ParameterDefinition is a struct that represents a parameter in an operation.
// Name is the original json parameter name, eg param_name
// In is where the parameter is defined - path, header, cookie, query
// Required is whether the parameter is required
// Spec is the parsed openapi3.Parameter object
// GoSchema is the GoSchema object
type ParameterDefinition struct {
	ParamName string
	In        string
	Required  bool
	Spec      *v3high.Parameter
	Schema    GoSchema
}

// TypeDef is here as an adapter after a large refactoring so that I don't
// have to update all the templates. It returns the type definition for a parameter,
// without the leading '*' for optional ones.
func (pd ParameterDefinition) TypeDef() string {
	typeDecl := pd.Schema.TypeDecl()
	return typeDecl
}

func (pd ParameterDefinition) IsJson() bool {
	p := pd.Spec
	if p.Content.Len() == 1 {
		if isMediaTypeJson(p.Content.First().Key()) {
			return true
		}
	}
	return false
}

func (pd ParameterDefinition) IsPassThrough() bool {
	p := pd.Spec
	if p.Content.Len() > 1 {
		return true
	}
	if p.Content.Len() == 1 {
		return !pd.IsJson()
	}
	return false
}

func (pd ParameterDefinition) IsStyled() bool {
	p := pd.Spec
	return p.Schema != nil
}

func (pd ParameterDefinition) Explode() bool {
	if pd.Spec.Explode == nil {
		in := pd.Spec.In
		switch in {
		case "path", "header":
			return false
		case "query", "cookie":
			return true
		default:
			panic("unknown parameter format")
		}
	}
	return *pd.Spec.Explode
}

func (pd ParameterDefinition) GoVariableName() string {
	name := strcase.ToLowerCamel(pd.GoName())
	if isGoKeyword(name) {
		name = "p" + strcase.ToCamel(name)
	}
	if unicode.IsNumber([]rune(name)[0]) {
		name = "n" + name
	}
	return name
}

func (pd ParameterDefinition) GoName() string {
	goName := pd.ParamName
	exts := extractExtensions(pd.Spec.Extensions)
	if extension, ok := exts[extGoName]; ok {
		if extGoFieldName, err := parseString(extension); err == nil {
			goName = extGoFieldName
		}
	}
	return schemaNameToTypeName(goName)
}

func (pd ParameterDefinition) IndirectOptional() bool {
	return !pd.Required && !pd.Schema.SkipOptionalPointer
}

type ParameterDefinitions []ParameterDefinition

func (p ParameterDefinitions) FindByName(name string) *ParameterDefinition {
	for _, param := range p {
		if param.ParamName == name {
			return &param
		}
	}
	return nil
}

// describeOperationParameters walks the given parameters dictionary, and generates the above
// descriptors into a flat list. This makes it a lot easier to traverse the
// data in the template engine.
func describeOperationParameters(params []*v3high.Parameter, options ParseOptions) ([]ParameterDefinition, error) {
	outParams := make([]ParameterDefinition, 0)
	for _, param := range params {
		schemaProxy := param.Schema
		schemaRef := ""
		if schemaProxy != nil {
			schemaRef = schemaProxy.GoLow().GetReference()
		}

		// Check if the parameter itself is a reference to a component parameter
		paramRef := param.GoLow().GetReference()

		inSuffix := "Param"
		switch param.In {
		case "query":
			inSuffix = "Query"
		case "path":
			inSuffix = "Path"
		case "header":
			inSuffix = "Header"
		}

		goSchema, err := paramToGoType(param, options.WithPath(append(options.path, inSuffix, param.Name)))
		if err != nil {
			return nil, fmt.Errorf("error generating type for param (%s): %s", param.Name, err)
		}

		required := false
		if param.Required != nil {
			required = *param.Required
		}

		pd := ParameterDefinition{
			ParamName: param.Name,
			In:        param.In,
			Required:  required,
			Spec:      param,
			Schema:    goSchema,
		}

		// If the parameter references a component parameter, use the registered type name
		if paramRef != "" && strings.HasPrefix(paramRef, "#/components/parameters/") {
			if registeredName, found := options.typeTracker.LookupByRef(paramRef); found {
				pd.Schema.GoType = registeredName
			}
		} else if schemaRef != "" && isStandardComponentReference(schemaRef) {
			// If this is a reference to a predefined schema type, simply use the reference
			// name as the type. $ref: "#/components/schemas/custom_type" becomes "CustomType".
			// However, for deep path references (e.g., #/paths/.../parameters/1/schema),
			// GenerateGoSchema has already created the type definition, so we don't override it.
			goType, err := refPathToGoType(schemaRef)
			if err != nil {
				return nil, fmt.Errorf("error dereferencing (%s) for param (%s): %s", schemaRef, param.Name, err)
			}
			pd.Schema.GoType = goType
		}
		outParams = append(outParams, pd)
	}
	return outParams, nil
}

// combineOperationParameters combines the Parameters defined at a global level (Parameters defined for all methods on a given path) with the Parameters defined at a local level (Parameters defined for a specific path), preferring the locally defined parameter over the global one
func combineOperationParameters(globalParams []ParameterDefinition, localParams []ParameterDefinition) ([]ParameterDefinition, error) {
	allParams := make([]ParameterDefinition, 0, len(globalParams)+len(localParams))
	dupCheck := make(map[string]map[string]string)

	for _, p := range localParams {
		if dupCheck[p.In] == nil {
			dupCheck[p.In] = make(map[string]string)
		}
		if _, exist := dupCheck[p.In][p.ParamName]; !exist {
			dupCheck[p.In][p.ParamName] = "local"
			allParams = append(allParams, p)
		}
	}

	for _, p := range globalParams {
		if dupCheck[p.In] == nil {
			dupCheck[p.In] = make(map[string]string)
		}
		if _, exist := dupCheck[p.In][p.ParamName]; !exist {
			dupCheck[p.In][p.ParamName] = "global"
			allParams = append(allParams, p)
		}
		// Duplicate global parameter - skip (first wins)
		// Consistent with duplicate local parameter handling
	}

	return allParams, nil
}

// generateParamsTypes defines the schema for a parameters definition object.
func generateParamsTypes(objectParams []ParameterDefinition, typeName string, options ParseOptions) (*RequestParametersDefinition, []TypeDefinition, []GoSchema) {
	if len(objectParams) == 0 {
		return nil, nil, nil
	}
	specLocation := SpecLocation(strings.ToLower(objectParams[0].In))

	var (
		typeDefs   []TypeDefinition
		properties []Property
		imports    []GoSchema
	)

	encodings := map[string]ParameterEncoding{}
	goFieldNames := make(map[string]int) // Track Go field names to detect conflicts

	for _, param := range objectParams {
		pSchema := param.Schema
		if pSchema.HasAdditionalProperties {
			propRefName := strings.Join([]string{typeName, param.GoName()}, "_")
			pSchema.RefType = propRefName

			td := TypeDefinition{
				Name:           propRefName,
				Schema:         param.Schema,
				SpecLocation:   specLocation,
				NeedsMarshaler: needsMarshaler(param.Schema),
			}
			typeDefs = append(typeDefs, td)
			options.typeTracker.register(td, "")
		}

		typeDefs = append(typeDefs, pSchema.AdditionalTypes...)
		exts := extractExtensions(param.Spec.Extensions)

		oapiSchemaProxy := param.Spec.Schema
		var oapiSchema *base.Schema
		if oapiSchemaProxy != nil {
			oapiSchema = oapiSchemaProxy.Schema()
		}

		// Generate the Go field name and handle conflicts
		baseGoName := createPropertyGoFieldName(param.ParamName, exts)
		goName := baseGoName
		if count, exists := goFieldNames[baseGoName]; exists {
			// Conflict detected - append a number
			count++
			goFieldNames[baseGoName] = count
			goName = fmt.Sprintf("%s%d", baseGoName, count)
		} else {
			goFieldNames[baseGoName] = 0
		}

		properties = append(properties, Property{
			GoName:        goName,
			Description:   param.Spec.Description,
			JsonFieldName: param.ParamName,
			Schema:        pSchema,
			Extensions:    exts,
			Constraints: newConstraints(oapiSchema, ConstraintsContext{
				required:     param.Required,
				specLocation: specLocation,
			}),
		})
		imports = append(imports, pSchema)
		encodings[param.ParamName] = ParameterEncoding{
			Style:         param.Spec.Style,
			Explode:       param.Spec.Explode,
			Required:      param.Required,
			AllowReserved: param.Spec.AllowReserved,
		}
	}

	s := GoSchema{
		Properties: properties,
	}
	fields := genFieldsFromProperties(properties, options)
	s.GoType = s.createGoStruct(fields)

	td := TypeDefinition{
		Name:         typeName,
		Schema:       s,
		SpecLocation: specLocation,
	}
	options.typeTracker.register(td, "")

	res := &RequestParametersDefinition{
		Name:     typeName,
		Encoding: encodings,
		Params:   objectParams,
		TypeDef:  td,
	}

	return res, append(typeDefs, td), imports
}

// This constructs a Go type for a parameter, looking at either the schema or
// the content, whichever is available
func paramToGoType(param *v3high.Parameter, options ParseOptions) (GoSchema, error) {
	if param.Content == nil && param.Schema == nil {
		return GoSchema{}, fmt.Errorf("parameter '%s' has no schema or content", param.Name)
	}

	ref := param.GoLow().GetReference()
	options = options.WithReference(ref)

	// We can process the schema through the generic schema processor
	if param.Schema != nil {
		return GenerateGoSchema(param.Schema, options)
	}

	// At this point, we have a content type. We know how to deal with
	// application/json, but if multiple formats are present, we can't do anything,
	// so we'll return the parameter as a string, not bothering to decode it.
	if param.Content.Len() > 1 {
		return GoSchema{
			GoType:      "string",
			Description: stringToGoComment(param.Description),
		}, nil
	}

	// Otherwise, look for application/json in there
	mediaType, found := param.Content.Get("application/json")
	if !found {
		// If we don't have json, it's a string
		return GoSchema{
			GoType:      "string",
			Description: stringToGoComment(param.Description),
		}, nil
	}

	mediaRef := mediaType.GoLow().GetReference()
	// For json, we go through the standard schema mechanism
	return GenerateGoSchema(mediaType.Schema, options.WithReference(mediaRef))
}
