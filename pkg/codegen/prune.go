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
	"log/slog"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func pruneSchema(model *v3high.Document) error {
	// Aggressively remove everything we don't generate code for
	slog.Debug("Pruning: removing webhooks, security schemes, callbacks, component examples, links")
	model.Webhooks = nil
	if model.Components != nil {
		// Set to nil - we don't generate code for these
		model.Components.SecuritySchemes = nil
		model.Components.Callbacks = nil
		model.Components.Examples = nil
		model.Components.Links = nil
	}

	// Iteratively prune unreferenced components
	iteration := 0
	maxIterations := 1000

	// Iteratively prune unreferenced components until nothing left to remove
	for {
		iteration++
		if iteration > maxIterations {
			return fmt.Errorf("pruning exceeded maximum iterations (%d), possible infinite loop", maxIterations)
		}

		refs := findOperationRefs(model)
		slog.Debug("Found operation refs", "count", len(refs), "iteration", iteration)

		countRemoved := removeOrphanedComponents(model, refs)
		slog.Debug("Removed orphaned components", "count", countRemoved, "iteration", iteration)

		if countRemoved < 1 {
			// Done pruning
			return nil
		}
	}
}

func removeOrphanedComponents(model *v3high.Document, refs map[string]bool) int {
	if model.Components == nil {
		return 0
	}

	countRemoved := 0

	if model.Components.Schemas != nil {
		for _, key := range getComponentKeys(model.Components.Schemas.KeysFromOldest()) {
			ref := fmt.Sprintf("#/components/schemas/%s", key)
			if !refs[ref] {
				countRemoved++
				model.Components.Schemas.Delete(key)
			}
		}
	}

	if model.Components.Parameters != nil {
		for _, key := range getComponentKeys(model.Components.Parameters.KeysFromOldest()) {
			ref := fmt.Sprintf("#/components/parameters/%s", key)
			if !refs[ref] {
				countRemoved++
				model.Components.Parameters.Delete(key)
			}
		}
	}

	if model.Components.RequestBodies != nil {
		for _, key := range getComponentKeys(model.Components.RequestBodies.KeysFromOldest()) {
			ref := fmt.Sprintf("#/components/requestBodies/%s", key)
			if !refs[ref] {
				countRemoved++
				model.Components.RequestBodies.Delete(key)
			}
		}
	}

	if model.Components.Responses != nil {
		for _, key := range getComponentKeys(model.Components.Responses.KeysFromOldest()) {
			ref := fmt.Sprintf("#/components/responses/%s", key)
			if !refs[ref] {
				countRemoved++
				model.Components.Responses.Delete(key)
			}
		}
	}

	if model.Components.Headers != nil {
		for _, key := range getComponentKeys(model.Components.Headers.KeysFromOldest()) {
			ref := fmt.Sprintf("#/components/headers/%s", key)
			if !refs[ref] {
				countRemoved++
				model.Components.Headers.Delete(key)
			}
		}
	}

	// Note: Links, Callbacks, Examples are set to nil in pruneSchema, so we don't need to prune them here

	return countRemoved
}

