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
	"iter"
	"slices"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func pruneSchema(doc libopenapi.Document) (libopenapi.Document, error) {
	for {
		model, err := doc.BuildV3Model()
		if err != nil {
			return nil, fmt.Errorf("error building model: %w", err)
		}

		refs := findOperationRefs(&model.Model)
		countRemoved := removeOrphanedComponents(&model.Model, refs)

		_, doc, _, err = doc.RenderAndReload()
		if err != nil {
			return nil, fmt.Errorf("error reloading document: %w", err)
		}

		if countRemoved < 1 {
			return doc, nil
		}
	}
}

func removeOrphanedComponents(model *v3high.Document, refs []string) int {
	if model.Components == nil {
		return 0
	}

	countRemoved := 0

	for _, key := range getComponentKeys(model.Components.Schemas.KeysFromOldest()) {
		ref := fmt.Sprintf("#/components/schemas/%s", key)
		if !slices.Contains(refs, ref) {
			countRemoved++
			model.Components.Schemas.Delete(key)
		}
	}

	for _, key := range getComponentKeys(model.Components.Parameters.KeysFromOldest()) {
		ref := fmt.Sprintf("#/components/parameters/%s", key)
		if !slices.Contains(refs, ref) {
			countRemoved++
			model.Components.Parameters.Delete(key)
		}
	}

	for _, key := range getComponentKeys(model.Components.RequestBodies.KeysFromOldest()) {
		ref := fmt.Sprintf("#/components/requestBodies/%s", key)
		if !slices.Contains(refs, ref) {
			countRemoved++
			model.Components.RequestBodies.Delete(key)
		}
	}

	for _, key := range getComponentKeys(model.Components.Responses.KeysFromOldest()) {
		ref := fmt.Sprintf("#/components/responses/%s", key)
		if !slices.Contains(refs, ref) {
			countRemoved++
			model.Components.Responses.Delete(key)
		}
	}

	for _, key := range getComponentKeys(model.Components.Headers.KeysFromOldest()) {
		ref := fmt.Sprintf("#/components/headers/%s", key)
		if !slices.Contains(refs, ref) {
			countRemoved++
			model.Components.Headers.Delete(key)
		}
	}

	for _, key := range getComponentKeys(model.Components.Examples.KeysFromOldest()) {
		ref := fmt.Sprintf("#/components/examples/%s", key)
		if !slices.Contains(refs, ref) {
			countRemoved++
			model.Components.Examples.Delete(key)
		}
	}

	for _, key := range getComponentKeys(model.Components.Links.KeysFromOldest()) {
		ref := fmt.Sprintf("#/components/links/%s", key)
		if !slices.Contains(refs, ref) {
			countRemoved++
			model.Components.Links.Delete(key)
		}
	}

	for _, key := range getComponentKeys(model.Components.Callbacks.KeysFromOldest()) {
		ref := fmt.Sprintf("#/components/callbacks/%s", key)
		if !slices.Contains(refs, ref) {
			countRemoved++
			model.Components.Callbacks.Delete(key)
		}
	}

	return countRemoved
}

