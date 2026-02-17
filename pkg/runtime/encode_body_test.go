// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package runtime

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeFormFields(t *testing.T) {
	type Coordinates struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	type Address struct {
		City        string `json:"city"`
		Country     string `json:"country"`
		Coordinates `json:"coordinates"`
	}

	type User struct {
		ID        int      `json:"id"`
		Name      string   `json:"name"`
		Age       int      `json:"age"`
		Address   Address  `json:"address"`
		Nicknames []string `json:"nicknames"`
	}

	payload := User{
		ID:   123456789,
		Name: "Jane Doe",
		Age:  30,
		Address: Address{
			City:    "Berlin",
			Country: "DE",
			Coordinates: Coordinates{
				Latitude:  52.5200,
				Longitude: 13.4050,
			},
		},
		Nicknames: []string{"JD", "Janie"},
	}

	t.Run("default", func(t *testing.T) {
		res, err := EncodeFormFields(payload, map[string]FieldEncoding{
			"address": {},
			"nicknames": {
				Style: "form",
			},
		})

		resDecoded, _ := url.QueryUnescape(res)
		require.NoError(t, err)
		expected := "address.city=Berlin&address.coordinates.latitude=52.52&address.coordinates.longitude=13.405&address.country=DE&age=30&id=123456789&name=Jane Doe&nicknames=JD&nicknames=Janie"
		assert.Equal(t, expected, resDecoded)
	})

	t.Run("deepObject", func(t *testing.T) {
		res, err := EncodeFormFields(payload, map[string]FieldEncoding{
			"address": {
				Style: "deepObject",
			},
			"nicknames": {
				Style: "deepObject",
			},
		})

		resDecoded, _ := url.QueryUnescape(res)
		require.NoError(t, err)
		expected := "address[city]=Berlin&address[coordinates][latitude]=52.52&address[coordinates][longitude]=13.405&address[country]=DE&age=30&id=123456789&name=Jane Doe&nicknames[0]=JD&nicknames[1]=Janie"
		assert.Equal(t, expected, resDecoded)
	})

	t.Run("different types", func(t *testing.T) {
		data := map[string]any{
			"v1": 1,
			"v2": 1.2,
			"v3": true,
			"v4": "test",
			"v5": 123456789,
			"v6": 123.456789,
			"v7": int64(12345678901234),
		}
		res, err := EncodeFormFields(data, map[string]FieldEncoding{})

		resDecoded, _ := url.QueryUnescape(res)
		require.NoError(t, err)
		expected := "v1=1&v2=1.2&v3=true&v4=test&v5=123456789&v6=123.456789&v7=12345678901234"
		assert.Equal(t, expected, resDecoded)
	})
}

