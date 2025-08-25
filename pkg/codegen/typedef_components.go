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
		goSchema, err := GenerateGoSchema(schemaRef, ref, []string{schemaName}, options)
		if err != nil {
			return nil, fmt.Errorf("error converting GoSchema %s to Go type: %w", schemaName, err)
		}
		if goSchema.IsZero() {
			continue
		}

		goTypeName, err := renameSchema(schemaName, schemaRef)
		if err != nil {
			return nil, fmt.Errorf("error making name for components/schemas/%s: %w", schemaName, err)
		}

		types = append(types, TypeDefinition{
			JsonName:     schemaName,
			Name:         goTypeName,
			Schema:       goSchema,
			SpecLocation: SpecLocationSchema,
		})

		types = append(types, goSchema.AdditionalTypes...)
	}

	return types, nil
}

// getComponentParameters generates type definitions for any custom types defined in the
// components/parameters section of the Swagger spec.
func getComponentParameters(params *orderedmap.Map[string, *v3high.Parameter], options ParseOptions) ([]TypeDefinition, error) {
	var types []TypeDefinition
	for paramName, paramOrRef := range params.FromOldest() {
		goType, err := paramToGoType(paramOrRef, nil, options)
		if err != nil {
			return nil, fmt.Errorf("error generating Go type for schema in parameter %s: %w", paramName, err)
		}

		goTypeName, err := renameParameter(paramName, paramOrRef)
		if err != nil {
			return nil, fmt.Errorf("error making name for components/parameters/%s: %w", paramName, err)
		}

		typeDef := TypeDefinition{
			JsonName:     paramName,
			Schema:       goType,
			Name:         goTypeName,
			SpecLocation: SpecLocation(strings.ToLower(paramOrRef.In)),
		}

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
			goType, err := GenerateGoSchema(body.Schema, ref, []string{requestBodyName}, options)
			if err != nil {
				return nil, fmt.Errorf("error generating Go type for schema in body %s: %w", requestBodyName, err)
			}
			if goType.IsZero() {
				continue
			}

			goTypeName, err := renameRequestBody(requestBodyName, body.Schema)
			if err != nil {
				return nil, fmt.Errorf("error making name for components/schemas/%s: %w", requestBodyName, err)
			}

			typeDef := TypeDefinition{
				JsonName:     requestBodyName,
				Schema:       goType,
				Name:         goTypeName,
				SpecLocation: SpecLocationBody,
			}

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
			goType, err := GenerateGoSchema(content.Schema, ref, []string{responseName}, options)
			if err != nil {
				return nil, fmt.Errorf("error generating Go type for schema in response %s: %w", responseName, err)
			}

			goTypeName, err := renameResponse(responseName, content.Schema)
			if err != nil {
				return nil, fmt.Errorf("error making name for components/responses/%s: %w", responseName, err)
			}

			typeDef := TypeDefinition{
				JsonName:     responseName,
				Schema:       goType,
				Name:         goTypeName,
				SpecLocation: SpecLocationResponse,
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

			types = append(types, typeDef)
		}
	}
	return types, nil
}
