package codegen

import (
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func MergeDocuments(src, other libopenapi.Document) (libopenapi.Document, error) {
	srcModel, errs := src.BuildV3Model()
	if len(errs) > 0 {
		return nil, fmt.Errorf("error building model for src: %w", errs[0])
	}

	otherModel, errs := other.BuildV3Model()
	if len(errs) > 0 {
		return nil, fmt.Errorf("error building model for other: %w", errs[0])
	}

	mergeOperations(srcModel, otherModel)

	// Merge the components of the two documents
	if otherModel.Model.Components != nil && otherModel.Model.Components.Schemas != nil {
		for compName, schemaProxy := range otherModel.Model.Components.Schemas.FromOldest() {
			current, exists := srcModel.Model.Components.Schemas.Get(compName)
			if !exists {
				srcModel.Model.Components.Schemas.Set(compName, schemaProxy)
				continue
			}
			mergeSchemaProxy(current, schemaProxy)
		}
	}

	_, res, _, errs := src.RenderAndReload()
	if errs != nil {
		return nil, fmt.Errorf("error reloading document: %w", errs[0])
	}
	return res, nil
}

func mergeOperations(srcModel, otherModel *libopenapi.DocumentModel[v3.Document]) {
	if srcModel == nil || otherModel == nil || otherModel.Model.Paths == nil || otherModel.Model.Paths.PathItems == nil {
		return
	}

	for path, pathItem := range otherModel.Model.Paths.PathItems.FromOldest() {
		current, exists := srcModel.Model.Paths.PathItems.Get(path)
		if !exists {
			srcModel.Model.Paths.PathItems.Set(path, pathItem)
			continue
		}

		for method, operation := range pathItem.GetOperations().FromOldest() {
			currentOperation, opExists := current.GetOperations().Get(method)
			if !opExists {
				switch strings.ToLower(method) {
				case "get":
					current.Get = operation
				case "post":
					current.Post = operation
				case "put":
					current.Put = operation
				case "delete":
					current.Delete = operation
				case "patch":
					current.Patch = operation
				case "head":
					current.Head = operation
				case "options":
					current.Options = operation
				case "trace":
					current.Trace = operation
				}
				continue
			}

			// Merge parameters
			existingParams := currentOperation.Parameters
			existingParams = append(existingParams, operation.Parameters...)
			currentOperation.Parameters = existingParams

			// Merge request body
			if operation.RequestBody != nil {
				for contentType, content := range operation.RequestBody.Content.FromOldest() {
					reqBodyExists := false
					var currentContent *v3.MediaType

					if currentOperation.RequestBody != nil {
						currentContent, reqBodyExists = currentOperation.RequestBody.Content.Get(contentType)
					}

					if reqBodyExists {
						mergeSchemaProxy(currentContent.Schema, content.Schema)
					} else {
						if currentOperation.RequestBody == nil {
							currentOperation.RequestBody = operation.RequestBody
						} else {
							currentOperation.RequestBody.Content.Set(contentType, content)
						}
					}
				}
			}

			// Merge responses
			if operation.Responses != nil {
				for code, response := range operation.Responses.Codes.FromOldest() {
					currentResponse, resExists := currentOperation.Responses.Codes.Get(code)
					if resExists {
						mergeResponses(currentResponse, response)
						continue
					}
					currentOperation.Responses.Codes.Set(code, response)
				}
			}
		}
	}
}

func mergeSchemaProxy(src *base.SchemaProxy, other *base.SchemaProxy) {
	if src == nil || other == nil {
		return
	}

	srcRef := src.GoLow().GetReference()
	if srcRef != "" {
		// If the source schema is a reference, we can't merge it with another schema right now.
		return
	}

	if src.Schema() == nil {
		return
	}

	if src.Schema().Properties == nil {
		src.Schema().Properties = other.Schema().Properties
	} else {
		for key, value := range other.Schema().Properties.FromOldest() {
			srcKeySchema, exists := src.Schema().Properties.Get(key)
			if exists {
				mergeSchemaProxy(srcKeySchema, value)
			} else {
				src.Schema().Properties.Set(key, value)
			}
		}
	}

	if len(src.Schema().Enum) > 0 {
		for _, enumNode := range other.Schema().Enum {
			src.Schema().Enum = append(src.Schema().Enum, enumNode)
		}
	}

	for _, schemaProxies := range other.Schema().AllOf {
		src.Schema().AllOf = append(src.Schema().AllOf, schemaProxies)
	}

	for _, schemaProxies := range other.Schema().AnyOf {
		src.Schema().AnyOf = append(src.Schema().AnyOf, schemaProxies)
	}

	for _, schemaProxies := range other.Schema().OneOf {
		src.Schema().OneOf = append(src.Schema().OneOf, schemaProxies)
	}

	// overwrite completely
	if other.Schema().Not != nil {
		src.Schema().Not = other.Schema().Not
	}
}

func mergeResponses(src, other *v3.Response) {
	if src == nil || other == nil {
		return
	}

	// Merge headers
	for typ, response := range other.Headers.FromOldest() {
		src.Headers.Set(typ, response)
	}

	// Merge content
	for contentType, content := range other.Content.FromOldest() {
		srcContent, exists := src.Content.Get(contentType)
		if exists {
			mergeSchemaProxy(srcContent.Schema, content.Schema)
		} else {
			src.Content.Set(contentType, content)
		}
	}
}
