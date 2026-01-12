package union

import (
	"encoding/json"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPetUnion_UnmarshalDog(t *testing.T) {
	// Test unmarshaling a Dog with explicit type field
	// This tests that the discriminator correctly routes to Dog (Either.A)
	dogJSON := `{"name": "Buddy", "type": "dog"}`

	var pet Pet
	err := json.Unmarshal([]byte(dogJSON), &pet)
	require.NoError(t, err)

	// Verify we got a Dog (Either.A)
	require.True(t, pet.Pet_OneOf.IsA(), "Expected Dog (A) but got Cat (B)")
	dog := pet.Pet_OneOf.A
	assert.Equal(t, "Buddy", dog.Name)
	assert.NotNil(t, dog.Type)
	assert.Equal(t, DogTypeDog, *dog.Type)
}

func TestPetUnion_UnmarshalDogWithoutType(t *testing.T) {
	// Test unmarshaling a Dog without the type field
	// Even though Dog.type is optional, the discriminator is required for the union
	// to determine which type to unmarshal into, so this should fail
	dogJSON := `{"name": "Max"}`

	var pet Pet
	err := json.Unmarshal([]byte(dogJSON), &pet)
	require.Error(t, err, "Expected error when discriminator field is missing")
	assert.Contains(t, err.Error(), "unknown discriminator value")
}

func TestPetUnion_UnmarshalCat(t *testing.T) {
	// Test unmarshaling a Cat with required type field
	// This tests that the discriminator correctly routes to Cat (Either.B)
	catJSON := `{"name": "Whiskers", "type": "cat"}`

	var pet Pet
	err := json.Unmarshal([]byte(catJSON), &pet)
	require.NoError(t, err)

	// Verify we got a Cat (Either.B)
	require.True(t, pet.Pet_OneOf.IsB(), "Expected Cat (B) but got Dog (A)")
	cat := pet.Pet_OneOf.B
	assert.Equal(t, "Whiskers", cat.Name)
	assert.Equal(t, CatTypeCat, cat.Type)
}

func TestPetUnion_MarshalDog(t *testing.T) {
	// Create a Dog and marshal it through the Pet union
	// This tests that marshaling a Dog includes the discriminator
	dogType := DogTypeDog
	dog := Dog{
		Name: "Buddy",
		Type: &dogType,
	}

	var pet Pet
	either := runtime.NewEitherFromA[Dog, Cat](dog)
	pet.Pet_OneOf = &Pet_OneOf{Either: either}

	data, err := json.Marshal(pet)
	require.NoError(t, err)

	// Verify the JSON contains the discriminator
	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Equal(t, "Buddy", result["name"])
	assert.Equal(t, "dog", result["type"])
}

func TestPetUnion_MarshalCat(t *testing.T) {
	// Create a Cat and marshal it through the Pet union
	// This tests that marshaling a Cat includes the discriminator
	cat := Cat{
		Name: "Whiskers",
		Type: CatTypeCat,
	}

	var pet Pet
	either := runtime.NewEitherFromB[Dog, Cat](cat)
	pet.Pet_OneOf = &Pet_OneOf{Either: either}

	data, err := json.Marshal(pet)
	require.NoError(t, err)

	// Verify the JSON contains the discriminator
	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Equal(t, "Whiskers", result["name"])
	assert.Equal(t, "cat", result["type"])
}
