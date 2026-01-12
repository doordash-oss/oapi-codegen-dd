package union

import (
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUserBody_User_Validation(t *testing.T) {
	t.Run("valid user", func(t *testing.T) {
		score := float32(50.5)
		user := CreateUserBody_User{
			ID:    100,
			Score: &score,
		}
		err := user.Validate()
		assert.NoError(t, err)
	})

	t.Run("id below minimum", func(t *testing.T) {
		user := CreateUserBody_User{
			ID: 0,
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Equal(t, "ID is required", err.Error())
	})

	t.Run("id above maximum", func(t *testing.T) {
		user := CreateUserBody_User{
			ID: 1000000,
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Equal(t, "ID must be less than or equal to 999999", err.Error())
	})

	t.Run("score below minimum", func(t *testing.T) {
		score := float32(-1)
		user := CreateUserBody_User{
			ID:    100,
			Score: &score,
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Score must be greater than or equal to 0", err.Error())
	})

	t.Run("score above maximum", func(t *testing.T) {
		score := float32(101)
		user := CreateUserBody_User{
			ID:    100,
			Score: &score,
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Score must be less than or equal to 100", err.Error())
	})
}

func TestCreateUserBody_Pages_Validation(t *testing.T) {
	t.Run("valid with all fields", func(t *testing.T) {
		tag1 := "short"
		tag2 := "valid tag"

		anyOf := CreateUserBody_Pages_AnyOf{
			Either: runtime.NewEitherFromA[CreateUserBody_Pages_AnyOf_0, CreateUserBody_Pages_AnyOf_1](
				CreateUserBody_Pages_AnyOf_0{Offset: 10},
			),
		}

		oneOf := CreateUserBody_Pages_OneOf{
			Either: runtime.NewEitherFromA[CreateUserBody_Pages_OneOf_0, CreateUserBody_Pages_OneOf_1](
				CreateUserBody_Pages_OneOf_0{First: 1, Second: 2},
			),
		}

		pages := CreateUserBody_Pages_Item{
			Limit:                      100,
			Tag1:                       &tag1,
			Tag2:                       &tag2,
			CreateUserBody_Pages_AnyOf: &anyOf,
			CreateUserBody_Pages_OneOf: &oneOf,
		}
		err := pages.Validate()
		assert.NoError(t, err)
	})

	t.Run("limit below minimum", func(t *testing.T) {
		pages := CreateUserBody_Pages_Item{
			Limit: 0,
		}
		err := pages.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Limit is required", err.Error())
	})

	t.Run("limit above maximum", func(t *testing.T) {
		pages := CreateUserBody_Pages_Item{
			Limit: 1001,
		}
		err := pages.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Limit must be less than or equal to 1000", err.Error())
	})

	t.Run("tag1 exceeds maxLength", func(t *testing.T) {
		tag1 := "this is a very long tag that exceeds the maximum length of 50 characters"
		pages := CreateUserBody_Pages_Item{
			Limit: 100,
			Tag1:  &tag1,
		}
		err := pages.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Tag1 length must be less than or equal to 50", err.Error())
	})

	t.Run("tag2 below minLength", func(t *testing.T) {
		tag2 := ""
		pages := CreateUserBody_Pages_Item{
			Limit: 100,
			Tag2:  &tag2,
		}
		err := pages.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Tag2 length must be greater than or equal to 1", err.Error())
	})
}

func TestCreateUserBody_Pages_AnyOf_Validation(t *testing.T) {
	t.Run("valid offset", func(t *testing.T) {
		anyOf := CreateUserBody_Pages_AnyOf{
			Either: runtime.NewEitherFromA[CreateUserBody_Pages_AnyOf_0, CreateUserBody_Pages_AnyOf_1](
				CreateUserBody_Pages_AnyOf_0{Offset: 10},
			),
		}
		err := anyOf.Validate()
		assert.NoError(t, err)
	})

	t.Run("offset below minimum", func(t *testing.T) {
		anyOf := CreateUserBody_Pages_AnyOf{
			Either: runtime.NewEitherFromA[CreateUserBody_Pages_AnyOf_0, CreateUserBody_Pages_AnyOf_1](
				CreateUserBody_Pages_AnyOf_0{Offset: -1},
			),
		}
		err := anyOf.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Offset must be greater than or equal to 0", err.Error())
	})

	t.Run("valid query", func(t *testing.T) {
		anyOf := CreateUserBody_Pages_AnyOf{
			Either: runtime.NewEitherFromB[CreateUserBody_Pages_AnyOf_0, CreateUserBody_Pages_AnyOf_1](
				CreateUserBody_Pages_AnyOf_1{Query: "search term"},
			),
		}
		err := anyOf.Validate()
		assert.NoError(t, err)
	})

	t.Run("query below minLength", func(t *testing.T) {
		anyOf := CreateUserBody_Pages_AnyOf{
			Either: runtime.NewEitherFromB[CreateUserBody_Pages_AnyOf_0, CreateUserBody_Pages_AnyOf_1](
				CreateUserBody_Pages_AnyOf_1{Query: ""},
			),
		}
		err := anyOf.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Query is required", err.Error())
	})
}

func TestCreateUserBody_Pages_OneOf_Validation(t *testing.T) {
	t.Run("valid first and second", func(t *testing.T) {
		oneOf := CreateUserBody_Pages_OneOf{
			Either: runtime.NewEitherFromA[CreateUserBody_Pages_OneOf_0, CreateUserBody_Pages_OneOf_1](
				CreateUserBody_Pages_OneOf_0{First: 1, Second: 2},
			),
		}
		err := oneOf.Validate()
		assert.NoError(t, err)
	})

	t.Run("first below minimum", func(t *testing.T) {
		oneOf := CreateUserBody_Pages_OneOf{
			Either: runtime.NewEitherFromA[CreateUserBody_Pages_OneOf_0, CreateUserBody_Pages_OneOf_1](
				CreateUserBody_Pages_OneOf_0{First: 0, Second: 2},
			),
		}
		err := oneOf.Validate()
		assert.Error(t, err)
		assert.Equal(t, "First is required", err.Error())
	})

	t.Run("valid last", func(t *testing.T) {
		oneOf := CreateUserBody_Pages_OneOf{
			Either: runtime.NewEitherFromB[CreateUserBody_Pages_OneOf_0, CreateUserBody_Pages_OneOf_1](
				CreateUserBody_Pages_OneOf_1{Last: 5},
			),
		}
		err := oneOf.Validate()
		assert.NoError(t, err)
	})

	t.Run("last below minimum", func(t *testing.T) {
		oneOf := CreateUserBody_Pages_OneOf{
			Either: runtime.NewEitherFromB[CreateUserBody_Pages_OneOf_0, CreateUserBody_Pages_OneOf_1](
				CreateUserBody_Pages_OneOf_1{Last: 0},
			),
		}
		err := oneOf.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Last is required", err.Error())
	})
}

func TestCreateUserBody_Pages_Integration(t *testing.T) {
	t.Run("validates regular fields in types with union properties", func(t *testing.T) {
		// This test ensures that when a type has union properties,
		// both the union properties AND regular fields are validated
		tag1 := "this is way too long and should fail validation because it exceeds 50 characters"

		pages := CreateUserBody_Pages_Item{
			Limit: 100, // valid
			Tag1:  &tag1,
		}

		err := pages.Validate()
		require.Error(t, err)

		// Should fail on Tag1 validation
		assert.Contains(t, err.Error(), "Tag1")
	})

	t.Run("validates union properties", func(t *testing.T) {
		anyOf := CreateUserBody_Pages_AnyOf{
			Either: runtime.NewEitherFromA[CreateUserBody_Pages_AnyOf_0, CreateUserBody_Pages_AnyOf_1](
				CreateUserBody_Pages_AnyOf_0{Offset: -5}, // invalid
			),
		}

		pages := CreateUserBody_Pages_Item{
			Limit:                      100, // valid
			CreateUserBody_Pages_AnyOf: &anyOf,
		}

		err := pages.Validate()
		require.Error(t, err)

		// Should fail on Offset validation in the union
		assert.Contains(t, err.Error(), "Offset")
	})
}
