package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeTracker_Register(t *testing.T) {
	r := newTypeTracker()

	td := TypeDefinition{Name: "Status", JsonName: "status"}
	r.register(td, "#/components/schemas/status")

	assert.Equal(t, 1, r.Size())
	assert.True(t, r.Exists("Status"))
}

func TestTypeTracker_LookupByName(t *testing.T) {
	r := newTypeTracker()

	td := TypeDefinition{Name: "Status", JsonName: "status"}
	r.register(td, "#/components/schemas/status")

	found, ok := r.LookupByName("Status")
	assert.True(t, ok)
	assert.Equal(t, "Status", found.Name)

	_, ok = r.LookupByName("NotExists")
	assert.False(t, ok)
}

func TestTypeTracker_LookupByRef(t *testing.T) {
	r := newTypeTracker()

	td := TypeDefinition{Name: "Status", JsonName: "status"}
	r.register(td, "#/components/schemas/status")

	name, ok := r.LookupByRef("#/components/schemas/status")
	assert.True(t, ok)
	assert.Equal(t, "Status", name)

	_, ok = r.LookupByRef("#/components/schemas/not-exists")
	assert.False(t, ok)
}

func TestTypeTracker_GenerateUniqueName(t *testing.T) {
	r := newTypeTracker()

	// First use of name - no conflict
	name := r.generateUniqueName("Status")
	assert.Equal(t, "Status", name)

	// Register it
	r.register(TypeDefinition{Name: "Status"}, "")

	// Second use - should get Status1
	name = r.generateUniqueName("Status")
	assert.Equal(t, "Status1", name)

	// Register it
	r.register(TypeDefinition{Name: "Status1"}, "")

	// Third use - should get Status2
	name = r.generateUniqueName("Status")
	assert.Equal(t, "Status2", name)
}

func TestTypeTracker_GenerateUniqueNameWithSuffixes(t *testing.T) {
	r := newTypeTracker()

	// Register base name
	r.register(TypeDefinition{Name: "Response"}, "")

	// With suffixes, should try ResponseJSON first
	name := r.generateUniqueNameWithSuffixes("Response", []string{"JSON", "Text"})
	assert.Equal(t, "ResponseJSON", name)

	// Register it
	r.register(TypeDefinition{Name: "ResponseJSON"}, "")

	// Next should try ResponseText
	name = r.generateUniqueNameWithSuffixes("Response", []string{"JSON", "Text"})
	assert.Equal(t, "ResponseText", name)

	// Register it
	r.register(TypeDefinition{Name: "ResponseText"}, "")

	// All suffixes taken, should fall back to numeric
	name = r.generateUniqueNameWithSuffixes("Response", []string{"JSON", "Text"})
	assert.Equal(t, "Response1", name)
}

func TestTypeTracker_WithDefaultSuffixes(t *testing.T) {
	r := newTypeTracker().withDefaultSuffixes([]string{"JSON", "Text"})

	// Register base name
	r.register(TypeDefinition{Name: "Response"}, "")

	// GenerateUniqueName should use default suffixes
	name := r.generateUniqueName("Response")
	assert.Equal(t, "ResponseJSON", name)
}

func TestTypeTracker_RefMapping(t *testing.T) {
	r := newTypeTracker()

	// Simulate schema/parameter name collision
	// Schema "status" gets registered first as "Status"
	r.register(TypeDefinition{Name: "Status", JsonName: "status"}, "#/components/schemas/status")

	// Parameter "status" would conflict, so it gets renamed
	paramName := r.generateUniqueName("Status")
	assert.Equal(t, "Status1", paramName)
	r.register(TypeDefinition{Name: paramName, JsonName: "status"}, "#/components/parameters/status")

	// Now when resolving refs, we get the correct Go type names
	schemaType, ok := r.LookupByRef("#/components/schemas/status")
	assert.True(t, ok)
	assert.Equal(t, "Status", schemaType)

	paramType, ok := r.LookupByRef("#/components/parameters/status")
	assert.True(t, ok)
	assert.Equal(t, "Status1", paramType)
}
