package gen

import (
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
)

func TestGetFiles_Response_Item_Validate(t *testing.T) {
	t.Run("has first value", func(t *testing.T) {
		a := runtime.NewEitherFromA[string, File]("foo")

		res := &GetFiles_Response_Item{
			GetFiles_Response_OneOf: &GetFiles_Response_OneOf{Either: a},
		}
		err := res.Validate()
		if err != nil {
			t.Logf("Validation error: %v", err)
		}
		assert.Nil(t, err)
	})

	t.Run("has second value", func(t *testing.T) {
		file := File{
			ID:      "file-123",
			Object:  "file",
			Purpose: "account_requirement",
			Size:    1024,
		}
		b := runtime.NewEitherFromB[string, File](file)

		res := &GetFiles_Response_Item{
			GetFiles_Response_OneOf: &GetFiles_Response_OneOf{Either: b},
		}
		err := res.Validate()
		assert.Nil(t, err)
	})

	t.Run("validation fails for invalid File", func(t *testing.T) {
		// Create a File with missing required fields
		file := File{
			ID: "file-123",
			// Missing Object, Purpose, and Size
		}
		b := runtime.NewEitherFromB[string, File](file)

		res := &GetFiles_Response_Item{
			GetFiles_Response_OneOf: &GetFiles_Response_OneOf{Either: b},
		}
		err := res.Validate()
		assert.NotNil(t, err)
	})
}
