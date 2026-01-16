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

import "fmt"

// TypeTracker tracks type definitions and provides lookup by name or reference.
// It handles name conflicts by generating unique names and maintains a mapping
// from original OpenAPI references to Go type names.
type TypeTracker struct {
	// byName maps Go type name to TypeDefinition pointer
	byName map[string]*TypeDefinition

	// byRef maps original OpenAPI reference path to Go type name.
	// e.g., "#/components/schemas/status" -> "Status"
	// This allows looking up the actual Go type name when a ref is used,
	// even if the type was renamed due to conflicts.
	byRef map[string]string

	// counters tracks how many times a base name has been used for unique name generation
	counters map[string]int

	// defaultSuffixes are tried before falling back to numeric suffixes
	defaultSuffixes []string
}

// newTypeTracker creates a new TypeTracker.
func newTypeTracker() *TypeTracker {
	return &TypeTracker{
		byName:   make(map[string]*TypeDefinition),
		byRef:    make(map[string]string),
		counters: make(map[string]int),
	}
}

// withDefaultSuffixes sets the default suffixes to try before numeric suffixes.
func (r *TypeTracker) withDefaultSuffixes(suffixes []string) *TypeTracker {
	r.defaultSuffixes = suffixes
	return r
}

// register adds a type definition to the tracker.
// If ref is non-empty, it also maps the ref to the type name.
func (r *TypeTracker) register(td TypeDefinition, ref string) {
	r.byName[td.Name] = &td
	if ref != "" {
		r.byRef[ref] = td.Name
	}
}

// registerName registers a name in the tracker without a full type definition.
// This is used to reserve names (e.g., enum constant names) to prevent conflicts.
func (r *TypeTracker) registerName(name string) {
	if _, exists := r.byName[name]; !exists {
		r.byName[name] = &TypeDefinition{Name: name}
	}
}

// registerRef registers a ref to name mapping without a full type definition.
// This is used in the first pass to enable forward reference resolution.
func (r *TypeTracker) registerRef(ref, name string) {
	r.byRef[ref] = name
}

// LookupByName returns the TypeDefinition pointer for the given Go type name.
func (r *TypeTracker) LookupByName(name string) (*TypeDefinition, bool) {
	td, ok := r.byName[name]
	return td, ok
}

// LookupByRef returns the Go type name for the given OpenAPI reference path.
func (r *TypeTracker) LookupByRef(ref string) (string, bool) {
	name, ok := r.byRef[ref]
	return name, ok
}

// Exists checks if a type with the given name already exists.
func (r *TypeTracker) Exists(name string) bool {
	_, ok := r.byName[name]
	return ok
}

// generateUniqueName generates a unique type name based on the given base name.
// If the base name doesn't exist, it returns the base name.
// Otherwise, it tries default suffixes first, then appends numbers.
func (r *TypeTracker) generateUniqueName(baseName string) string {
	return r.generateUniqueNameWithSuffixes(baseName, r.defaultSuffixes)
}

// generateUniqueNameWithSuffixes generates a unique type name with custom suffixes.
// If the base name doesn't exist, it returns the base name.
// Otherwise, it tries the provided suffixes first, then appends numbers.
func (r *TypeTracker) generateUniqueNameWithSuffixes(baseName string, suffixes []string) string {
	if !r.Exists(baseName) {
		return baseName
	}

	// First, try each suffix without a counter
	for _, suffix := range suffixes {
		name := baseName + suffix
		if !r.Exists(name) {
			return name
		}
	}

	// If all suffixes are taken, use counter-based naming
	// Start from the current counter value for this base name
	counter := r.counters[baseName]
	if counter == 0 {
		counter = 1 // Start from 1 since base name is taken
	}

	for {
		name := fmt.Sprintf("%s%d", baseName, counter)
		if !r.Exists(name) {
			r.counters[baseName] = counter + 1
			return name
		}
		counter++
	}
}

// generateUniqueBaseName finds a unique base name such that baseName + suffix doesn't collide.
// It returns the original baseName if no collision, otherwise tries baseName1, baseName2, etc.
// This is useful when you need to generate a derived type name like {operationID}RequestOptions.
func (r *TypeTracker) generateUniqueBaseName(baseName, suffix string) string {
	derivedName := baseName + suffix
	if !r.Exists(derivedName) {
		return baseName
	}

	for counter := 1; ; counter++ {
		newBase := fmt.Sprintf("%s%d", baseName, counter)
		derivedName = newBase + suffix
		if !r.Exists(derivedName) {
			return newBase
		}
	}
}

// AsMap returns the internal map of type definitions.
func (r *TypeTracker) AsMap() map[string]*TypeDefinition {
	return r.byName
}

// Size returns the number of registered types.
func (r *TypeTracker) Size() int {
	return len(r.byName)
}