func findOperationRefs(model *v3high.Document) []string {
	refSet := make(map[string]bool)

	if model.Paths == nil || model.Paths.PathItems == nil {
		return []string{}
	}

	for _, pathItem := range model.Paths.PathItems.FromOldest() {
		for _, op := range pathItem.GetOperations().FromOldest() {
			if op.RequestBody != nil {
				ref := op.RequestBody.GoLow().GetReference()
				if ref != "" {
					refSet[ref] = true
				}
				for _, mediaType := range op.RequestBody.Content.FromOldest() {
					if mediaType.Schema == nil {
						continue
					}
					medRef := mediaType.Schema.GetReference()
					if medRef != "" {
						refSet[medRef] = true
					}
					if mediaType.Schema != nil {
						collectSchemaRefs(mediaType.Schema.Schema(), refSet)
					}
				}
			}

			for _, param := range op.Parameters {
				if param == nil {
					continue
				}
				ref := param.GoLow().GetReference()
				if ref != "" {
					refSet[ref] = true
				}
				// Collect schema refs from param.Schema
				if param.Schema != nil {
					// Collect the $ref from the SchemaProxy itself
					if schemaRef := param.Schema.GoLow().GetReference(); schemaRef != "" {
						refSet[schemaRef] = true
					}
					collectSchemaRefs(param.Schema.Schema(), refSet)
				}
				for _, mediaType := range param.Content.FromOldest() {
					if mediaType.Schema != nil {
						// Collect the $ref from the SchemaProxy itself
						if schemaRef := mediaType.Schema.GoLow().GetReference(); schemaRef != "" {
							refSet[schemaRef] = true
						}
						collectSchemaRefs(mediaType.Schema.Schema(), refSet)
					}
				}
			}

			if op.Responses != nil && op.Responses.Default != nil {
				ref := op.Responses.Default.GoLow().GetReference()
				if ref != "" {
					refSet[ref] = true
				}
				for _, mediaType := range op.Responses.Default.Content.FromOldest() {
					if mediaType.Schema != nil {
						mRef := mediaType.Schema.GoLow().GetReference()
						if mRef != "" {
							refSet[mRef] = true
						}
						collectSchemaRefs(mediaType.Schema.Schema(), refSet)
					}
				}
			}

			if op.Responses != nil {
				for _, resp := range op.Responses.Codes.FromOldest() {
					if resp == nil {
						continue
					}
					ref := resp.GoLow().GetReference()
					if ref != "" {
						refSet[ref] = true
					}

					for _, mediaType := range resp.Content.FromOldest() {
						if mediaType.Schema != nil {
							mRef := mediaType.Schema.GoLow().GetReference()
							if mRef != "" {
								refSet[mRef] = true
							}
							collectSchemaRefs(mediaType.Schema.Schema(), refSet)
						}
					}

					for _, header := range resp.Headers.FromOldest() {
						if header == nil {
							continue
						}
						hRef := header.GoLow().GetReference()
						if hRef != "" {
							refSet[hRef] = true
						}
						for _, mediaType := range header.Content.FromOldest() {
							if mediaType.Schema != nil {
								mRef := mediaType.Schema.GoLow().GetReference()
								if mRef != "" {
									refSet[mRef] = true
								}
								collectSchemaRefs(mediaType.Schema.Schema(), refSet)
							}
						}
					}
				}
			}
		}

		// collect path parameters< defined in the path item for all methods
		for _, param := range pathItem.Parameters {
			if param == nil {
				continue
			}
			ref := param.GoLow().GetReference()
			if ref != "" {
				refSet[ref] = true
			}

			// Collect schema refs from param.Schema
			if param.Schema != nil {
				collectSchemaRefs(param.Schema.Schema(), refSet)
			}
			for _, mediaType := range param.Content.FromOldest() {
				if mediaType.Schema != nil {
					collectSchemaRefs(mediaType.Schema.Schema(), refSet)
				}
			}
		}
	}

	// Collect schema refs from component parameters
	if model.Components != nil && model.Components.Parameters != nil {
		for _, param := range model.Components.Parameters.FromOldest() {
			if param == nil {
				continue
			}
			// Collect schema refs from param.Schema
			if param.Schema != nil {
				schemaRef := param.Schema.GoLow().GetReference()
				if schemaRef != "" {
					refSet[schemaRef] = true
				}
				collectSchemaRefs(param.Schema.Schema(), refSet)
			}
			// Collect schema refs from param.Content
			for _, mediaType := range param.Content.FromOldest() {
				if mediaType.Schema != nil {
					collectSchemaRefs(mediaType.Schema.Schema(), refSet)
				}
			}
		}
	}

	// Collect schema refs from component request bodies
	if model.Components != nil && model.Components.RequestBodies != nil {
		for _, reqBody := range model.Components.RequestBodies.FromOldest() {
			if reqBody == nil {
				continue
			}
			for _, mediaType := range reqBody.Content.FromOldest() {
				if mediaType.Schema != nil {
					collectSchemaRefs(mediaType.Schema.Schema(), refSet)
				}
			}
		}
	}

	// Collect schema refs from component responses
	if model.Components != nil && model.Components.Responses != nil {
		for _, resp := range model.Components.Responses.FromOldest() {
			if resp == nil {
				continue
			}
			for _, mediaType := range resp.Content.FromOldest() {
				if mediaType.Schema != nil {
					collectSchemaRefs(mediaType.Schema.Schema(), refSet)
				}
			}
			// Collect schema refs from response headers
			for _, header := range resp.Headers.FromOldest() {
				if header == nil {
					continue
				}
				if header.Schema != nil {
					collectSchemaRefs(header.Schema.Schema(), refSet)
				}
				for _, mediaType := range header.Content.FromOldest() {
					if mediaType.Schema != nil {
						collectSchemaRefs(mediaType.Schema.Schema(), refSet)
					}
				}
			}
		}
	}

	// Collect schema refs from component headers
	if model.Components != nil && model.Components.Headers != nil {
		for _, header := range model.Components.Headers.FromOldest() {
			if header == nil {
				continue
			}
			if header.Schema != nil {
				collectSchemaRefs(header.Schema.Schema(), refSet)
			}
			for _, mediaType := range header.Content.FromOldest() {
				if mediaType.Schema != nil {
					collectSchemaRefs(mediaType.Schema.Schema(), refSet)
				}
			}
		}
	}

	refs := make([]string, 0, len(refSet))
	for r := range refSet {
		refs = append(refs, r)
	}
	slices.Sort(refs)
	return refs
}