func findOperationRefs(model *v3high.Document) map[string]bool {
	refSet := make(map[string]bool)

	if model.Paths == nil || model.Paths.PathItems == nil {
		return refSet
	}

	// Walk all operations and collect refs
	for _, pathItem := range model.Paths.PathItems.FromOldest() {
		// Collect path-level parameters
		for _, param := range pathItem.Parameters {
			collectRefFromProxy(param, refSet, model)
		}

		// Collect operation-level refs
		for _, op := range pathItem.GetOperations().FromOldest() {
			// Request body
			if op.RequestBody != nil {
				collectRefFromProxy(op.RequestBody, refSet, model)
			}

			// Parameters
			for _, param := range op.Parameters {
				collectRefFromProxy(param, refSet, model)
			}

			// Responses
			if op.Responses != nil {
				if op.Responses.Default != nil {
					collectRefFromProxy(op.Responses.Default, refSet, model)
				}
				for _, resp := range op.Responses.Codes.FromOldest() {
					collectRefFromProxy(resp, refSet, model)
				}
			}
		}
	}

	// Walk component parameters/requestBodies/responses/headers to collect schema refs
	// These components are only kept if referenced by operations (collected above)
	// But we need to walk them to find schemas they reference
	if model.Components != nil {
		if model.Components.Parameters != nil {
			for _, param := range model.Components.Parameters.FromOldest() {
				collectRefFromProxy(param, refSet, model)
			}
		}
		if model.Components.RequestBodies != nil {
			for _, reqBody := range model.Components.RequestBodies.FromOldest() {
				collectRefFromProxy(reqBody, refSet, model)
			}
		}
		if model.Components.Responses != nil {
			for _, resp := range model.Components.Responses.FromOldest() {
				collectRefFromProxy(resp, refSet, model)
			}
		}
		if model.Components.Headers != nil {
			for _, header := range model.Components.Headers.FromOldest() {
				collectRefFromProxy(header, refSet, model)
			}
		}

		// Walk component schemas that are already in refSet to collect refs to other schemas (for composition)
		// This is done iteratively to handle transitive references
		if model.Components.Schemas != nil {
			// Keep expanding until no new refs are added
			for {
				prevSize := len(refSet)
				for schemaName, schemaProxy := range model.Components.Schemas.FromOldest() {
					if schemaProxy == nil {
						continue
					}
					schemaRef := fmt.Sprintf("#/components/schemas/%s", schemaName)
					// Only process schemas that are already in the refSet
					if !refSet[schemaRef] {
						continue
					}
					// Check if the schema proxy itself is a $ref to another schema
					if targetRef := schemaProxy.GoLow().GetReference(); targetRef != "" {
						refSet[targetRef] = true
					}
					// Collect refs from the schema's content (allOf, oneOf, anyOf, properties, etc.)
					collectSchemaRefs(schemaProxy.Schema(), refSet, model)
				}
				// If no new refs were added, we're done
				if len(refSet) == prevSize {
					break
				}
			}
		}
	}

	slog.Debug("All collected refs", "count", len(refSet))
	return refSet
}

// addParentSchemaRef adds the parent schema reference if the given ref is a property reference
// e.g., if ref is "#/components/schemas/Foo/properties/bar", also add "#/components/schemas/Foo"
func addParentSchemaRef(ref string, refSet map[string]bool) {
	// Check if this is a property reference pattern: #/components/schemas/{name}/properties/{prop}
	if strings.Contains(ref, "/properties/") {
		// Extract the parent schema reference
		parts := strings.Split(ref, "/properties/")
		if len(parts) > 0 {
			parentRef := parts[0]
			if !refSet[parentRef] {
				refSet[parentRef] = true
			}
		}
	}
}

