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
	TypeTracker     *TypeTracker
}

type operationsCollection struct {
	operations     []OperationDefinition
	importSchemas  []GoSchema
	typeDefs       []TypeDefinition
	responseErrors []string
}

// Generate creates Go code from an OpenAPI document and a configuration in single file output.
func Generate(docContents []byte, cfg Configuration) (GeneratedCode, error) {
	cfg = cfg.WithDefaults()
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
	cfg = cfg.WithDefaults()

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
	cfg = cfg.WithDefaults()

	builtModel, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("error building model: %w", err)
	}
	model := &builtModel.Model

	if model == nil {
		return nil, nil
	}

	parseOptions := ParseOptions{
		OmitDescription:        cfg.Generate.OmitDescription,
		DefaultIntType:         cfg.Generate.DefaultIntType,
		AlwaysPrefixEnumValues: cfg.Generate.AlwaysPrefixEnumValues,
		SkipValidation:         cfg.Generate.Validation.Skip,
		typeTracker:            newTypeTracker(),
		visited:                map[string]bool{},
		model:                  model,
	}

	var (
		operations     []OperationDefinition
		importSchemas  []GoSchema
		responseErrors []string
	)

	// Process Components
	typeDefs, err := collectComponentDefinitions(model, parseOptions)
	if err != nil {
		return nil, fmt.Errorf("error collecting component definitions: %s", err)
	}

	// collect operations
	opColl, err := collectOperationDefinitions(model, parseOptions)
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

	enums, typeDefs := filterOutEnums(typeDefs, parseOptions)

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

	respErrs, err := collectResponseErrors(responseErrors, parseOptions.typeTracker)
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
		TypeTracker:     parseOptions.typeTracker,
	}, nil
}

func collectOperationDefinitions(model *v3high.Document, options ParseOptions) (*operationsCollection, error) {
	if model.Paths == nil || model.Paths.PathItems == nil {
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
		globalParams, err := describeOperationParameters(pathItem.Parameters, options.WithPath(nil))
		if err != nil {
			return nil, fmt.Errorf("error describing global parameters for %s: %s", path, err)
		}

		for method, operation := range pathItem.GetOperations().FromOldest() {
			var (
				headerDef     *TypeDefinition
				pathParamsDef *TypeDefinition
			)

			operationID, err := createOperationID(method, path, operation.OperationId)
			if err != nil {
				return nil, fmt.Errorf("error creating operation ID: %w", err)
			}

			// These are parameters defined for the specific path method that we're iterating over.
			localParams, err := describeOperationParameters(operation.Parameters, options.WithPath([]string{operationID}))
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
			reqParamsDef, pathDefs, pathSchemas := generateParamsTypes(pathParameters, operationID+"Path", options)
			if reqParamsDef != nil {
				pathParamsDef = &reqParamsDef.TypeDef
				typeDefs = append(typeDefs, pathDefs...)
				if len(pathSchemas) > 0 {
					importSchemas = append(importSchemas, pathSchemas...)
				}
			}

			queryParams := filterParameterDefinitionByType(allParams, "query")
			queryParamsDef, queryDefs, querySchemas := generateParamsTypes(queryParams, operationID+"Query", options)
			if queryParamsDef != nil {
				typeDefs = append(typeDefs, queryDefs...)
				if len(querySchemas) > 0 {
					importSchemas = append(importSchemas, querySchemas...)
				}
			}

			headerParams := filterParameterDefinitionByType(allParams, "header")
			headerParamsDef, headerDefs, headerSchemas := generateParamsTypes(headerParams, operationID+"Headers", options)
			if headerParamsDef != nil {
				headerDef = &headerParamsDef.TypeDef
				typeDefs = append(typeDefs, headerDefs...)
				if len(headerSchemas) > 0 {
					importSchemas = append(importSchemas, headerSchemas...)
				}
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
			responseDef, responseTypes, err := getOperationResponses(operationID, operation.Responses, options)
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
				Query:      queryParamsDef,
				Response:   response,
				Body:       bodyDefinition,
			})
		}
	}

	// Deduplicate operation IDs and resolve RequestOptions name collisions
	operations = deduplicateOperationIDs(operations)
	operations = resolveRequestOptionsCollisions(operations, options.typeTracker)

	allTypeDefs := extractAllTypeDefinitions(typeDefs)

	return &operationsCollection{
		operations:     operations,
		importSchemas:  importSchemas,
		typeDefs:       allTypeDefs,
		responseErrors: responseErrors,
	}, nil
}

// resolveRequestOptionsCollisions checks if any operation's RequestOptions type name
// would collide with existing component schemas, and renames the operation ID if needed.
func resolveRequestOptionsCollisions(operations []OperationDefinition, tracker *TypeTracker) []OperationDefinition {
	result := make([]OperationDefinition, len(operations))

	for i, op := range operations {
		if !op.HasRequestOptions() {
			result[i] = op
			continue
		}

		// Check if the RequestOptions type name would collide and get a unique base name
		op.ID = tracker.generateUniqueBaseName(UppercaseFirstCharacter(op.ID), "RequestOptions")

		result[i] = op
	}

	return result
}

// deduplicateOperationIDs ensures all operation IDs are unique by appending a suffix to duplicates
func deduplicateOperationIDs(operations []OperationDefinition) []OperationDefinition {
	seen := make(map[string]int) // map of operation ID to count
	result := make([]OperationDefinition, len(operations))

	for i, op := range operations {
		originalID := op.ID
		count, exists := seen[originalID]

		if exists {
			// This is a duplicate, append a suffix
			count++
			seen[originalID] = count
			op.ID = fmt.Sprintf("%s_%d", originalID, count)
		} else {
			seen[originalID] = 0
		}

		result[i] = op
	}

	return result
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
func collectResponseErrors(errNames []string, tracker *TypeTracker) ([]string, error) {
	if len(errNames) == 0 {
		return nil, nil
	}

	res := make([]string, 0, len(errNames))
	visited := make(map[string]bool, 8) // reuse across iterations

	for _, errName := range errNames {
		name := errName
		// Clear visited map for reuse
		for k := range visited {
			delete(visited, k)
		}

		for {
			if visited[name] {
				// Circular reference detected, use current name
				res = append(res, name)
				break
			}
			visited[name] = true

			typ, found := tracker.LookupByName(name)
			if !found {
				return nil, fmt.Errorf("error finding type '%s'", name)
			}
			if !typ.IsAlias() {
				res = append(res, name)
				break
			}
			newName := typ.Schema.RefType
			if newName == "" || newName == name {
				res = append(res, name)
				break
			}
			name = newName
		}
	}

	return res, nil
}
