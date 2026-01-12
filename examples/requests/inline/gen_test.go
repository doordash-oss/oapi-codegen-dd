package gen

import (
	"encoding/json"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/require"
)

type dA = ProcessPaymentBody_D_AllOf0_OneOf_0
type dB = ProcessPaymentBody_D_AllOf0_OneOf_1

func createProcessPaymentBody(t *testing.T, dVal runtime.Either[dA, dB]) ProcessPaymentBody {
	t.Helper()

	var a, b string
	a, b = "a-val", "b-val"

	c := &ProcessPaymentBody_C{
		ProcessPaymentBody_C_OneOf: &ProcessPaymentBody_C_OneOf{
			runtime.NewEitherFromB[string, bool](false),
		},
	}

	d := &ProcessPaymentBody_D1{
		ProcessPaymentBody_D_AllOf0: &ProcessPaymentBody_D_AllOf0{
			ProcessPaymentBody_D_AllOf0_OneOf: &ProcessPaymentBody_D_AllOf0_OneOf{
				dVal,
			},
		},
	}

	return ProcessPaymentBody{
		A: &a,
		B: &b,
		C: c,
		D: d,
	}
}

func TestProcessPaymentBody(t *testing.T) {
	t.Run("marshal with d float", func(t *testing.T) {
		eitherAVal := dA{
			ProcessPaymentBody_D_AllOf0_OneOf_0_AnyOf: &ProcessPaymentBody_D_AllOf0_OneOf_0_AnyOf{
				union: json.RawMessage(`12.34`),
			},
		}
		dVal := runtime.NewEitherFromA[dA, dB](eitherAVal)

		obj := createProcessPaymentBody(t, dVal)
		res, err := json.Marshal(obj)
		require.NoError(t, err)
		expectedJSON := `{
			"a": "a-val",
			"b": "b-val",
			"c": false,
			"d": 12.34
		}`
		require.JSONEq(t, expectedJSON, string(res))
	})

	t.Run("unmarshal with d float", func(t *testing.T) {
		inputJSON := `{
			"a": "a-val",
			"b": "b-val",
			"c": false,
			"d": 12.34
		}`
		var obj ProcessPaymentBody
		err := json.Unmarshal([]byte(inputJSON), &obj)
		require.NoError(t, err)

		eitherAVal := dA{
			ProcessPaymentBody_D_AllOf0_OneOf_0_AnyOf: &ProcessPaymentBody_D_AllOf0_OneOf_0_AnyOf{
				union: json.RawMessage(`12.34`),
			},
		}
		dVal := runtime.NewEitherFromA[dA, dB](eitherAVal)

		expectedObj := createProcessPaymentBody(t, dVal)
		require.Equal(t, expectedObj, obj)
	})

	t.Run("marshal with d map", func(t *testing.T) {
		eitherBVal := map[string]any{
			"foo": "bar",
			"car": "var",
		}
		dVal := runtime.NewEitherFromB[dA, dB](eitherBVal)

		obj := createProcessPaymentBody(t, dVal)
		res, err := json.Marshal(obj)
		require.NoError(t, err)
		expectedJSON := `{
			"a": "a-val",
			"b": "b-val",
			"c": false,
			"d": {"foo": "bar", "car": "var"}
		}`
		require.JSONEq(t, expectedJSON, string(res))
	})

	t.Run("unmarshal with d map", func(t *testing.T) {
		inputJSON := `{
			"a": "a-val",
			"b": "b-val",
			"c": false,
			"d": {"foo": "bar", "car": "var"}
		}`
		var obj ProcessPaymentBody
		err := json.Unmarshal([]byte(inputJSON), &obj)
		require.NoError(t, err)

		actual, _ := json.Marshal(obj)
		require.JSONEq(t, inputJSON, string(actual))
	})
}
