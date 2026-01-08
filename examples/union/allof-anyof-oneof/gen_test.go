package union

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrder_UnmarshalJSON_NilPointerInitialization(t *testing.T) {
	// This test verifies that unmarshaling works correctly when
	// the union pointer fields (anyOf/oneOf/allOf) are nil.
	// The UnmarshalJSON method should initialize them before unmarshaling.
	inputJSON := `{
		"client": {
			"name": "John Doe",
			"id": 123,
			"issuer": "TestIssuer",
			"city": "San Francisco"
		}
	}`

	var order Order
	err := json.Unmarshal([]byte(inputJSON), &order)
	require.NoError(t, err)

	// Verify the data was unmarshaled correctly
	require.NotNil(t, order.Client)
	require.Equal(t, "John Doe", order.Client.Name)
	require.Equal(t, 123, order.Client.ID)

	// Verify union fields were initialized and unmarshaled
	require.NotNil(t, order.Client.Order_Client_AnyOf)
	require.NotNil(t, order.Client.Order_Client_OneOf)

	// Verify we can marshal it back
	result, err := json.Marshal(order)
	require.NoError(t, err)
	require.JSONEq(t, inputJSON, string(result))
}

func TestOrder_MarshalJSON(t *testing.T) {
	// Test that marshaling works correctly
	expectedJSON := `{
		"client": {
			"name": "Jane Smith",
			"id": 456,
			"issuer": "AnotherIssuer",
			"city": "New York"
		}
	}`

	var order Order
	err := json.Unmarshal([]byte(expectedJSON), &order)
	require.NoError(t, err)

	result, err := json.Marshal(order)
	require.NoError(t, err)
	require.JSONEq(t, expectedJSON, string(result))
}
