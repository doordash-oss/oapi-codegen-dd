package codegen

import (
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// ParseContext holds the OpenAPI models.
type ParseContext struct {
	Operations      []OperationDefinition
	TypeDefinitions map[SpecLocation][]TypeDefinition
	Enums           []EnumDefinition
	UnionTypes      []TypeDefinition
	Imports         []string
	ResponseErrors  []string
	TypeRegistry    TypeRegistry
}

type operationsCollection struct {
	operations     []OperationDefinition
	importSchemas  []GoSchema
	typeDefs       []TypeDefinition
	responseErrors []string
}

// Generate creates Go code from an OpenAPI document and a configuration in single file output.
func Generate(docContents []byte, cfg Configuration) (GeneratedCode, error) {
	cfg = cfg.Merge(NewDefaultConfiguration())
	parseCtx, errs := CreateParseContext(docContents, cfg)
	if errs != nil {
		return nil, fmt.Errorf("error creating parse context: %w", errs[0])
	}
	if parseCtx == nil {
		return nil, ErrEmptySchema
	}

	parser, err := NewParser(cfg, parseCtx)
	if err != nil {
		return nil, fmt.Errorf("error creating parser: %w", err)
	}

	return parser.Parse()
}

// CreateParseContext creates a ParseContext from an OpenAPI contents and a ParseConfig.
func CreateParseContext(docContents []byte, cfg Configuration) (*ParseContext, []error) {
	cfg = cfg.Merge(NewDefaultConfiguration())

	doc, err := CreateDocument(docContents, cfg)
	if err != nil {
		return nil, []error{fmt.Errorf("error filtering document: %w", err)}
	}

	res, err := CreateParseContextFromDocument(doc, cfg)
	if err != nil {
		return nil, []error{err}
	}

	return res, nil
}

func CreateParseContextFromDocument(doc libopenapi.Document, cfg Configuration) (*ParseContext, error) {
	cfg = cfg.Merge(NewDefaultConfiguration())
	parseOptions := ParseOptions{
		OmitDescription: cfg.Generate.OmitDescription,
		DefaultIntType:  cfg.Generate.DefaultIntType,
	}

	builtModel, errs := doc.BuildV3Model()
	if len(errs) > 0 {
		return nil, errs[0]
	}
	model := &builtModel.Model

	var (
		operations     []OperationDefinition
		importSchemas  []GoSchema
		responseErrors []string
	)

	if model == nil {
		return nil, nil
	}

	// Process Components
	typeDefs, err := collectComponentDefinitions(model, parseOptions)
	if err != nil {
		return nil, fmt.Errorf("error collecting component definitions: %s", err)
	}

	// group current type definitions by name for easy lookup.
	currentTypes := make(map[string]TypeDefinition)
	for _, comp := range typeDefs {
		currentTypes[comp.Name] = comp
	}

	// collect operations
	opColl, err := collectOperationDefinitions(model, currentTypes, parseOptions)
	if err != nil {
		return nil, fmt.Errorf("error collecting operation definitions: %w", err)
	}

	if opColl != nil {
		operations = opColl.operations
		importSchemas = opColl.importSchemas
		typeDefs = append(typeDefs, opColl.typeDefs...)
		responseErrors = opColl.responseErrors
	}

	// Collect Schemas from components
	for _, componentDef := range typeDefs {
		importSchemas = append(importSchemas, componentDef.Schema)
	}

	// Collect Imports
	imprts := map[string]goImport{}
	for _, schema := range importSchemas {
		importRes, err := collectSchemaImports(schema)
		if err != nil {
			return nil, fmt.Errorf("error getting schema imports: %w", err)
		}
		mergeImports(imprts, importRes)
	}

	enums, typeDefs, registry := filterOutEnums(typeDefs)

	groupedTypeDefs := make(map[SpecLocation][]TypeDefinition)
	var unionTypes []TypeDefinition

	for _, td := range typeDefs {
		collected := false

		if len(td.Schema.UnionElements) != 0 {
			td.SpecLocation = SpecLocationUnion
			unionTypes = append(unionTypes, td)
			collected = true
		} else if td.SpecLocation == SpecLocationUnion {
			unionTypes = append(unionTypes, td)
			collected = true
		}

		if !collected && td.Name != "" {
			specLocation := td.SpecLocation
			if specLocation == "" {
				specLocation = SpecLocationSchema
			}
			if _, found := groupedTypeDefs[specLocation]; !found {
				groupedTypeDefs[specLocation] = []TypeDefinition{}
			}
			groupedTypeDefs[specLocation] = append(groupedTypeDefs[specLocation], td)
		}
	}

	respErrs, err := collectResponseErrors(responseErrors, typeDefs)
	if err != nil {
		return nil, fmt.Errorf("error collecting response errors: %w", err)
	}

	return &ParseContext{
		Operations:      operations,
		TypeDefinitions: groupedTypeDefs,
		Enums:           enums,
		UnionTypes:      unionTypes,
		Imports:         importMap(imprts).GoImports(),
		ResponseErrors:  respErrs,
		TypeRegistry:    registry,
	}, nil
}

func collectOperationDefinitions(model *v3high.Document, currentTypes map[string]TypeDefinition, options ParseOptions) (*operationsCollection, error) {
	if model.Paths == nil {
		return nil, nil
	}

	var (
		operations     []OperationDefinition
		importSchemas  []GoSchema
		typeDefs       []TypeDefinition
		responseErrors []string
	)

	for path, pathItem := range model.Paths.PathItems.FromOldest() {
		// These are parameters defined for all methods on a given path. They
		// are shared by all methods.
		globalParams, err := describeOperationParameters(pathItem.Parameters, nil, options)
		if err != nil {
			return nil, fmt.Errorf("error describing global parameters for %s: %s", path, err)
		}

		for method, operation := range pathItem.GetOperations().FromOldest() {
			var (
				headerDef     *TypeDefinition
				queryDef      *TypeDefinition
				pathParamsDef *TypeDefinition
			)

			operationID, err := createOperationID(method, path, operation.OperationId)
			if err != nil {
				return nil, fmt.Errorf("error creating operation ID: %w", err)
			}

			// These are parameters defined for the specific path method that we're iterating over.
			localParams, err := describeOperationParameters(operation.Parameters, []string{operationID}, options)
			if err != nil {
				return nil, fmt.Errorf("error describing local parameters for %s/%s: %s", method, path, err)
			}

			// All the parameters required by a handler are the union of the
			// global parameters and the local parameters.
			allParams, err := combineOperationParameters(globalParams, localParams)
			if err != nil {
				return nil, err
			}
			for _, param := range allParams {
				importSchemas = append(importSchemas, param.Schema)
			}

			// Order the path parameters to match the order as specified in
			// the path, not in the openapi spec, and validate that the parameter
			// names match, as downstream code depends on that.
			pathParameters := filterParameterDefinitionByType(allParams, "path")
			pathDefs, pathSchemas := generateParamsTypes(pathParameters, operationID+"Path", options)
			if len(pathDefs) > 0 {
				pathParamsDef = &pathDefs[len(pathDefs)-1]
				typeDefs = append(typeDefs, pathDefs...)
			}
			if len(pathSchemas) > 0 {
				importSchemas = append(importSchemas, pathSchemas...)
			}

			queryParams := filterParameterDefinitionByType(allParams, "query")
			queryDefs, querySchemas := generateParamsTypes(queryParams, operationID+"Query", options)
			if len(queryDefs) > 0 {
				queryDef = &queryDefs[len(queryDefs)-1]
				typeDefs = append(typeDefs, queryDefs...)
			}
			if len(querySchemas) > 0 {
				importSchemas = append(importSchemas, querySchemas...)
			}

			headerParams := filterParameterDefinitionByType(allParams, "header")
			headerDefs, headerSchemas := generateParamsTypes(headerParams, operationID+"Headers", options)
			if len(headerDefs) > 0 {
				headerDef = &headerDefs[len(headerDefs)-1]
				typeDefs = append(typeDefs, headerDefs...)
			}
			if len(headerSchemas) > 0 {
				importSchemas = append(importSchemas, headerSchemas...)
			}

			// Process Request Body
			bodyDefinition, bodyTypeDef, err := createBodyDefinition(operationID, operation.RequestBody, options)
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
			response := ResponseDefinition{}
			responseDef, responseTypes, err := getOperationResponses(operationID, operation.Responses, currentTypes, options)
			if err != nil {
				return nil, fmt.Errorf("error getting operation responses: %w", err)
			}
			if responseTypes != nil {
				typeDefs = append(typeDefs, responseTypes...)
				for _, responseType := range responseTypes {
					importSchemas = append(importSchemas, responseType.Schema)
				}
			}
			if responseDef != nil {
				response = *responseDef
				if responseDef.Error != nil {
					responseErrors = append(responseErrors, responseDef.Error.ResponseName)
				}
			}

			operations = append(operations, OperationDefinition{
				ID:          operationID,
				Summary:     operation.Summary,
				Description: operation.Description,
				// https://datatracker.ietf.org/doc/html/rfc7231
				Method:     strings.ToUpper(method),
				Path:       path,
				PathParams: pathParamsDef,
				Header:     headerDef,
				Query:      queryDef,
				Response:   response,
				Body:       bodyDefinition,
			})
		}
	}

	allTypeDefs := extractAllTypeDefinitions(typeDefs)

	return &operationsCollection{
		operations:     operations,
		importSchemas:  importSchemas,
		typeDefs:       allTypeDefs,
		responseErrors: responseErrors,
	}, nil
}

// collectComponentDefinitions collects all the components from the model and returns them as a list of TypeDefinition.
func collectComponentDefinitions(model *v3high.Document, options ParseOptions) ([]TypeDefinition, error) {
	if model.Components == nil {
		return nil, nil
	}

	var typeDefs []TypeDefinition

	// Parameters
	if model.Components.Parameters != nil {
		res, err := getComponentParameters(model.Components.Parameters, options)
		if err != nil {
			return nil, err
		}
		typeDefs = append(typeDefs, res...)
	}

	// Schemas
	if model.Components.Schemas != nil {
		schemas, err := getComponentsSchemas(model.Components.Schemas, options)
		if err != nil {
			return nil, fmt.Errorf("error getting components schemas: %w", err)
		}
		typeDefs = append(typeDefs, schemas...)
	}

	// RequestBodies
	if model.Components.RequestBodies != nil {
		bodyTypes, err := getComponentsRequestBodies(model.Components.RequestBodies, options)
		if err != nil {
			return nil, fmt.Errorf("error getting components request bodies: %w", err)
		}
		typeDefs = append(typeDefs, bodyTypes...)
	}

	// Responses
	if model.Components.Responses != nil {
		componentResponses, err := getComponentResponses(model.Components.Responses, options)
		if err != nil {
			return nil, fmt.Errorf("error getting content responses: %w", err)
		}
		typeDefs = append(typeDefs, componentResponses...)
	}

	all := extractAllTypeDefinitions(typeDefs)
	return all, nil
}

func extractAllTypeDefinitions(types []TypeDefinition) []TypeDefinition {
	var res []TypeDefinition
	for _, typeDef := range types {
		res = append(res, typeDef)
		res = append(res, extractAllTypeDefinitions(typeDef.Schema.AdditionalTypes)...)
	}
	return res
}

// collectResponseErrors collects the response errors from the type definitions.
// We need non-alias types for the response errors, so we could generate Error function.
func collectResponseErrors(errNames []string, types []TypeDefinition) ([]string, error) {
	tds := make(map[string]TypeDefinition)
	for _, typeDef := range types {
		tds[typeDef.Name] = typeDef
	}

	var res []string
	for _, errName := range errNames {
		name := errName
		for {
			typ, found := tds[name]
			if !found {
				return nil, fmt.Errorf("error finding type '%s'", name)
			}
			if !typ.IsAlias() {
				res = append(res, name)
				break
			}
			name = typ.Schema.RefType
			if name == "" {
				break
			}
		}
	}

	return res, nil
}