func TestConvertFormFields(t *testing.T) {
	t.Run("simple key-value pairs", func(t *testing.T) {
		input := []byte("name=John&age=30&active=true")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		assert.Equal(t, "John", data["name"])
		assert.Equal(t, float64(30), data["age"]) // JSON numbers are float64
		assert.Equal(t, true, data["active"])
	})

	t.Run("deepObject encoding - nested object", func(t *testing.T) {
		input := []byte("address[city]=Berlin&address[country]=DE")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		address, ok := data["address"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Berlin", address["city"])
		assert.Equal(t, "DE", address["country"])
	})

	t.Run("deepObject encoding - deeply nested", func(t *testing.T) {
		input := []byte("address[coordinates][latitude]=52.52&address[coordinates][longitude]=13.405")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		address, ok := data["address"].(map[string]any)
		require.True(t, ok)
		coords, ok := address["coordinates"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, 52.52, coords["latitude"])
		assert.Equal(t, 13.405, coords["longitude"])
	})

	t.Run("deepObject encoding - array with indices", func(t *testing.T) {
		input := []byte("items[0]=first&items[1]=second&items[2]=third")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		items, ok := data["items"].([]any)
		require.True(t, ok)
		assert.Len(t, items, 3)
		assert.Equal(t, "first", items[0])
		assert.Equal(t, "second", items[1])
		assert.Equal(t, "third", items[2])
	})

	t.Run("deepObject encoding - array of objects", func(t *testing.T) {
		input := []byte("items[0][id]=1&items[0][name]=first&items[1][id]=2&items[1][name]=second")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		items, ok := data["items"].([]any)
		require.True(t, ok)
		assert.Len(t, items, 2)

		item0, ok := items[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(1), item0["id"])
		assert.Equal(t, "first", item0["name"])

		item1, ok := items[1].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(2), item1["id"])
		assert.Equal(t, "second", item1["name"])
	})

	t.Run("type conversion - booleans", func(t *testing.T) {
		input := []byte("enabled=true&disabled=false")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		assert.Equal(t, true, data["enabled"])
		assert.Equal(t, false, data["disabled"])
	})

	t.Run("type conversion - integers", func(t *testing.T) {
		input := []byte("count=42&zero=0&negative=-10")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		assert.Equal(t, float64(42), data["count"])
		assert.Equal(t, float64(0), data["zero"])
		assert.Equal(t, float64(-10), data["negative"])
	})

	t.Run("type conversion - floats", func(t *testing.T) {
		input := []byte("price=19.99&rate=0.05")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		assert.Equal(t, 19.99, data["price"])
		assert.Equal(t, 0.05, data["rate"])
	})

	t.Run("preserves strings that look like numbers but shouldn't convert", func(t *testing.T) {
		input := []byte("phone=%2B1234567890&zip=00123&formatted=1%20234")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		// Phone numbers starting with + should stay as strings
		assert.Equal(t, "+1234567890", data["phone"])
		// Leading zeros should stay as strings
		assert.Equal(t, "00123", data["zip"])
		// Strings with spaces should stay as strings
		assert.Equal(t, "1 234", data["formatted"])
	})

	t.Run("Stripe-like flow_data structure", func(t *testing.T) {
		// Real-world example from Stripe API
		input := []byte("flow_data[subscription_update_confirm][items][0][id]=si_123&flow_data[subscription_update_confirm][items][0][quantity]=2")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		flowData, ok := data["flow_data"].(map[string]any)
		require.True(t, ok)
		subUpdate, ok := flowData["subscription_update_confirm"].(map[string]any)
		require.True(t, ok)
		items, ok := subUpdate["items"].([]any)
		require.True(t, ok)
		require.Len(t, items, 1)
		item0, ok := items[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "si_123", item0["id"])
		assert.Equal(t, float64(2), item0["quantity"])
	})

	t.Run("expand array parameter", func(t *testing.T) {
		// Common pattern in APIs like Stripe
		input := []byte("expand[0]=customer&expand[1]=subscription")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)

		expand, ok := data["expand"].([]any)
		require.True(t, ok)
		assert.Len(t, expand, 2)
		assert.Equal(t, "customer", expand[0])
		assert.Equal(t, "subscription", expand[1])
	})

	t.Run("invalid form data returns error", func(t *testing.T) {
		input := []byte("%invalid")
		_, err := ConvertFormFields(input)
		assert.Error(t, err)
	})

	t.Run("empty input", func(t *testing.T) {
		input := []byte("")
		result, err := ConvertFormFields(input)
		require.NoError(t, err)

		var data map[string]any
		err = json.Unmarshal(result, &data)
		require.NoError(t, err)
		assert.Empty(t, data)
	})
}

func TestParseDeepObjectKey(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"simple", []string{"simple"}},
		{"obj[key]", []string{"obj", "key"}},
		{"obj[key][nested]", []string{"obj", "key", "nested"}},
		{"items[0]", []string{"items", "0"}},
		{"items[0][id]", []string{"items", "0", "id"}},
		{"flow_data[subscription_update_confirm][items][0][id]", []string{"flow_data", "subscription_update_confirm", "items", "0", "id"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseDeepObjectKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertFormStringValue(t *testing.T) {
	tests := []struct {
		input    string
		expected any
	}{
		{"true", true},
		{"false", false},
		{"42", int64(42)},
		{"0", int64(0)},
		{"-10", int64(-10)},
		{"3.14", 3.14},
		{"0.5", 0.5},
		{"hello", "hello"},
		{"+1234567890", "+1234567890"}, // Phone number - stays string
		{"00123", "00123"},             // Leading zeros - stays string
		{"hello world", "hello world"}, // Contains space - stays string
		{"(123)", "(123)"},             // Contains parens - stays string
		{"", ""},                       // Empty string
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := convertFormStringValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
