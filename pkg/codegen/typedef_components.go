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

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

// getComponentsSchemas generates type definitions for any custom types defined in the
// components/schemas section of the Swagger spec.
func getComponentsSchemas(schemas *orderedmap.Map[string, *base.SchemaProxy], options ParseOptions) ([]TypeDefinition, error) {
	types := make([]TypeDefinition, 0)

	// We're going to define Go types for every object under components/schemas
	for schemaName, schemaRef := range schemas.FromOldest() {
		ref := schemaRef.GoLow().GetReference()
		opts := options.WithReference(ref).WithPath([]string{schemaName})
		goSchema, err := GenerateGoSchema(schemaRef, opts)
		if err != nil {
			return nil, fmt.Errorf("error converting GoSchema %s to Go type: %w", schemaName, err)
		}
		if goSchema.IsZero() {
			continue
		}

		goTypeName, err := renameComponent(schemaNameToTypeName(schemaName), schemaRef)
		if err != nil {
			return nil, fmt.Errorf("error making name for components/schemas/%s: %w", schemaName, err)
		}

		td := TypeDefinition{
			JsonName:         schemaName,
			Name:             goTypeName,
			Schema:           goSchema,
			SpecLocation:     SpecLocationSchema,
			NeedsMarshaler:   needsMarshaler(goSchema),
			HasSensitiveData: hasSensitiveData(goSchema),
		}
		types = append(types, td)
		opts.AddType(td)

		types = append(types, goSchema.AdditionalTypes...)
	}

	return types, nil
}

// getComponentParameters generates type definitions for any custom types defined in the
// components/parameters section of the Swagger spec.
func getComponentParameters(params *orderedmap.Map[string, *v3high.Parameter], options ParseOptions) ([]TypeDefinition, error) {
	var types []TypeDefinition
	for paramName, paramOrRef := range params.FromOldest() {
		goType, err := paramToGoType(paramOrRef, options.WithPath([]string{paramName}))
		if err != nil {
			return nil, fmt.Errorf("error generating Go type for schema in parameter %s: %w", paramName, err)
		}

		goTypeName, err := renameParameter(paramName, paramOrRef)
		if err != nil {
			return nil, fmt.Errorf("error making name for components/parameters/%s: %w", paramName, err)
		}

		typeDef := TypeDefinition{
			JsonName:         paramName,
			Schema:           goType,
			Name:             goTypeName,
			SpecLocation:     SpecLocation(strings.ToLower(paramOrRef.In)),
			NeedsMarshaler:   needsMarshaler(goType),
			HasSensitiveData: hasSensitiveData(goType),
		}
		options.AddType(typeDef)

		ref := ""
		if paramOrRef.Schema != nil {
			ref = paramOrRef.Schema.GoLow().GetReference()
		}

		if ref != "" {
			// Generate a reference type for referenced parameters
			refType, err := refPathToGoType(ref)
			if err != nil {
				return nil, fmt.Errorf("error generating Go type for (%s) in parameter %s: %w", ref, paramName, err)
			}
			typeDef.Name = schemaNameToTypeName(refType)
		}

		types = append(types, typeDef)
	}
	return types, nil
}

// getComponentsRequestBodies generates type definitions for any custom types defined in the
// components/requestBodies section of the Swagger spec.
func getComponentsRequestBodies(bodies *orderedmap.Map[string, *v3high.RequestBody], options ParseOptions) ([]TypeDefinition, error) {
	var types []TypeDefinition

	for requestBodyName, requestBodyRef := range bodies.FromOldest() {
		// As for responses, we will only generate Go code for JSON bodies,
		// the other body formats are up to the user.
		response := requestBodyRef
		for mediaType, body := range response.Content.FromOldest() {
			if !isMediaTypeJson(mediaType) {
				continue
			}

			ref := response.GoLow().GetReference()
			opts := options.WithReference(ref).WithPath([]string{requestBodyName})
			goType, err := GenerateGoSchema(body.Schema, opts)
			if err != nil {
				return nil, fmt.Errorf("error generating Go type for schema in body %s: %w", requestBodyName, err)
			}
			if goType.IsZero() {
				continue
			}

			goTypeName, err := renameComponent(schemaNameToTypeName(requestBodyName), body.Schema)
			if err != nil {
				return nil, fmt.Errorf("error making name for components/schemas/%s: %w", requestBodyName, err)
			}

			typeDef := TypeDefinition{
				JsonName:       requestBodyName,
				Schema:         goType,
				Name:           goTypeName,
				SpecLocation:   SpecLocationBody,
				NeedsMarshaler: needsMarshaler(goType),
			}
			options.AddType(typeDef)

			bodyRef := ""
			if body.Schema != nil {
				bodyRef = body.Schema.GoLow().GetReference()
			}
			if bodyRef != "" {
				// Generate a reference type for referenced bodies
				refType, err := refPathToGoType(bodyRef)
				if err != nil {
					return nil, fmt.Errorf("error generating Go type for (%s) in body %s: %w", bodyRef, requestBodyName, err)
				}
				typeDef.Name = schemaNameToTypeName(refType)
			}
			types = append(types, typeDef)
		}
	}
	return types, nil
}

// getComponentResponses generates type definitions for any custom types defined in the
// components/responses section of the OpenAPI spec.
func getComponentResponses(responses *orderedmap.Map[string, *v3high.Response], options ParseOptions) ([]TypeDefinition, error) {
	var types []TypeDefinition

	for responseName, response := range responses.FromOldest() {
		// We have to generate the response object.
		// We're only going to handle media types that conform to JSON.
		for mediaType, content := range response.Content.FromOldest() {
			if !isMediaTypeJson(mediaType) {
				continue
			}

			ref := content.GoLow().GetReference()
			opts := options.WithReference(ref).WithPath([]string{responseName})
			goType, err := GenerateGoSchema(content.Schema, opts)
			if err != nil {
				return nil, fmt.Errorf("error generating Go type for schema in response %s: %w", responseName, err)
			}

			goTypeName, err := renameComponent(schemaNameToTypeName(responseName), content.Schema)
			if err != nil {
				return nil, fmt.Errorf("error making name for components/responses/%s: %w", responseName, err)
			}

			typeDef := TypeDefinition{
				JsonName:       responseName,
				Schema:         goType,
				Name:           goTypeName,
				SpecLocation:   SpecLocationResponse,
				NeedsMarshaler: needsMarshaler(goType),
			}
			options.AddType(typeDef)

			// TODO: check if same as ref
			contentRef := ""
			if content.Schema != nil {
				contentRef = content.Schema.GoLow().GetReference()
			}
			if contentRef != "" {
				// Generate a reference type for referenced parameters
				refType, err := refPathToGoType(contentRef)
				if err != nil {
					return nil, fmt.Errorf("error generating Go type for (%s) in parameter %s: %w",
						content.Schema.GetReference(), responseName, err)
				}
				renamed := schemaNameToTypeName(refType)
				// typeDef.Name = renamed
				typeDef.Schema.RefType = renamed
				typeDef.Schema.DefineViaAlias = true
			}

			types = append(types, typeDef)
		}
	}
	return types, nil
}
