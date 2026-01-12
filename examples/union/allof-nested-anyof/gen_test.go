package union

import (
	"encoding/json"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/require"
)

func createOrderProduct(t *testing.T) Order_Product1 {
	t.Helper()
	return Order_Product1{
		Order_Product_AllOf0: &Order_Product_AllOf0{
			Order_Product_AllOf0_AnyOf: &Order_Product_AllOf0_AnyOf{
				runtime.NewEitherFromB[VariantA, VariantB](VariantB{
					Country: func() *string {
						s := "DE"
						return &s
					}(),
				}),
			},
		},
		Base: Base{
			Weight: 12.32,
		},
	}
}

func TestOrder_Product1_MarshalJSON(t *testing.T) {
	obj := createOrderProduct(t)

	expectedJSON := `
	{
		"country": "DE",
		"weight": 12.32
	}
	`

	res, err := json.Marshal(obj)
	require.NoError(t, err)
	require.JSONEq(t, expectedJSON, string(res))
}

func TestOrder_Product1_UnmarshalJSON(t *testing.T) {
	inputJSON := `{
		"country": "DE",
		"weight": 12.32
	}`
	var obj Order_Product1
	err := json.Unmarshal([]byte(inputJSON), &obj)
	require.NoError(t, err)

	actual, _ := json.Marshal(obj)
	require.JSONEq(t, inputJSON, string(actual))
}
