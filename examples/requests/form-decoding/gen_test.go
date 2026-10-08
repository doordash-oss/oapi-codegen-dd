package gen

import (
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UnmarshalForm recognizes generated unions and inline parts by their shape; if the generator changes it, the digit strings below get guessed as numbers.
func TestUnmarshalForm(t *testing.T) {
	decode := func(t *testing.T, form string) Order {
		t.Helper()
		var order Order
		require.NoError(t, runtime.UnmarshalForm([]byte(form), &order))
		return order
	}

	t.Run("plain fields", func(t *testing.T) {
		order := decode(t, "reference=12345&quantity=3")
		assert.Equal(t, "12345", *order.Reference)
		assert.Equal(t, 3, *order.Quantity)
	})

	t.Run("anyOf", func(t *testing.T) {
		address := decode(t, "address[city]=Berlin&address[postal_code]=10115").Address.Order_Address_AnyOf
		require.True(t, address.IsA())
		assert.Equal(t, "10115", *address.A.PostalCode)
	})

	t.Run("oneOf", func(t *testing.T) {
		phone, err := decode(t, "contact[phone]=4155551234").Contact.Order_Contact_OneOf.AsValidatedPhone()
		require.NoError(t, err)
		assert.Equal(t, "4155551234", phone.Phone)
	})

	t.Run("additionalProperties", func(t *testing.T) {
		labels := decode(t, "labels[kind]=7&labels[sku]=12345").Labels
		assert.Equal(t, "7", *labels.Kind)
		assert.Equal(t, map[string]string{"sku": "12345"}, labels.AdditionalProperties)
	})
}
