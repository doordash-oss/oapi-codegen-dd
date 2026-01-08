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
	"slices"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

func pruneSchema(doc libopenapi.Document, allowedExtensions map[string]bool) (libopenapi.Document, error) {
	firstIteration := true
	iteration := 0

	// Safety limit to prevent infinite loops (increased for large specs)
	maxIterations := 1000

	for {
		iteration++
		if iteration > maxIterations {
			return nil, fmt.Errorf("pruning exceeded maximum iterations (%d), possible infinite loop - this may indicate a circular reference issue", maxIterations)
		}

		model, err := doc.BuildV3Model()
		if err != nil {
			return nil, fmt.Errorf("error building model: %w", err)
		}

		// On first iteration, clean up things we don't need
		if firstIteration {
			// Filter extensions from root document
			filterExtensions(model.Model.Extensions, allowedExtensions)

			// Filter extensions from info
			if model.Model.Info != nil {
				filterExtensions(model.Model.Info.Extensions, allowedExtensions)
				if model.Model.Info.Contact != nil {
					filterExtensions(model.Model.Info.Contact.Extensions, allowedExtensions)
				}
				if model.Model.Info.License != nil {
					filterExtensions(model.Model.Info.License.Extensions, allowedExtensions)
				}
			}

			// Filter extensions from servers
			for _, server := range model.Model.Servers {
				if server != nil {
					filterExtensions(server.Extensions, allowedExtensions)
					if server.Variables != nil {
						for varPair := server.Variables.First(); varPair != nil; varPair = varPair.Next() {
							variable := varPair.Value()
							if variable != nil {
								filterExtensions(variable.Extensions, allowedExtensions)
							}
						}
					}
				}
			}

			// Filter extensions from tags
			for _, tag := range model.Model.Tags {
				if tag != nil {
					filterExtensions(tag.Extensions, allowedExtensions)
				}
			}

			// Filter extensions from external docs
			if model.Model.ExternalDocs != nil {
				filterExtensions(model.Model.ExternalDocs.Extensions, allowedExtensions)
			}

			// Remove webhooks - we don't generate code for them
			model.Model.Webhooks = nil

			// Remove things we don't use from components
			if model.Model.Components != nil {
				model.Model.Components.SecuritySchemes = nil
				model.Model.Components.Callbacks = nil
			}

			// Clean up extensions and examples from paths and operations using low-level iteration
			// to avoid triggering reference resolution which can cause infinite loops
			if model.Model.Paths != nil {
				// Filter extensions from Paths object itself
				filterExtensions(model.Model.Paths.Extensions, allowedExtensions)

				if model.Model.Paths.PathItems != nil {
					for pair := model.Model.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
						pathItem := pair.Value()
						// Filter extensions from path item (keep only allowed ones)
						filterExtensions(pathItem.Extensions, allowedExtensions)

						// Clean up path-level parameters
						for _, param := range pathItem.Parameters {
							cleanupParameter(param, allowedExtensions)
						}

						// Filter extensions from operations
						for opPair := pathItem.GetOperations().First(); opPair != nil; opPair = opPair.Next() {
							op := opPair.Value()
							filterExtensions(op.Extensions, allowedExtensions)

							// Clean up parameters
							for _, param := range op.Parameters {
								cleanupParameter(param, allowedExtensions)
							}

							// Clean up request body
							cleanupRequestBody(op.RequestBody, allowedExtensions)

							// Clean up responses
							if op.Responses != nil {
								filterExtensions(op.Responses.Extensions, allowedExtensions)
								cleanupResponse(op.Responses.Default, allowedExtensions)
								if op.Responses.Codes != nil {
									for respPair := op.Responses.Codes.First(); respPair != nil; respPair = respPair.Next() {
										cleanupResponse(respPair.Value(), allowedExtensions)
									}
								}
							}
						}
					}
				}
			}

			// Clean up examples and extensions from components
			if model.Model.Components != nil {
				// Filter extensions from components object itself
				filterExtensions(model.Model.Components.Extensions, allowedExtensions)

				// Remove ALL component examples - we don't generate code for them
				// and they can contain complex structures with extensions
				// We need to delete all keys from the Examples map, not just set it to nil
				if model.Model.Components.Examples != nil {
					for _, key := range getComponentKeys(model.Model.Components.Examples.KeysFromOldest()) {
						model.Model.Components.Examples.Delete(key)
					}
				}

				// Filter extensions from schemas (recursively)
				if model.Model.Components.Schemas != nil {
					for pair := model.Model.Components.Schemas.First(); pair != nil; pair = pair.Next() {
						schemaProxy := pair.Value()
						if schemaProxy != nil && schemaProxy.Schema() != nil {
							cleanupSchemaExtensions(schemaProxy.Schema(), allowedExtensions)
						}
					}
				}

				// Clean up parameters
				if model.Model.Components.Parameters != nil {
					for pair := model.Model.Components.Parameters.First(); pair != nil; pair = pair.Next() {
						cleanupParameter(pair.Value(), allowedExtensions)
					}
				}

				// Clean up headers
				if model.Model.Components.Headers != nil {
					for pair := model.Model.Components.Headers.First(); pair != nil; pair = pair.Next() {
						cleanupHeader(pair.Value(), allowedExtensions)
					}
				}

				// Clean up responses
				if model.Model.Components.Responses != nil {
					for pair := model.Model.Components.Responses.First(); pair != nil; pair = pair.Next() {
						cleanupResponse(pair.Value(), allowedExtensions)
					}
				}

				// Clean up request bodies
				if model.Model.Components.RequestBodies != nil {
					for pair := model.Model.Components.RequestBodies.First(); pair != nil; pair = pair.Next() {
						cleanupRequestBody(pair.Value(), allowedExtensions)
					}
				}

				// Clean up links
				if model.Model.Components.Links != nil {
					for pair := model.Model.Components.Links.First(); pair != nil; pair = pair.Next() {
						cleanupLink(pair.Value(), allowedExtensions)
					}
				}
			}

			firstIteration = false

			// Continue to next iteration to start pruning
			continue
		}

		refs := findOperationRefs(&model.Model)
		slog.Debug("Found operation refs", "count", len(refs), "iteration", iteration)
		countRemoved := removeOrphanedComponents(&model.Model, refs)
		slog.Debug("Removed orphaned components", "count", countRemoved, "iteration", iteration)

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

	// Note: Component examples are removed entirely during cleanup (first iteration)
	// by deleting all keys from the Examples map
	// We don't generate code for them and they can contain complex structures.

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

				for _, header := range op.Responses.Default.Headers.FromOldest() {
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

				// Collect link refs from default response
				for _, link := range op.Responses.Default.Links.FromOldest() {
					if link == nil {
						continue
					}
					lRef := link.GoLow().GetReference()
					if lRef != "" {
						refSet[lRef] = true
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

					// Collect link refs from response
					for _, link := range resp.Links.FromOldest() {
						if link == nil {
							continue
						}
						lRef := link.GoLow().GetReference()
						if lRef != "" {
							refSet[lRef] = true
						}
					}
				}
			}

			// Collect refs from operation callbacks
			// Callbacks contain operations that can reference schemas, parameters, etc.
			if op.Callbacks != nil {
				for _, callback := range op.Callbacks.FromOldest() {
					if callback == nil {
						continue
					}
					// Collect the callback reference itself
					cbRef := callback.GoLow().GetReference()
					if cbRef != "" {
						refSet[cbRef] = true
					}

					// Iterate through callback expressions (e.g., "{$request.body#/callbackUrl}")
					for _, expression := range callback.Expression.FromOldest() {
						if expression == nil {
							continue
						}

						// Each expression contains operations (like a path item)
						for _, cbOp := range expression.GetOperations().FromOldest() {
							if cbOp == nil {
								continue
							}

							// Collect refs from callback operation request body
							if cbOp.RequestBody != nil {
								cbReqBodyRef := cbOp.RequestBody.GoLow().GetReference()
								if cbReqBodyRef != "" {
									refSet[cbReqBodyRef] = true
								}
								for _, mediaType := range cbOp.RequestBody.Content.FromOldest() {
									if mediaType.Schema != nil {
										if schemaRef := mediaType.Schema.GoLow().GetReference(); schemaRef != "" {
											refSet[schemaRef] = true
										}
										collectSchemaRefs(mediaType.Schema.Schema(), refSet)
									}
								}
							}

							// Collect refs from callback operation parameters
							for _, cbParam := range cbOp.Parameters {
								if cbParam == nil {
									continue
								}
								cbParamRef := cbParam.GoLow().GetReference()
								if cbParamRef != "" {
									refSet[cbParamRef] = true
								}
								if cbParam.Schema != nil {
									if schemaRef := cbParam.Schema.GoLow().GetReference(); schemaRef != "" {
										refSet[schemaRef] = true
									}
									collectSchemaRefs(cbParam.Schema.Schema(), refSet)
								}
							}

							// Collect refs from callback operation responses
							if cbOp.Responses != nil {
								for _, cbResp := range cbOp.Responses.Codes.FromOldest() {
									if cbResp == nil {
										continue
									}
									cbRespRef := cbResp.GoLow().GetReference()
									if cbRespRef != "" {
										refSet[cbRespRef] = true
									}
									for _, mediaType := range cbResp.Content.FromOldest() {
										if mediaType.Schema != nil {
											if schemaRef := mediaType.Schema.GoLow().GetReference(); schemaRef != "" {
												refSet[schemaRef] = true
											}
											collectSchemaRefs(mediaType.Schema.Schema(), refSet)
										}
									}
								}
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
					// Collect the $ref from the SchemaProxy itself
					if schemaRef := mediaType.Schema.GoLow().GetReference(); schemaRef != "" {
						refSet[schemaRef] = true
					}
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

			// Collect link refs from component responses
			for _, link := range resp.Links.FromOldest() {
				if link == nil {
					continue
				}
				lRef := link.GoLow().GetReference()
				if lRef != "" {
					refSet[lRef] = true
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

	// Collect schema refs from component schemas themselves
	// This handles cases where a component schema is just a $ref to another schema
	if model.Components != nil && model.Components.Schemas != nil {
		for _, schemaProxy := range model.Components.Schemas.FromOldest() {
			if schemaProxy == nil {
				continue
			}
			// Check if the schema proxy itself is a $ref
			if schemaRef := schemaProxy.GoLow().GetReference(); schemaRef != "" {
				refSet[schemaRef] = true
			}
			// Also collect refs from the schema's content
			collectSchemaRefs(schemaProxy.Schema(), refSet)
		}
	}

	// Collect refs from component request bodies themselves
	// This handles cases where a component request body is just a $ref to another request body
	if model.Components != nil && model.Components.RequestBodies != nil {
		for _, reqBody := range model.Components.RequestBodies.FromOldest() {
			if reqBody == nil {
				continue
			}
			// Check if the request body itself is a $ref
			if reqBodyRef := reqBody.GoLow().GetReference(); reqBodyRef != "" {
				refSet[reqBodyRef] = true
			}
		}
	}

	// Collect refs from component responses themselves
	// This handles cases where a component response is just a $ref to another response
	if model.Components != nil && model.Components.Responses != nil {
		for _, resp := range model.Components.Responses.FromOldest() {
			if resp == nil {
				continue
			}
			// Check if the response itself is a $ref
			if respRef := resp.GoLow().GetReference(); respRef != "" {
				refSet[respRef] = true
			}
		}
	}

	// Collect refs from component headers themselves
	// This handles cases where a component header is just a $ref to another header
	if model.Components != nil && model.Components.Headers != nil {
		for _, header := range model.Components.Headers.FromOldest() {
			if header == nil {
				continue
			}
			// Check if the header itself is a $ref
			if headerRef := header.GoLow().GetReference(); headerRef != "" {
				refSet[headerRef] = true
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

// filterExtensions removes extensions that are not in the allowedExtensions map
func filterExtensions(extensions *orderedmap.Map[string, *yaml.Node], allowedExtensions map[string]bool) {
	if extensions == nil || extensions.Len() == 0 {
		return
	}

	// Collect keys to delete (can't delete while iterating)
	var keysToDelete []string
	for pair := extensions.First(); pair != nil; pair = pair.Next() {
		key := pair.Key()
		if !allowedExtensions[key] {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Delete the disallowed extensions
	for _, key := range keysToDelete {
		extensions.Delete(key)
	}
}

// cleanupMediaType removes examples and filters extensions from a media type
func cleanupMediaType(mediaType *v3high.MediaType, allowedExtensions map[string]bool) {
	if mediaType == nil {
		return
	}
	filterExtensions(mediaType.Extensions, allowedExtensions)
	mediaType.Examples = nil
	// Clean up schema extensions recursively
	if mediaType.Schema != nil && mediaType.Schema.Schema() != nil {
		cleanupSchemaExtensions(mediaType.Schema.Schema(), allowedExtensions)
	}
}

// cleanupParameter removes examples and filters extensions from a parameter
func cleanupParameter(param *v3high.Parameter, allowedExtensions map[string]bool) {
	if param == nil {
		return
	}
	filterExtensions(param.Extensions, allowedExtensions)
	param.Examples = nil
	// Clean up schema extensions recursively
	if param.Schema != nil && param.Schema.Schema() != nil {
		cleanupSchemaExtensions(param.Schema.Schema(), allowedExtensions)
	}
	// Clean up content schemas
	if param.Content != nil {
		for pair := param.Content.First(); pair != nil; pair = pair.Next() {
			cleanupMediaType(pair.Value(), allowedExtensions)
		}
	}
}

// cleanupHeader removes examples and filters extensions from a header
func cleanupHeader(header *v3high.Header, allowedExtensions map[string]bool) {
	if header == nil {
		return
	}
	filterExtensions(header.Extensions, allowedExtensions)
	header.Examples = nil
	// Clean up schema extensions recursively
	if header.Schema != nil && header.Schema.Schema() != nil {
		cleanupSchemaExtensions(header.Schema.Schema(), allowedExtensions)
	}
	// Clean up content schemas
	if header.Content != nil {
		for pair := header.Content.First(); pair != nil; pair = pair.Next() {
			cleanupMediaType(pair.Value(), allowedExtensions)
		}
	}
}

// cleanupRequestBody removes examples and filters extensions from a request body and its content
func cleanupRequestBody(requestBody *v3high.RequestBody, allowedExtensions map[string]bool) {
	if requestBody == nil {
		return
	}
	filterExtensions(requestBody.Extensions, allowedExtensions)
	if requestBody.Content != nil {
		for mediaPair := requestBody.Content.First(); mediaPair != nil; mediaPair = mediaPair.Next() {
			cleanupMediaType(mediaPair.Value(), allowedExtensions)
		}
	}
}

// cleanupResponse removes examples and filters extensions from a response, its content, headers, and links
func cleanupResponse(response *v3high.Response, allowedExtensions map[string]bool) {
	if response == nil {
		return
	}
	filterExtensions(response.Extensions, allowedExtensions)

	// Clean up content
	if response.Content != nil {
		for mediaPair := response.Content.First(); mediaPair != nil; mediaPair = mediaPair.Next() {
			cleanupMediaType(mediaPair.Value(), allowedExtensions)
		}
	}

	// Clean up headers
	if response.Headers != nil {
		for headerPair := response.Headers.First(); headerPair != nil; headerPair = headerPair.Next() {
			cleanupHeader(headerPair.Value(), allowedExtensions)
		}
	}

	// Clean up links
	if response.Links != nil {
		for linkPair := response.Links.First(); linkPair != nil; linkPair = linkPair.Next() {
			cleanupLink(linkPair.Value(), allowedExtensions)
		}
	}
}

// cleanupLink filters extensions from a link
func cleanupLink(link *v3high.Link, allowedExtensions map[string]bool) {
	if link == nil {
		return
	}
	filterExtensions(link.Extensions, allowedExtensions)
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
		// Also add parent schema if this is a property reference
		// e.g., if ref is "#/components/schemas/Foo/properties/bar", also add "#/components/schemas/Foo"
		addParentSchemaRef(ref, refSet)
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
					// Also add parent schema if this is a property reference
					addParentSchemaRef(pRef, refSet)
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
		// Handle array format: [{$ref: "..."}]
		if extSlice, ok := extValue.([]any); ok {
			for _, v := range extSlice {
				if kv, ok := v.(keyValue[string, string]); ok && kv.key == "$ref" {
					refSet[kv.value] = true
				}
			}
		}
		// Handle map format: {$ref: "..."}
		if extMap, ok := extValue.(map[string]any); ok {
			if ref, ok := extMap["$ref"].(string); ok {
				refSet[ref] = true
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

// cleanupSchemaExtensions filters extensions and removes examples from a single schema (non-recursive)
// The caller is responsible for iterating through all schemas in the document
func cleanupSchemaExtensions(schema *base.Schema, allowedExtensions map[string]bool) {
	if schema == nil {
		return
	}

	// Filter extensions from this schema
	filterExtensions(schema.Extensions, allowedExtensions)

	// Remove examples (plural) from schema - we don't generate code for them
	schema.Examples = nil

	// Remove example (singular) if it's a $ref - this is invalid OpenAPI
	// The example field should only contain inline values, not references
	// We need to clear the Content array to remove the $ref
	if schema.Example != nil && schema.Example.Kind == yaml.MappingNode {
		// Check if this mapping node contains a $ref key (which is invalid for example field)
		for i := 0; i < len(schema.Example.Content); i += 2 {
			if i+1 < len(schema.Example.Content) {
				keyNode := schema.Example.Content[i]
				if keyNode.Value == "$ref" {
					// Invalid: example field cannot contain $ref
					// Clear the content to remove the reference
					slog.Debug("Removing invalid example $ref from schema")
					schema.Example.Content = nil
					schema.Example.Kind = yaml.ScalarNode
					schema.Example.Value = ""
					break
				}
			}
		}
	}
}
