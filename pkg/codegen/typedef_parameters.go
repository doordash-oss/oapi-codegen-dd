package codegen

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/iancoleman/strcase"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

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

// JsonTag generates the JSON annotation to map GoType to json type name. If Parameter
// Foo is marshaled to json as "foo", this will create the annotation
// 'json:"foo"'
func (pd ParameterDefinition) JsonTag() string {
	if pd.Required {
		return fmt.Sprintf("`json:\"%s\"`", pd.ParamName)
	} else {
		return fmt.Sprintf("`json:\"%s,omitempty\"`", pd.ParamName)
	}
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
	if extension, ok := pd.Spec.Extensions.Get(extGoName); ok {
		if extGoFieldName, err := extParseGoFieldName(extension); err == nil {
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
func describeOperationParameters(params []*v3high.Parameter, path []string) ([]ParameterDefinition, error) {
	outParams := make([]ParameterDefinition, 0)
	for _, param := range params {
		schemaProxy := param.Schema
		ref := ""
		if schemaProxy != nil {
			ref = schemaProxy.GoLow().GetReference()
		}

		inSuffix := "Param"
		if param.In == "query" {
			inSuffix = "Query"
		} else if param.In == "path" {
			inSuffix = "Path"
		} else if param.In == "header" {
			inSuffix = "Header"
		}

		goSchema, err := paramToGoType(param, append(path, inSuffix, param.Name))
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

		// If this is a reference to a predefined type, simply use the reference
		// name as the type. $ref: "#/components/schemas/custom_type" becomes "CustomType".
		if ref != "" {
			goType, err := refPathToGoType(ref)
			if err != nil {
				return nil, fmt.Errorf("error dereferencing (%s) for param (%s): %s", ref, param.Name, err)
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
		} else {
			return nil, fmt.Errorf("duplicate local parameter %s/%s", p.In, p.ParamName)
		}
	}
	for _, p := range globalParams {
		if dupCheck[p.In] == nil {
			dupCheck[p.In] = make(map[string]string)
		}
		if t, exist := dupCheck[p.In][p.ParamName]; !exist {
			dupCheck[p.In][p.ParamName] = "global"
			allParams = append(allParams, p)
		} else if t == "global" {
			return nil, fmt.Errorf("duplicate global parameter %s/%s", p.In, p.ParamName)
		}
	}

	return allParams, nil
}

// generateParamsTypes defines the schema for a parameters definition object.
func generateParamsTypes(objectParams []ParameterDefinition, typeName string) ([]TypeDefinition, []GoSchema) {
	if len(objectParams) == 0 {
		return nil, nil
	}
	specLocation := SpecLocation(strings.ToLower(objectParams[0].In))

	var typeDefs []TypeDefinition
	var properties []Property
	var imports []GoSchema

	for _, param := range objectParams {
		pSchema := param.Schema
		if pSchema.HasAdditionalProperties {
			propRefName := strings.Join([]string{typeName, param.GoName()}, "_")
			pSchema.RefType = propRefName

			typeDefs = append(typeDefs, TypeDefinition{
				Name:         propRefName,
				Schema:       param.Schema,
				SpecLocation: specLocation,
			})
		}

		typeDefs = append(typeDefs, pSchema.AdditionalTypes...)
		exts := extractExtensions(param.Spec.Extensions)

		oapiSchemaProxy := param.Spec.Schema
		var oapiSchema *base.Schema
		if oapiSchemaProxy != nil {
			oapiSchema = oapiSchemaProxy.Schema()
		}

		properties = append(properties, Property{
			GoName:        createPropertyGoFieldName(param.ParamName, exts),
			Description:   param.Spec.Description,
			JsonFieldName: param.ParamName,
			Schema:        pSchema,
			Extensions:    exts,
			Constraints: newConstraints(oapiSchema, ConstraintsContext{
				name:     param.ParamName,
				required: param.Required,
			}),
		})
		imports = append(imports, pSchema)
	}

	s := GoSchema{
		Properties: properties,
	}
	fields := genFieldsFromProperties(properties)
	s.GoType = s.createGoStruct(fields)

	td := TypeDefinition{
		Name:         typeName,
		Schema:       s,
		SpecLocation: specLocation,
	}

	return append(typeDefs, td), imports
}

// This constructs a Go type for a parameter, looking at either the schema or
// the content, whichever is available
func paramToGoType(param *v3high.Parameter, path []string) (GoSchema, error) {
	if param.Content == nil && param.Schema == nil {
		return GoSchema{}, fmt.Errorf("parameter '%s' has no schema or content", param.Name)
	}

	ref := param.GoLow().GetReference()

	// We can process the schema through the generic schema processor
	if param.Schema != nil {
		return GenerateGoSchema(param.Schema, ref, path)
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
	return GenerateGoSchema(mediaType.Schema, mediaRef, path)
}
