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
	"slices"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

func filterOutDocument(doc libopenapi.Document, cfg FilterConfig) (libopenapi.Document, error) {
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("error building model: %w", err)
	}

	removeAllExamples(&model.Model)
	filterOperations(&model.Model, cfg)
	filterComponentSchemaProperties(&model.Model, cfg)

	_, doc, _, err = doc.RenderAndReload()
	if err != nil {
		return nil, fmt.Errorf("error reloading document: %w", err)
	}

	return doc, nil
}

func filterOperations(model *v3high.Document, cfg FilterConfig) {
	if cfg.IsEmpty() {
		return
	}

	paths := map[string]*v3high.PathItem{}

	// iterate over copy
	if model.Paths != nil && model.Paths.PathItems != nil {
		for path, pathItem := range model.Paths.PathItems.FromOldest() {
			paths[path] = pathItem
		}
	}

	for path, pathItem := range paths {
		if cfg.Include.Paths != nil && !slices.Contains(cfg.Include.Paths, path) {
			model.Paths.PathItems.Delete(path)
			continue
		}

		if cfg.Exclude.Paths != nil && slices.Contains(cfg.Exclude.Paths, path) {
			model.Paths.PathItems.Delete(path)
			continue
		}

		for method, op := range pathItem.GetOperations().FromOldest() {
			remove := false

			// Tags
			for _, tag := range op.Tags {
				if slices.Contains(cfg.Exclude.Tags, tag) {
					remove = true
					break
				}
			}

			if !remove && len(cfg.Include.Tags) > 0 {
				// Only include if it matches Include.Tags
				includeMatch := false
				for _, tag := range op.Tags {
					if slices.Contains(cfg.Include.Tags, tag) {
						includeMatch = true
						break
					}
				}
				if !includeMatch {
					remove = true
				}
			}

			// OperationIDs
			if cfg.Exclude.OperationIDs != nil && slices.Contains(cfg.Exclude.OperationIDs, op.OperationId) {
				remove = true
			}
			if cfg.Include.OperationIDs != nil && !slices.Contains(cfg.Include.OperationIDs, op.OperationId) {
				remove = true
			}

			if remove {
				switch strings.ToLower(method) {
				case "get":
					pathItem.Get = nil
				case "post":
					pathItem.Post = nil
				case "put":
					pathItem.Put = nil
				case "delete":
					pathItem.Delete = nil
				case "patch":
					pathItem.Patch = nil
				case "head":
					pathItem.Head = nil
				case "options":
					pathItem.Options = nil
				case "trace":
					pathItem.Trace = nil
				}
			}
		}
	}
}

func filterComponentSchemaProperties(model *v3high.Document, cfg FilterConfig) {
	if cfg.IsEmpty() {
		return
	}

	if model.Components == nil || model.Components.Schemas == nil {
		return
	}

	includeExts := sliceToBoolMap(cfg.Include.Extensions)
	excludeExts := sliceToBoolMap(cfg.Exclude.Extensions)

	for schemaName, schemaProxy := range model.Components.Schemas.FromOldest() {
		schema := schemaProxy.Schema()
		if schema == nil || schema.Properties == nil {
			continue
		}

		if schema.Extensions.Len() > 0 && (len(includeExts) > 0 || len(excludeExts) > 0) {
			newExtensions := orderedmap.New[string, *yaml.Node]()
			for key, val := range schema.Extensions.FromOldest() {
				if shouldIncludeExtension(key, includeExts, excludeExts) {
					newExtensions.Set(key, val)
				}
			}
			schema.Extensions = newExtensions
		}

		var copiedKeys []string
		for prop := range schema.Properties.KeysFromOldest() {
			copiedKeys = append(copiedKeys, prop)
		}

		for _, propName := range copiedKeys {
			isRequired := slices.Contains(schema.Required, propName)
			if isRequired {
				continue
			}

			if include := cfg.Include.SchemaProperties[schemaName]; include != nil {
				if !slices.Contains(include, propName) {
					schema.Properties.Delete(propName)
				}
			}

			if exclude := cfg.Exclude.SchemaProperties[schemaName]; exclude != nil {
				if slices.Contains(exclude, propName) {
					schema.Properties.Delete(propName)
				}
			}
		}
	}
}

func shouldIncludeExtension(ext string, includeExts, excludeExts map[string]bool) bool {
	if len(includeExts) > 0 {
		return includeExts[ext]
	}

	if len(excludeExts) > 0 {
		return !excludeExts[ext]
	}

	return true
}

func sliceToBoolMap(slice []string) map[string]bool {
	m := make(map[string]bool, len(slice))
	for _, s := range slice {
		m[s] = true
	}
	return m
}

