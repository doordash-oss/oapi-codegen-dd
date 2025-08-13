package codegen

import (
	"fmt"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

const extSrcMergeRef = "x-src-merge-ref"

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
		if srcModel.Model.Components == nil {
			srcModel.Model.Components = &v3.Components{
				Schemas: orderedmap.New[string, *base.SchemaProxy](),
			}
		}
		for compName, schemaProxy := range otherModel.Model.Components.Schemas.FromOldest() {
			current, exists := srcModel.Model.Components.Schemas.Get(compName)
			if !exists {
				srcModel.Model.Components.Schemas.Set(compName, schemaProxy)
				continue
			}
			mergeSchemaProxy(current, schemaProxy, srcModel)
			resolveRefExtensions(current, srcModel)
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
						mergeSchemaProxy(currentContent.Schema, content.Schema, srcModel)
					} else {
						if currentOperation.RequestBody == nil {
							currentOperation.RequestBody = operation.RequestBody
						} else {
							currentOperation.RequestBody.Content.Set(contentType, content)
						}
					}
					if currentContent != nil {
						resolveRefExtensions(currentContent.Schema, srcModel)
					}
				}
			}

			// Merge responses
			if operation.Responses != nil {
				for code, response := range operation.Responses.Codes.FromOldest() {
					currentResponse, resExists := currentOperation.Responses.Codes.Get(code)
					if resExists {
						mergeResponses(currentResponse, response, srcModel)
						continue
					}
					currentOperation.Responses.Codes.Set(code, response)
				}
			}
		}
	}
}

func mergeSchemaProxy(src *base.SchemaProxy, other *base.SchemaProxy, docModel *libopenapi.DocumentModel[v3.Document]) {
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

	otherLow := other.GoLow()
	otherRef := otherLow.GetReference()
	if otherRef != "" {
		src.GoLow().SetReference(otherRef, otherLow.GetReferenceNode())
		return
	}

	if src.Schema().Properties == nil {
		src.Schema().Properties = other.Schema().Properties
	} else {
		for key, value := range other.Schema().Properties.FromOldest() {
			srcKeySchema, exists := src.Schema().Properties.Get(key)
			if !exists {
				src.Schema().Properties.Set(key, value)
				continue
			}
			mergeSchemaProxy(srcKeySchema, value, docModel)
		}
	}

	if src.Schema().Items == nil {
		src.Schema().Items = other.Schema().Items
	} else {
		if other.Schema().Items != nil && other.Schema().Items.IsA() && other.Schema().Items.A != nil {
			srcItems := src.Schema().Items.A
			mergeSchemaProxy(srcItems, other.Schema().Items.A, docModel)
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

	if other.Schema().Extensions != nil && other.Schema().Extensions.Len() > 0 {
		for key, value := range other.Schema().Extensions.FromOldest() {
			src.Schema().Extensions.Set(key, value)
		}
	}

	if len(other.Schema().Required) > 0 {
		src.Schema().Required = other.Schema().Required
	}

	if other.Schema().Minimum != nil {
		src.Schema().Minimum = other.Schema().Minimum
	}

	if other.Schema().Maximum != nil {
		src.Schema().Maximum = other.Schema().Maximum
	}

	if other.Schema().ExclusiveMinimum != nil {
		src.Schema().ExclusiveMinimum = other.Schema().ExclusiveMinimum
	}

	if other.Schema().ExclusiveMaximum != nil {
		src.Schema().ExclusiveMaximum = other.Schema().ExclusiveMaximum
	}
}

func mergeResponses(src, other *v3.Response, docModel *libopenapi.DocumentModel[v3.Document]) {
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
			mergeSchemaProxy(srcContent.Schema, content.Schema, docModel)
			resolveRefExtensions(srcContent.Schema, docModel)
		} else {
			src.Content.Set(contentType, content)
		}
	}
}

// resolveRefExtensions resolves the x-src-merge-ref extension in the schema and sets the source ref to the
// ref pointed to by the other schema. This is used to merge schemas that are referenced in the OpenAPI document.
func resolveRefExtensions(src *base.SchemaProxy, docModel *libopenapi.DocumentModel[v3.Document]) {
	if src == nil || src.Schema() == nil {
		return
	}

	if src.Schema().Properties != nil && src.Schema().Properties.Len() > 0 {
		for _, prop := range src.Schema().Properties.FromOldest() {
			resolveRefExtensions(prop, docModel)
		}
	}

	if src.Schema().Items != nil && src.Schema().Items.IsA() && src.Schema().Items.A != nil {
		resolveRefExtensions(src.Schema().Items.A, docModel)
	}

	// set source ref to the ref pointed to by the other schema
	valueExtensions := src.Schema().Extensions
	if valueExtensions == nil || valueExtensions.Len() == 0 {
		return
	}

	// set source ref to the ref pointed to by the other schema
	exts := extractExtensions(valueExtensions)
	if exts == nil || exts[extSrcMergeRef] == nil {
		return
	}

	refName, _ := parseString(exts[extSrcMergeRef])

	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(refName, prefix) {
		return
	}

	schemaName := strings.TrimPrefix(refName, prefix)
	if schemaName == "" {
		return
	}

	ref := docModel.Model.Components.Schemas.Value(schemaName)
	if ref != nil {
		src.GoLow().SetReference(refName, ref.GoLow().GetReferenceNode())
		src.Schema().Properties = nil
		return
	}
}
