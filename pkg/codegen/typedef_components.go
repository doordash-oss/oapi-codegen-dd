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

	// First pass: register all schema names with their refs.
	// This allows forward references to be resolved correctly during schema generation.
	schemaNames := make(map[string]string) // schemaName -> goTypeName
	for schemaName, schemaRef := range schemas.FromOldest() {
		goTypeName, err := renameComponent(schemaNameToTypeName(schemaName), schemaRef)
		if err != nil {
			return nil, fmt.Errorf("error making name for components/schemas/%s: %w", schemaName, err)
		}

		// Check if a type with the same name already exists (e.g., from parameters).
		// If it does, generate a unique name to avoid conflicts.
		if options.typeTracker.Exists(goTypeName) {
			goTypeName = options.typeTracker.generateUniqueName(goTypeName)
		}

		schemaNames[schemaName] = goTypeName
		componentRef := "#/components/schemas/" + schemaName
		// Register just the name and ref mapping - full type definition comes in second pass
		options.typeTracker.registerName(goTypeName)
		options.typeTracker.registerRef(componentRef, goTypeName)
	}

	// Second pass: generate full schemas (now all refs can be resolved)
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

		goTypeName := schemaNames[schemaName]

		td := TypeDefinition{
			JsonName:         schemaName,
			Name:             goTypeName,
			Schema:           goSchema,
			SpecLocation:     SpecLocationSchema,
			NeedsMarshaler:   needsMarshaler(goSchema),
			HasSensitiveData: hasSensitiveData(goSchema),
		}
		types = append(types, td)
		// Update the registration with full type definition
		componentRef := "#/components/schemas/" + schemaName
		opts.typeTracker.register(td, componentRef)

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

		ref := ""
		if paramOrRef.Schema != nil {
			ref = paramOrRef.Schema.GoLow().GetReference()
		}

		// For inline schemas (no $ref), we need to create a type alias
		// For referenced schemas, we use the referenced type name
		if ref == "" {
			// Inline schema - create a type alias
			goType.DefineViaAlias = true
		}

		typeDef := TypeDefinition{
			JsonName:         paramName,
			Schema:           goType,
			Name:             goTypeName,
			SpecLocation:     SpecLocation(strings.ToLower(paramOrRef.In)),
			NeedsMarshaler:   needsMarshaler(goType),
			HasSensitiveData: hasSensitiveData(goType),
		}
		// Register with the original ref path so we can look it up later
		paramRef := "#/components/parameters/" + paramName
		options.typeTracker.register(typeDef, paramRef)

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
			options.typeTracker.register(typeDef, "")

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

			// Check if a type with the same name already exists (e.g., from components/schemas).
			// If so, and the existing type is different (e.g., schema is struct, response is array),
			// generate a unique name for the response type.
			if existingType, exists := options.typeTracker.LookupByName(goTypeName); exists {
				// Check if the types are different (different GoType or different structure)
				isDifferent := existingType.Schema.GoType != goType.GoType ||
					(existingType.Schema.ArrayType == nil) != (goType.ArrayType == nil)
				if isDifferent {
					// Generate a unique name by appending "Response" suffix
					goTypeName = options.typeTracker.generateUniqueNameWithSuffixes(goTypeName, []string{"Response"})
				}
			}

			typeDef := TypeDefinition{
				JsonName:       responseName,
				Schema:         goType,
				Name:           goTypeName,
				SpecLocation:   SpecLocationResponse,
				NeedsMarshaler: needsMarshaler(goType),
			}

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

			// Register after setting RefType and DefineViaAlias so the tracker has the correct values
			options.typeTracker.register(typeDef, "")
			types = append(types, typeDef)
		}
	}
	return types, nil
}