func collectSchemaRefs(schema *base.Schema, refSet map[string]bool) {
	collectSchemaRefsInternal(schema, refSet)
}

func collectSchemaRefsInternal(schema *base.Schema, refSet map[string]bool) {
	if schema == nil {
		return
	}

	// Check if this schema is a $ref
	if ref := schema.GoLow().GetReference(); ref != "" {
		if refSet[ref] {
			return
		}
		refSet[ref] = true
	}

	// Traverse object properties
	if schema.Properties != nil {
		for _, prop := range schema.Properties.FromOldest() {
			if prop == nil {
				continue
			}
			// Check for $ref on the property itself
			if pRef := prop.GoLow().GetReference(); pRef != "" {
				if !refSet[pRef] {
					refSet[pRef] = true
					// Don't return — we still want to walk its schema if possible
				} else {
					continue
				}
			}
			collectSchemaRefsInternal(prop.Schema(), refSet)
		}
	}

	// Traverse array items
	if items := schema.Items; items != nil && items.IsA() && items.A != nil {
		if iRef := items.A.GoLow().GetReference(); iRef != "" {
			if !refSet[iRef] {
				refSet[iRef] = true
				// keep walking
			} else {
				return
			}
		}
		collectSchemaRefsInternal(items.A.Schema(), refSet)
	}

	// Traverse additionalProperties
	if ap := schema.AdditionalProperties; ap != nil && ap.IsA() && ap.A != nil {
		if aRef := ap.A.GoLow().GetReference(); aRef != "" {
			if !refSet[aRef] {
				refSet[aRef] = true
			} else {
				return
			}
		}
		collectSchemaRefsInternal(ap.A.Schema(), refSet)
	}

	// allOf / oneOf / anyOf / not
	for _, group := range [][]*base.SchemaProxy{schema.AllOf, schema.OneOf, schema.AnyOf, {schema.Not}} {
		for _, sp := range group {
			if sp == nil {
				continue
			}
			if sRef := sp.GoLow().GetReference(); sRef != "" {
				if !refSet[sRef] {
					refSet[sRef] = true
				} else {
					continue
				}
			}
			collectSchemaRefsInternal(sp.Schema(), refSet)
		}
	}

	// x-extensions
	for _, extValue := range extractExtensions(schema.Extensions) {
		if extSlice, ok := extValue.([]any); ok {
			for _, v := range extSlice {
				if kv, ok := v.(keyValue[string, string]); ok && kv.key == "$ref" {
					refSet[kv.value] = true
				}
			}
		}
	}
}

func getComponentKeys(component iter.Seq[string]) []string {
	keys := make([]string, 0)
	for k := range component {
		keys = append(keys, k)
	}
	return keys
}
