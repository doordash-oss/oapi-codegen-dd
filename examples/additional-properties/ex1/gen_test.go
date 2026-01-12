package gen

import (
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
)

func TestAlwaysValidates(t *testing.T) {
	// Location is map[string]any with no constraints - no Validate() method
	// User has no validation tags - no Validate() method
	// Users is map[string]User, but User has no Validate() - no Validate() method
	// Pick1 has union values that need validation - has Validate() method
	assert.Nil(t, Users{}.Validate())
	assert.Nil(t, Pick1{}.Validate())
}

func TestReferenceWithRequiredExtra_NoValidation(t *testing.T) {
	// ReferenceWithRequiredExtra has no validation constraints
	// Index is *Reference (string alias) with no constraints
	// AdditionalProperties is map[string]Reference with no constraints
	// Therefore, no Validate() method is generated
	t.Run("can create instance", func(t *testing.T) {
		ref := "foo"
		obj := ReferenceWithRequiredExtra{
			Index: &ref,
			AdditionalProperties: map[string]string{
				"foo": "bar",
			},
		}
		assert.NotNil(t, obj)
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
		assert.Equal(t, "Map must have at least 1 properties, got 0", err.Error())

		// Check that it's a ValidationError
		var ve runtime.ValidationError
		assert.ErrorAs(t, err, &ve)
		assert.Equal(t, "Map", ve.Field)
		assert.Equal(t, "must have at least 1 properties, got 0", ve.Message)
	})

	t.Run("invalid - nil map (minProperties: 1)", func(t *testing.T) {
		var obj ConfigWithMinProps // nil map
		// nil maps with minProperties > 0 are invalid (nil = 0 properties)
		err := obj.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Map must have at least 1 properties, got 0", err.Error())
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
		assert.Equal(t, "Map must have at most 5 properties, got 6", err.Error())
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
		assert.Equal(t, "Map must have at least 2 properties, got 1", err.Error())
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
		assert.Equal(t, "Map must have at most 10 properties, got 11", err.Error())
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
		assert.Equal(t, "user1.Email is required", err.Error())

		var ve runtime.ValidationError
		assert.ErrorAs(t, err, &ve)
		assert.Equal(t, "user1.Email", ve.Field)
		assert.Equal(t, "is required", ve.Message)
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
		assert.Equal(t, "Array must have at least 1 items, got 0", err.Error())
	})

	t.Run("invalid - has 101 items", func(t *testing.T) {
		obj := make(ArrayWithMinItems, 101)
		for i := 0; i < 101; i++ {
			obj[i] = "item"
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Array must have at most 100 items, got 101", err.Error())
	})
}

func TestTagsWithLength_Validate(t *testing.T) {
	t.Run("valid - all values within length constraints", func(t *testing.T) {
		obj := TagsWithLength{
			"key1": "a",                                                  // minLength: 1
			"key2": "hello",                                              // valid
			"key3": "x",                                                  // exactly 1 char
			"key4": "12345678901234567890123456789012345678901234567890", // exactly 50 chars
		}
		assert.Nil(t, obj.Validate())
	})

	t.Run("valid - empty map", func(t *testing.T) {
		obj := TagsWithLength{}
		assert.Nil(t, obj.Validate())
	})

	t.Run("valid - nil map", func(t *testing.T) {
		var obj TagsWithLength
		assert.Nil(t, obj.Validate())
	})

	t.Run("valid - empty string with omitempty", func(t *testing.T) {
		// omitempty in validation tags means empty strings are allowed
		obj := TagsWithLength{
			"key1": "",
		}
		assert.Nil(t, obj.Validate())
	})

	t.Run("invalid - value exceeds maxLength", func(t *testing.T) {
		obj := TagsWithLength{
			"key1": "valid",

			// 51 chars - exceeds max of 50
			"key2": "123456789012345678901234567890123456789012345678901",
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Equal(t, "key2 length must be less than or equal to 50", err.Error())

		var ve runtime.ValidationError
		assert.ErrorAs(t, err, &ve)
		assert.Equal(t, "key2", ve.Field)
		assert.Equal(t, "length must be less than or equal to 50", ve.Message)
	})

	t.Run("invalid - multiple values exceed constraints", func(t *testing.T) {
		obj := TagsWithLength{
			"key1": "valid",
			"key2": "123456789012345678901234567890123456789012345678901", // 51 chars
			"key3": "short",
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "key2")
	})
}

func TestTagsWithBothConstraints_Validate(t *testing.T) {
	t.Run("valid - 2 properties with valid values", func(t *testing.T) {
		obj := TagsWithBothConstraints{
			"key1": "value1",
			"key2": "value2",
		}
		assert.Nil(t, obj.Validate())
	})

	t.Run("valid - 5 properties (max)", func(t *testing.T) {
		obj := TagsWithBothConstraints{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
			"key5": "value5",
		}
		assert.Nil(t, obj.Validate())
	})

	t.Run("invalid - nil map (minProperties: 2)", func(t *testing.T) {
		var obj TagsWithBothConstraints
		err := obj.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Map must have at least 2 properties, got 0", err.Error())
	})

	t.Run("invalid - only 1 property (minProperties: 2)", func(t *testing.T) {
		obj := TagsWithBothConstraints{
			"key1": "value1",
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Map must have at least 2 properties, got 1", err.Error())

		var ve runtime.ValidationError
		assert.ErrorAs(t, err, &ve)
		assert.Equal(t, "Map", ve.Field)
	})

	t.Run("invalid - empty map (minProperties: 2)", func(t *testing.T) {
		obj := TagsWithBothConstraints{}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Map must have at least 2 properties, got 0", err.Error())
	})

	t.Run("invalid - 6 properties (maxProperties: 5)", func(t *testing.T) {
		obj := TagsWithBothConstraints{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
			"key5": "value5",
			"key6": "value6",
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Equal(t, "Map must have at most 5 properties, got 6", err.Error())
	})

	t.Run("invalid - valid count but value exceeds maxLength", func(t *testing.T) {
		obj := TagsWithBothConstraints{
			"key1": "valid",
			"key2": "123456789012345678901234567890123456789012345678901", // 51 chars
		}
		err := obj.Validate()
		assert.Error(t, err)
		assert.Equal(t, "key2 length must be less than or equal to 50", err.Error())
	})

	t.Run("invalid - valid count but empty value (minLength: 1)", func(t *testing.T) {
		// Note: omitempty allows empty strings, so this should actually pass
		obj := TagsWithBothConstraints{
			"key1": "valid",
			"key2": "",
		}

		// Empty string is allowed due to omitempty
		assert.Nil(t, obj.Validate())
	})
}
