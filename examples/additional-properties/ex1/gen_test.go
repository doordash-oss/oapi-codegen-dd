package gen

import (
	"testing"

	"github.com/doordash/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
)

func TestAlwaysValidates(t *testing.T) {
	// Items is []any and doesn't have a Validate method (can't validate 'any' types)
	assert.Nil(t, Location{}.Validate())
	assert.Nil(t, User{}.Validate())
	assert.Nil(t, Users{}.Validate())
	assert.Nil(t, Pick1{}.Validate())
}

func TestReferenceWithRequiredExtra_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		ref := "foo"
		obj := ReferenceWithRequiredExtra{
			Index: &ref,
			AdditionalProperties: map[string]string{
				"foo": "bar",
			},
		}
		assert.Nil(t, obj.Validate())
	})
}

func TestConfigWithMinProps_Validate(t *testing.T) {
	t.Run("valid - has 1 property", func(t *testing.T) {
		obj := ConfigWithMinProps{"key": "value"}
		assert.Nil(t, obj.Validate())
	})

	t.Run("valid - has multiple properties", func(t *testing.T) {
		obj := ConfigWithMinProps{"key1": "value1", "key2": "value2"}
		assert.Nil(t, obj.Validate())
	})

	t.Run("invalid - empty map", func(t *testing.T) {
		obj := ConfigWithMinProps{}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have at least 1 properties")
		// Check that it's a ValidationError
		var ve runtime.ValidationError
		assert.ErrorAs(t, err, &ve)
		assert.Equal(t, "", ve.Field)
		assert.Contains(t, ve.Message, "must have at least 1 properties")
	})

	t.Run("valid - nil map", func(t *testing.T) {
		var obj ConfigWithMinProps // nil map
		// nil maps are treated as valid
		assert.Nil(t, obj.Validate())
	})
}

func TestConfigWithMaxProps_Validate(t *testing.T) {
	t.Run("valid - has 1 property", func(t *testing.T) {
		obj := ConfigWithMaxProps{"key": "value"}
		assert.Nil(t, obj.Validate())
	})

	t.Run("valid - has 5 properties", func(t *testing.T) {
		obj := ConfigWithMaxProps{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
			"key5": "value5",
		}
		assert.Nil(t, obj.Validate())
	})

	t.Run("invalid - has 6 properties", func(t *testing.T) {
		obj := ConfigWithMaxProps{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
			"key5": "value5",
			"key6": "value6",
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have at most 5 properties")
	})
}

func TestConfigWithBothProps_Validate(t *testing.T) {
	t.Run("valid - has 2 properties", func(t *testing.T) {
		obj := ConfigWithBothProps{"key1": 1, "key2": 2}
		assert.Nil(t, obj.Validate())
	})

	t.Run("valid - has 10 properties", func(t *testing.T) {
		obj := ConfigWithBothProps{
			"key1":  1,
			"key2":  2,
			"key3":  3,
			"key4":  4,
			"key5":  5,
			"key6":  6,
			"key7":  7,
			"key8":  8,
			"key9":  9,
			"key10": 10,
		}
		assert.Nil(t, obj.Validate())
	})

	t.Run("invalid - has 1 property", func(t *testing.T) {
		obj := ConfigWithBothProps{"key1": 1}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have at least 2 properties")
	})

	t.Run("invalid - has 11 properties", func(t *testing.T) {
		obj := ConfigWithBothProps{
			"key1":  1,
			"key2":  2,
			"key3":  3,
			"key4":  4,
			"key5":  5,
			"key6":  6,
			"key7":  7,
			"key8":  8,
			"key9":  9,
			"key10": 10,
			"key11": 11,
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have at most 10 properties")
	})
}

func TestUsersWithRequiredFields_Validate(t *testing.T) {
	t.Run("valid - all required fields present", func(t *testing.T) {
		obj := UsersWithRequiredFields{
			"user1": UsersWithRequiredFields_AdditionalProperties{
				Name:  "John",
				Email: "john@example.com",
			},
		}
		assert.Nil(t, obj.Validate())
	})

	t.Run("invalid - missing required field", func(t *testing.T) {
		obj := UsersWithRequiredFields{
			"user1": UsersWithRequiredFields_AdditionalProperties{
				Name: "John",
				// Email is missing
			},
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Email")
		assert.Contains(t, err.Error(), "required")
		// Check that it's a ValidationError with the key as the field
		var ve runtime.ValidationError
		assert.ErrorAs(t, err, &ve)
		assert.Equal(t, "user1", ve.Field)
		assert.Contains(t, ve.Message, "Email")
		assert.Contains(t, ve.Message, "required")
	})
}

func TestArrayWithMinItems_Validate(t *testing.T) {
	t.Run("valid - has 1 item", func(t *testing.T) {
		obj := ArrayWithMinItems{"item1"}
		assert.Nil(t, obj.Validate())
	})

	t.Run("valid - has 100 items", func(t *testing.T) {
		obj := make(ArrayWithMinItems, 100)
		for i := 0; i < 100; i++ {
			obj[i] = "item"
		}
		assert.Nil(t, obj.Validate())
	})

	t.Run("invalid - empty array", func(t *testing.T) {
		obj := ArrayWithMinItems{}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have at least 1 items")
	})

	t.Run("invalid - has 101 items", func(t *testing.T) {
		obj := make(ArrayWithMinItems, 101)
		for i := 0; i < 101; i++ {
			obj[i] = "item"
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must have at most 100 items")
	})
}
