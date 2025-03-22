package codegen

import (
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
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

// CreateParseContext creates a ParseContext from an OpenAPI file and a ParseConfig.
func CreateParseContext(file string, cfg *Configuration) (*ParseContext, []error) {
	if cfg == nil {
		cfg = NewDefaultConfiguration()
	}

	contents, err := os.ReadFile(file)
	if err != nil {
		return nil, []error{err}
	}

	doc, err := openapi3.NewLoader().LoadFromData(contents)
	if err != nil {
		return nil, []error{err}
	}

	res, err := createParseContextFromDocument(doc, cfg)
	if err != nil {
		return nil, []error{err}
	}

	return res, nil
}

// Generate uses the Go templating engine to generate all of our server wrappers from
// the descriptions we've built up above from the schema objects.
func Generate(spec *openapi3.T, cfg *Configuration) (string, error) {
	parseCtx, err := createParseContextFromDocument(spec, cfg)
	if err != nil {
		return "", fmt.Errorf("error creating parse context: %w", err)
	}

	parser, err := NewParser(cfg, parseCtx)
	if err != nil {
		return "", fmt.Errorf("error creating parser: %w", err)
	}

	codes, err := parser.Parse()
	if err != nil {
		return "", fmt.Errorf("error parsing: %w", err)
	}

	// original behavior is single file output
	return FormatCode(codes["all"]), nil
}

func createParseContextFromDocument(doc *openapi3.T, cfg *Configuration) (*ParseContext, error) {
	doc, err := filterDocument(doc, cfg)
	if err != nil {
		return nil, fmt.Errorf("error filtering document: %w", err)
	}

	var (
		operations    []OperationDefinition
		importSchemas []GoSchema
		typeDefs      []TypeDefinition
	)

	if doc == nil || doc.Paths == nil {
		return nil, nil
	}

	for _, requestPath := range SortedMapKeys(doc.Paths.Map()) {
		pathItem := doc.Paths.Value(requestPath)
		// These are parameters defined for all methods on a given path. They
		// are shared by all methods.
		globalParams, err := DescribeParameters(pathItem.Parameters, nil)
		if err != nil {
			return nil, fmt.Errorf("error describing global parameters for %s: %s",
				requestPath, err)
		}

		// Each path can have a number of operations, POST, GET, OPTIONS, etc.
		pathOps := pathItem.Operations()
		for _, method := range SortedMapKeys(pathOps) {
			var (
				headerDef     *TypeDefinition
				queryDef      *TypeDefinition
				pathParamsDef *TypeDefinition
			)

			op := pathOps[method]
			operationID, err := createOperationID(method, requestPath, op.OperationID)
			if err != nil {
				return nil, fmt.Errorf("error creating operation ID: %w", err)
			}

			// These are parameters defined for the specific path method that we're iterating over.
			localParams, err := DescribeParameters(op.Parameters, []string{op.OperationID + "Params"})
			if err != nil {
				return nil, fmt.Errorf("error describing global parameters for %s/%s: %s",
					method, requestPath, err)
			}
			// All the parameters required by a handler are the union of the
			// global parameters and the local parameters.
			allParams, err := CombineOperationParameters(globalParams, localParams)
			if err != nil {
				return nil, err
			}
			for _, param := range allParams {
				importSchemas = append(importSchemas, param.Schema)
			}

			// Order the path parameters to match the order as specified in
			// the path, not in the openapi spec, and validate that the parameter
			// names match, as downstream code depends on that.
			pathParams := FilterParameterDefinitionByType(allParams, "path")
			pathParams, err = SortParamsByPath(requestPath, pathParams)
			if err != nil {
				return nil, err
			}
			pathDefs, pathSchemas := generateParamsTypes(pathParams, operationID+"Path")
			if len(pathDefs) > 0 {
				pathParamsDef = &pathDefs[0]
				typeDefs = append(typeDefs, pathDefs...)
			}
			if len(pathSchemas) > 0 {
				importSchemas = append(importSchemas, pathSchemas...)
			}

			queryParams := FilterParameterDefinitionByType(allParams, "query")
			queryDefs, querySchemas := generateParamsTypes(queryParams, operationID+"Query")
			if len(queryDefs) > 0 {
				queryDef = &queryDefs[0]
				typeDefs = append(typeDefs, queryDefs...)
			}
			if len(querySchemas) > 0 {
				importSchemas = append(importSchemas, querySchemas...)
			}

			headerParams := FilterParameterDefinitionByType(allParams, "header")
			headerDefs, headerSchemas := generateParamsTypes(headerParams, operationID+"Headers")
			if len(headerDefs) > 0 {
				headerDef = &headerDefs[0]
				typeDefs = append(typeDefs, headerDefs...)
			}
			if len(headerSchemas) > 0 {
				importSchemas = append(importSchemas, headerSchemas...)
			}

			// Process Request Body
			bodyDefinition, bodyTypeDef, err := createBodyDefinition(operationID, op.RequestBody)
			if err != nil {
				return nil, fmt.Errorf("error generating body definitions: %w", err)
			}
			if bodyTypeDef != nil {
				typeDefs = append(typeDefs, *bodyTypeDef)
				importSchemas = append(importSchemas, bodyTypeDef.Schema)
			}
			if bodyDefinition != nil {
				typeDefs = append(typeDefs, bodyDefinition.Schema.AdditionalTypes...)
			}

			// Process Responses
			responseDef, responseTypes, err := getOperationResponses(operationID, op.Responses.Map())
			if err != nil {
				return nil, fmt.Errorf("error getting operation responses: %w", err)
			}
			typeDefs = append(typeDefs, responseTypes...)
			for _, responseType := range responseTypes {
				importSchemas = append(importSchemas, responseType.Schema)
			}

			opDef := OperationDefinition{
				ID:          operationID,
				Summary:     op.Summary,
				Description: op.Description,
				Method:      method,
				Path:        requestPath,
				PathParams:  pathParamsDef,
				Header:      headerDef,
				Query:       queryDef,
				Response:    *responseDef,
				Body:        bodyDefinition,
			}

			if op.RequestBody != nil {
				opDef.BodyRequired = op.RequestBody.Value.Required
			}

			operations = append(operations, opDef)
		}
	}

	// Process Components
	componentDefs, err := collectComponentDefinitions(doc)
	if err != nil {
		return nil, fmt.Errorf("error collecting component definitions: %s", err)
	}
	typeDefs = append(typeDefs, componentDefs...)

	// Collect Schemas from components
	for _, componentDef := range componentDefs {
		importSchemas = append(importSchemas, componentDef.Schema)
	}

	// Collect Imports
	imprts := map[string]goImport{}
	for _, schema := range importSchemas {
		importRes, err := collectSchemaImports(schema)
		if err != nil {
			return nil, fmt.Errorf("error getting schema imports: %w", err)
		}
		MergeImports(imprts, importRes)
	}

	typeDefs, err = checkDuplicates(typeDefs)
	if err != nil {
		return nil, fmt.Errorf("error checking for duplicate type definitions: %w", err)
	}

	enums, typeDefs := filterOutEnums(typeDefs)

	groupedTypeDefs := make(map[SpecLocation][]TypeDefinition)
	var (
		additionalTypes          []TypeDefinition
		unionTypes               []TypeDefinition
		unionWithAdditionalTypes []TypeDefinition
	)

	for _, td := range typeDefs {
		collected := false
		if td.Schema.HasAdditionalProperties {
			additionalTypes = append(additionalTypes, td)
			collected = true
		}

		if len(td.Schema.UnionElements) != 0 {
			td.SpecLocation = SpecLocationUnion
			unionTypes = append(unionTypes, td)
			collected = true
		} else if td.SpecLocation == SpecLocationUnion {
			unionTypes = append(unionTypes, td)
			collected = true
		}

		if len(additionalTypes) != 0 && len(unionTypes) != 0 {
			unionWithAdditionalTypes = append(unionWithAdditionalTypes, td)
			collected = true
		}

		if !collected && td.Name != "" && td.SpecLocation != "" {
			if _, found := groupedTypeDefs[td.SpecLocation]; !found {
				groupedTypeDefs[td.SpecLocation] = []TypeDefinition{}
			}
			groupedTypeDefs[td.SpecLocation] = append(groupedTypeDefs[td.SpecLocation], td)
		}
	}

	return &ParseContext{
		Operations:               operations,
		TypeDefinitions:          groupedTypeDefs,
		Enums:                    enums,
		UnionTypes:               unionTypes,
		AdditionalTypes:          additionalTypes,
		UnionWithAdditionalTypes: unionWithAdditionalTypes,
		Imports:                  importMap(imprts).GoImports(),
	}, nil
}

// collectComponentDefinitions collects all the components from the model and returns them as a list of TypeDefinition.
func collectComponentDefinitions(model *openapi3.T) ([]TypeDefinition, error) {
	if model.Components == nil {
		return nil, nil
	}

	var typeDefs []TypeDefinition
	schemaTypes, err := getComponentsSchemas(model.Components.Schemas)
	if err != nil {
		return nil, fmt.Errorf("error generating Go types for component schemas: %w", err)
	}

	paramTypes, err := getComponentParameters(model.Components.Parameters)
	if err != nil {
		return nil, fmt.Errorf("error generating Go types for component parameters: %w", err)
	}
	typeDefs = append(schemaTypes, paramTypes...)

	responseTypes, err := getContentResponses(model.Components.Responses)
	if err != nil {
		return nil, fmt.Errorf("error generating Go types for component responses: %w", err)
	}
	typeDefs = append(typeDefs, responseTypes...)

	bodyTypes, err := getComponentsRequestBodies(model.Components.RequestBodies)
	if err != nil {
		return nil, fmt.Errorf("error generating Go types for component request bodies: %w", err)
	}
	typeDefs = append(typeDefs, bodyTypes...)

	return typeDefs, nil
}
