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
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

func filterOutDocument(doc libopenapi.Document, cfg FilterConfig) (libopenapi.Document, bool, error) {
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, false, fmt.Errorf("error building model: %w", err)
	}

	removedOperations := filterOperations(&model.Model, cfg)
	removedProperties := filterComponentSchemaProperties(&model.Model, cfg)
	filtered := removedOperations || removedProperties

	// Don't reload yet - let the caller decide when to reload (after pruning if needed)
	return doc, filtered, nil
}

func filterOperations(model *v3high.Document, cfg FilterConfig) bool {
	if cfg.IsEmpty() {
		return false
	}

	removed := false
	paths := map[string]*v3high.PathItem{}

	// iterate over copy
	if model.Paths != nil && model.Paths.PathItems != nil {
		for path, pathItem := range model.Paths.PathItems.FromOldest() {
			paths[path] = pathItem
		}
	}

	for path, pathItem := range paths {
		if len(cfg.Include.Paths) > 0 && !slices.Contains(cfg.Include.Paths, path) {
			model.Paths.PathItems.Delete(path)
			removed = true
			continue
		}

		if len(cfg.Exclude.Paths) > 0 && slices.Contains(cfg.Exclude.Paths, path) {
			model.Paths.PathItems.Delete(path)
			removed = true
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
			if len(cfg.Exclude.OperationIDs) > 0 && slices.Contains(cfg.Exclude.OperationIDs, op.OperationId) {
				remove = true
			}
			if len(cfg.Include.OperationIDs) > 0 && !slices.Contains(cfg.Include.OperationIDs, op.OperationId) {
				remove = true
			}

			if remove {
				removed = true
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

	return removed
}

func filterComponentSchemaProperties(model *v3high.Document, cfg FilterConfig) bool {
	if cfg.IsEmpty() {
		return false
	}

	if model.Components == nil || model.Components.Schemas == nil {
		return false
	}

	removed := false
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
			if newExtensions.Len() != schema.Extensions.Len() {
				removed = true
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
					removed = true
				}
			}

			if exclude := cfg.Exclude.SchemaProperties[schemaName]; exclude != nil {
				if slices.Contains(exclude, propName) {
					schema.Properties.Delete(propName)
					removed = true
				}
			}
		}
	}

	return removed
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