func collectSchemaRefs(schema *base.Schema, refSet map[string]bool, model *v3high.Document) {
	if schema == nil {
		return
	}

	// Check if this schema is a $ref
	if ref := schema.GoLow().GetReference(); ref != "" {
		if refSet[ref] {
			return
		}
		refSet[ref] = true
		// Also add parent schema if this is a property reference
		// e.g., if ref is "#/components/schemas/Foo/properties/bar", also add "#/components/schemas/Foo"
		addParentSchemaRef(ref, refSet)

		if resolveComponentSchemaRef(ref, refSet, model) {
			return
		}
	}

	// Traverse object properties
	if schema.Properties != nil {
		for _, prop := range schema.Properties.FromOldest() {
			if prop == nil {
				continue
			}
			if pRef := prop.GoLow().GetReference(); pRef != "" {
				if refSet[pRef] {
					continue
				}
				refSet[pRef] = true
				addParentSchemaRef(pRef, refSet)
				if resolveComponentSchemaRef(pRef, refSet, model) {
					continue
				}
			}
			collectSchemaRefs(prop.Schema(), refSet, model)
		}
	}

	// Traverse array items
	if items := schema.Items; items != nil && items.IsA() && items.A != nil {
		if iRef := items.A.GoLow().GetReference(); iRef != "" {
			if refSet[iRef] {
				return
			}
			refSet[iRef] = true
			if resolveComponentSchemaRef(iRef, refSet, model) {
				return
			}
		}
		collectSchemaRefs(items.A.Schema(), refSet, model)
	}

	// Traverse additionalProperties
	if ap := schema.AdditionalProperties; ap != nil && ap.IsA() && ap.A != nil {
		if aRef := ap.A.GoLow().GetReference(); aRef != "" {
			if refSet[aRef] {
				return
			}
			refSet[aRef] = true
			if resolveComponentSchemaRef(aRef, refSet, model) {
				return
			}
		}
		collectSchemaRefs(ap.A.Schema(), refSet, model)
	}

	// allOf / oneOf / anyOf / not
	for _, group := range [][]*base.SchemaProxy{schema.AllOf, schema.OneOf, schema.AnyOf, {schema.Not}} {
		for _, sp := range group {
			if sp == nil {
				continue
			}
			if sRef := sp.GoLow().GetReference(); sRef != "" {
				if refSet[sRef] {
					continue
				}
				refSet[sRef] = true
				if resolveComponentSchemaRef(sRef, refSet, model) {
					continue
				}
			}
			collectSchemaRefs(sp.Schema(), refSet, model)
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

// collectRefFromProxy collects the $ref from a proxy object and all schema refs from its content
// This works for Parameters, RequestBodies, Responses, Headers - anything with GoLow().GetReference()
// When model is provided, component schema references are resolved from the model (which may have been mutated)
func collectRefFromProxy(proxy any, refSet map[string]bool, model *v3high.Document) {
	if proxy == nil {
		return
	}

	// Collect the proxy's own $ref (e.g., $ref to component parameter)
	// Try different types that have GoLow().GetReference()
	switch p := proxy.(type) {
	case *v3high.Parameter:
		if ref := p.GoLow().GetReference(); ref != "" {
			slog.Debug("Collected parameter ref", "ref", ref)
			refSet[ref] = true
		}
	case *v3high.RequestBody:
		if ref := p.GoLow().GetReference(); ref != "" {
			refSet[ref] = true
		}
	case *v3high.Response:
		if ref := p.GoLow().GetReference(); ref != "" {
			refSet[ref] = true
		}
	case *v3high.Header:
		if ref := p.GoLow().GetReference(); ref != "" {
			refSet[ref] = true
		}
	}

	// Collect schema refs from the proxy's content
	switch v := proxy.(type) {
	case *v3high.Parameter:
		collectSchemaProxy(v.Schema, refSet, model)
		if v.Content != nil {
			for _, mediaType := range v.Content.FromOldest() {
				collectSchemaProxy(mediaType.Schema, refSet, model)
			}
		}

	case *v3high.RequestBody:
		if v.Content != nil {
			for _, mediaType := range v.Content.FromOldest() {
				collectSchemaProxy(mediaType.Schema, refSet, model)
			}
		}

	case *v3high.Response:
		if v.Content != nil {
			for _, mediaType := range v.Content.FromOldest() {
				collectSchemaProxy(mediaType.Schema, refSet, model)
			}
		}
		if v.Headers != nil {
			for _, header := range v.Headers.FromOldest() {
				collectRefFromProxy(header, refSet, model)
			}
		}

	case *v3high.Header:
		collectSchemaProxy(v.Schema, refSet, model)
		if v.Content != nil {
			for _, mediaType := range v.Content.FromOldest() {
				collectSchemaProxy(mediaType.Schema, refSet, model)
			}
		}
	}
}

// collectSchemaProxy collects refs from a SchemaProxy (both the proxy's own $ref and its schema content)
// When model is provided, component schema references are resolved from the model (which may have been mutated)
// instead of following the low-level reference which may contain stale data.
func collectSchemaProxy(schemaProxy *base.SchemaProxy, refSet map[string]bool, model *v3high.Document) {
	if schemaProxy == nil {
		return
	}
	// Collect the proxy's own $ref
	if ref := schemaProxy.GoLow().GetReference(); ref != "" {
		refSet[ref] = true
		if resolveComponentSchemaRef(ref, refSet, model) {
			return
		}
	}
	// Collect refs from the schema's content
	collectSchemaRefs(schemaProxy.Schema(), refSet, model)
}

// resolveComponentSchemaRef checks if ref is a component schema reference and resolves it from the model.
// Returns true if the ref was resolved from the model (caller should skip further processing).
func resolveComponentSchemaRef(ref string, refSet map[string]bool, model *v3high.Document) bool {
	if model == nil || !strings.HasPrefix(ref, "#/components/schemas/") {
		return false
	}

	schemaName := strings.TrimPrefix(ref, "#/components/schemas/")
	if model.Components == nil || model.Components.Schemas == nil {
		return false
	}

	if modelSchema, ok := model.Components.Schemas.Get(schemaName); ok && modelSchema != nil {
		collectSchemaRefs(modelSchema.Schema(), refSet, model)
		return true
	}

	return false
}