// removeAllExamples removes the components/examples section and all "examples" fields
// that contain references. Inline "example" (singular) fields are preserved.
func removeAllExamples(model *v3high.Document) {
	visited := make(map[*base.SchemaProxy]bool)

	if model.Components != nil {
		model.Components.Examples = nil

		if model.Components.Schemas != nil {
			for _, schemaProxy := range model.Components.Schemas.FromOldest() {
				removeExamplesFromSchemaProxy(schemaProxy, visited)
			}
		}

		if model.Components.RequestBodies != nil {
			for _, requestBody := range model.Components.RequestBodies.FromOldest() {
				removeExamplesFromMediaTypes(requestBody.Content, visited)
			}
		}

		if model.Components.Responses != nil {
			for _, response := range model.Components.Responses.FromOldest() {
				removeExamplesFromMediaTypes(response.Content, visited)
				removeExamplesFromHeaderMap(response.Headers, visited)
			}
		}

		if model.Components.Parameters != nil {
			for _, param := range model.Components.Parameters.FromOldest() {
				removeExamplesFromParameter(param, visited)
			}
		}

		if model.Components.Headers != nil {
			for _, header := range model.Components.Headers.FromOldest() {
				removeExamplesFromHeader(header, visited)
			}
		}
	}

	if model.Paths != nil && model.Paths.PathItems != nil {
		for _, pathItem := range model.Paths.PathItems.FromOldest() {
			for _, op := range pathItem.GetOperations().FromOldest() {
				if op.RequestBody != nil {
					removeExamplesFromMediaTypes(op.RequestBody.Content, visited)
				}

				if op.Responses != nil && op.Responses.Codes != nil {
					for _, response := range op.Responses.Codes.FromOldest() {
						removeExamplesFromMediaTypes(response.Content, visited)
						removeExamplesFromHeaderMap(response.Headers, visited)
					}
				}

				for _, param := range op.Parameters {
					removeExamplesFromParameter(param, visited)
				}
			}
		}
	}
}

// removeExamplesFromSchemaProxy removes the "examples" field from a schema and its nested properties.
// The "example" field (singular) is preserved as it's typically inline.
// Uses a visited map to prevent infinite recursion on circular references.
func removeExamplesFromSchemaProxy(schemaProxy *base.SchemaProxy, visited map[*base.SchemaProxy]bool) {
	if schemaProxy == nil || visited[schemaProxy] {
		return
	}

	visited[schemaProxy] = true
	schema := schemaProxy.Schema()
	if schema == nil {
		return
	}

	schema.Examples = nil

	if schema.Properties != nil {
		for _, propProxy := range schema.Properties.FromOldest() {
			removeExamplesFromSchemaProxy(propProxy, visited)
		}
	}

	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		removeExamplesFromSchemaProxy(schema.AdditionalProperties.A, visited)
	}

	if schema.Items != nil && schema.Items.IsA() {
		removeExamplesFromSchemaProxy(schema.Items.A, visited)
	}
}

// removeExamplesFromMediaTypes removes the "examples" field from media type content
func removeExamplesFromMediaTypes(content *orderedmap.Map[string, *v3high.MediaType], visited map[*base.SchemaProxy]bool) {
	if content == nil {
		return
	}

	for _, mediaType := range content.FromOldest() {
		mediaType.Examples = nil
		if mediaType.Schema != nil {
			removeExamplesFromSchemaProxy(mediaType.Schema, visited)
		}
	}
}

// removeExamplesFromParameter removes the "examples" field from a parameter
func removeExamplesFromParameter(param *v3high.Parameter, visited map[*base.SchemaProxy]bool) {
	if param == nil {
		return
	}

	param.Examples = nil
	if param.Schema != nil {
		removeExamplesFromSchemaProxy(param.Schema, visited)
	}
	removeExamplesFromMediaTypes(param.Content, visited)
}

// removeExamplesFromHeader removes the "examples" field from a header
func removeExamplesFromHeader(header *v3high.Header, visited map[*base.SchemaProxy]bool) {
	if header == nil {
		return
	}

	header.Examples = nil
	if header.Schema != nil {
		removeExamplesFromSchemaProxy(header.Schema, visited)
	}
	removeExamplesFromMediaTypes(header.Content, visited)
}

// removeExamplesFromHeaderMap removes the "examples" field from a map of headers
func removeExamplesFromHeaderMap(headers *orderedmap.Map[string, *v3high.Header], visited map[*base.SchemaProxy]bool) {
	if headers == nil {
		return
	}

	for _, header := range headers.FromOldest() {
		removeExamplesFromHeader(header, visited)
	}
}
