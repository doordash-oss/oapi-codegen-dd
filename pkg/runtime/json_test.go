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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsMap(t *testing.T) {
	t.Run("converts struct to map[string]any", func(t *testing.T) {
		type TestStruct struct {
			Name  string `json:"name"`
			Age   int    `json:"age"`
			Email string `json:"email,omitempty"`
		}

		input := TestStruct{Name: "John", Age: 30}
		result, err := AsMap[any](input)

		require.NoError(t, err)
		assert.Equal(t, "John", result["name"])
		assert.Equal(t, float64(30), result["age"]) // JSON numbers unmarshal as float64
		assert.NotContains(t, result, "email")      // omitempty field not present
	})

	t.Run("converts struct to map[string]string", func(t *testing.T) {
		type HeaderStruct struct {
			Authorization string `json:"authorization"`
			ContentType   string `json:"content-type"`
		}

		input := HeaderStruct{Authorization: "Bearer token", ContentType: "application/json"}
		result, err := AsMap[string](input)

		require.NoError(t, err)
		assert.Equal(t, "Bearer token", result["authorization"])
		assert.Equal(t, "application/json", result["content-type"])
	})

	t.Run("returns nil for nil input", func(t *testing.T) {
		result, err := AsMap[any](nil)

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("handles pointer fields", func(t *testing.T) {
		type TestStruct struct {
			Name  *string `json:"name,omitempty"`
			Count *int    `json:"count,omitempty"`
		}

		name := "test"
		count := 42
		input := TestStruct{Name: &name, Count: &count}
		result, err := AsMap[any](input)

		require.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, float64(42), result["count"])
	})

	t.Run("handles nested structs", func(t *testing.T) {
		type Address struct {
			City string `json:"city"`
		}
		type Person struct {
			Name    string  `json:"name"`
			Address Address `json:"address"`
		}

		input := Person{Name: "John", Address: Address{City: "NYC"}}
		result, err := AsMap[any](input)

		require.NoError(t, err)
		assert.Equal(t, "John", result["name"])
		address, ok := result["address"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "NYC", address["city"])
	})
}

var (
	outputJSON, outputIndentJSON, outputNonexistentJSON string
	input                                               = `
{  
  "number": 1,
  "string": "value",
  "object": {
    "number": 1,
    "string": "value",
    "nested_object": {
      "number": 2
    },
    "array": [1, 2, 3],
    "partial_array": [1, 2, 3]
  }
}
    `
	patch = `
{  
  "number": 2,
  "string": "value1",
  "nonexitent": "woot",
  "object": {
    "number": 3,
    "string": "value2",
    "nested_object": {
      "number": 4
    },
    "array": [3, 2, 1],
    "partial_array": {
      "1": 4
    }
  }
}
    `
)

func TestMain(t *testing.M) {
	output := []byte(`
{  
  "number": 2,
  "string": "value1",
  "object": {
    "number": 3,
    "string": "value2",
    "nested_object": {
      "number": 4
    },
    "array": [3, 2, 1],
    "partial_array": [1, 4, 3]
  }
}
    `)
	outputNonexistent := []byte(`
{
  "number": 2,
  "string": "value1",
  "nonexitent": "woot",
  "object": {
    "number": 3,
    "string": "value2",
    "nested_object": {
      "number": 4
    },
    "array": [3, 2, 1],
    "partial_array": [1, 4, 3]
  }
}
`)

	var outputData any
	_ = json.Unmarshal(output, &outputData)

	output, _ = json.Marshal(outputData)
	outputJSON = string(output)

	output, _ = json.MarshalIndent(outputData, " ", "  ")
	outputIndentJSON = string(output)

	var outputNonexistentData any
	_ = json.Unmarshal(outputNonexistent, &outputNonexistentData)
	output, _ = json.Marshal(outputNonexistentData)
	outputNonexistentJSON = string(output)

	t.Run()
}

func TestMergeBytesIndent(t *testing.T) {
	merger := &Merger{}
	result, err := merger.MergeBytesIndent([]byte(input), []byte(patch), " ", "  ")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if string(result) != outputIndentJSON {
		t.Errorf("Result not equals output\nExpected:\n%s\n\nGot:\n%s\n\n", outputIndentJSON, result)
	}

	if len(merger.Errors) != 0 {
		t.Errorf("info.Errors count is not 0, count: %v", len(merger.Errors))
	}
}

func TestMergeBytes(t *testing.T) {
	merger := &Merger{}
	result, err := merger.MergeBytes([]byte(input), []byte(patch))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if string(result) != outputJSON {
		t.Errorf("Result not equals output\nExpected:\n%s\n\nGot:\n%s\n\n", outputJSON, result)
	}

	if len(merger.Errors) != 0 {
		t.Errorf("info.Errors count is not 0, count: %v", len(merger.Errors))
	}
}

func TestMergeBytesNonexistent(t *testing.T) {
	merger := &Merger{
		CopyNonexistent: true,
	}
	result, err := merger.MergeBytes([]byte(input), []byte(patch))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if string(result) != outputNonexistentJSON {
		t.Errorf("Result not equals output\nExpected:\n%s\n\nGot:\n%s\n\n", outputNonexistentJSON, result)
	}

	if len(merger.Errors) != 0 {
		t.Errorf("info.Errors count is not 0, count: %v", len(merger.Errors))
	}
}

func TestLongNumbers(t *testing.T) {
	input := `{"Id":12423434,"Value":12423434}`
	patch := `{"Value":12423439}`
	outputJSON := `{"Id":12423434,"Value":12423439}`

	merger := &Merger{}
	result, err := merger.MergeBytes([]byte(input), []byte(patch))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if string(result) != outputJSON {
		t.Errorf("Result not equals output\nExpected:\n%s\n\nGot:\n%s\n\n", outputJSON, result)
	}

	if len(merger.Errors) != 0 {
		t.Errorf("info.Errors count is not 0, count: %v", len(merger.Errors))
	}
}

func TestJSONMerge(t *testing.T) {
	t.Run("when object", func(t *testing.T) {
		t.Run("Merges properties defined in both objects", func(t *testing.T) {
			data := `{"foo": 1}`
			patch := `{"foo": null}`
			expected := `{"foo":null}`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Sets property defined in only src object", func(t *testing.T) {
			data := `{}`
			patch := `{"source":"merge-me"}`
			expected := `{"source":"merge-me"}`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Handles child objects", func(t *testing.T) {
			data := `{"channel":{"status":"valid"}}`
			patch := `{"channel":{"id":1}}`
			expected := `{"channel":{"id":1,"status":"valid"}}`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Handles empty objects", func(t *testing.T) {
			data := `{}`
			patch := `{}`
			expected := `{}`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Handles nil data", func(t *testing.T) {
			patch := `{"foo":"bar"}`
			expected := `{"foo":"bar"}`

			actual, err := JSONMerge(nil, []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})

		t.Run("Handles nil patch", func(t *testing.T) {
			data := `{"foo":"bar"}`
			expected := `{"foo":"bar"}`

			actual, err := JSONMerge([]byte(data), nil)
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})
	})
	t.Run("when array", func(t *testing.T) {
		t.Run("it does not merge", func(t *testing.T) {
			data := `[{"foo": 1}]`
			patch := `[{"foo": null}]`
			expected := `[{"foo":1}]`

			actual, err := JSONMerge([]byte(data), []byte(patch))
			assert.NoError(t, err)
			assert.Equal(t, expected, string(actual))
		})
	})
}

func TestCoalesceOrMerge(t *testing.T) {
	t.Run("when object", func(t *testing.T) {
		parts := []json.RawMessage{
			[]byte(`{"foo": 1}`),
			[]byte(`{"bar": {"car": "var"}}`),
			[]byte(`{"baz": null}`),
		}

		expected := `{"bar":{"car":"var"},"baz":null,"foo":1}`

		res, err := CoalesceOrMerge(parts...)
		require.NoError(t, err)
		require.JSONEq(t, expected, string(res))
	})

	t.Run("when array", func(t *testing.T) {
		parts := []json.RawMessage{
			[]byte(`[1, 2]`),
			[]byte(`[3, 4]`),
		}
		expected := `[1,2,3,4]`
		res, err := CoalesceOrMerge(parts...)
		require.NoError(t, err)
		require.JSONEq(t, expected, string(res))
	})

	t.Run("when scalar", func(t *testing.T) {
		parts := []json.RawMessage{
			[]byte(`1`),
			[]byte(`2`),
		}

		res, err := CoalesceOrMerge(parts...)
		assert.Nil(t, res)
		assert.Equal(t, "cannot combine 2 non-null branches of mixed/unsupported kinds", err.Error())
	})
}

func TestUnmarshalAs(t *testing.T) {
	t.Run("unmarshals into struct", func(t *testing.T) {
		type Person struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		data := json.RawMessage(`{"name":"John","age":30}`)
		result, err := UnmarshalAs[Person](data)

		require.NoError(t, err)
		assert.Equal(t, "John", result.Name)
		assert.Equal(t, 30, result.Age)
	})

	t.Run("unmarshals into primitive types", func(t *testing.T) {
		strData := json.RawMessage(`"hello"`)
		strResult, err := UnmarshalAs[string](strData)
		require.NoError(t, err)
		assert.Equal(t, "hello", strResult)

		intData := json.RawMessage(`42`)
		intResult, err := UnmarshalAs[int](intData)
		require.NoError(t, err)
		assert.Equal(t, 42, intResult)

		boolData := json.RawMessage(`true`)
		boolResult, err := UnmarshalAs[bool](boolData)
		require.NoError(t, err)
		assert.True(t, boolResult)
	})

	t.Run("unmarshals into slice", func(t *testing.T) {
		data := json.RawMessage(`[1,2,3]`)
		result, err := UnmarshalAs[[]int](data)

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, result)
	})

	t.Run("unmarshals into map", func(t *testing.T) {
		data := json.RawMessage(`{"key":"value"}`)
		result, err := UnmarshalAs[map[string]string](data)

		require.NoError(t, err)
		assert.Equal(t, map[string]string{"key": "value"}, result)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		data := json.RawMessage(`{invalid}`)
		_, err := UnmarshalAs[map[string]string](data)

		require.Error(t, err)
	})

	t.Run("returns error for type mismatch", func(t *testing.T) {
		data := json.RawMessage(`"not a number"`)
		_, err := UnmarshalAs[int](data)

		require.Error(t, err)
	})

	t.Run("handles null", func(t *testing.T) {
		data := json.RawMessage(`null`)
		result, err := UnmarshalAs[*string](data)

		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestMarshalEitherWithDiscriminator(t *testing.T) {
	t.Run("adds discriminator to object", func(t *testing.T) {
		data := []byte(`{"name":"John"}`)
		result, err := MarshalEitherWithDiscriminator(data, "type", "person")

		require.NoError(t, err)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Equal(t, "John", parsed["name"])
		assert.Equal(t, "person", parsed["type"])
	})

	t.Run("overwrites existing discriminator", func(t *testing.T) {
		data := []byte(`{"type":"old","name":"John"}`)
		result, err := MarshalEitherWithDiscriminator(data, "type", "new")

		require.NoError(t, err)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Equal(t, "new", parsed["type"])
		assert.Equal(t, "John", parsed["name"])
	})

	t.Run("handles empty object", func(t *testing.T) {
		data := []byte(`{}`)
		result, err := MarshalEitherWithDiscriminator(data, "kind", "test")

		require.NoError(t, err)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Equal(t, "test", parsed["kind"])
		assert.Len(t, parsed, 1)
	})

	t.Run("handles nil data", func(t *testing.T) {
		result, err := MarshalEitherWithDiscriminator(nil, "type", "value")

		require.NoError(t, err)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Equal(t, "value", parsed["type"])
		assert.Len(t, parsed, 1)
	})

	t.Run("preserves nested objects", func(t *testing.T) {
		data := []byte(`{"nested":{"foo":"bar"},"array":[1,2,3]}`)
		result, err := MarshalEitherWithDiscriminator(data, "discriminator", "test")

		require.NoError(t, err)
		var parsed map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(result, &parsed))

		var nested map[string]string
		require.NoError(t, json.Unmarshal(parsed["nested"], &nested))
		assert.Equal(t, "bar", nested["foo"])

		var arr []int
		require.NoError(t, json.Unmarshal(parsed["array"], &arr))
		assert.Equal(t, []int{1, 2, 3}, arr)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		data := []byte(`{invalid}`)
		_, err := MarshalEitherWithDiscriminator(data, "type", "value")

		require.Error(t, err)
	})

	t.Run("handles special characters in discriminator value", func(t *testing.T) {
		data := []byte(`{}`)
		result, err := MarshalEitherWithDiscriminator(data, "type", `value with "quotes" and \backslash`)

		require.NoError(t, err)
		var parsed map[string]string
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Equal(t, `value with "quotes" and \backslash`, parsed["type"])
	})
}

func TestDecodeJSONBody(t *testing.T) {
	type TestRequest struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	t.Run("decodes valid JSON", func(t *testing.T) {
		body := strings.NewReader(`{"name":"test","count":42}`)
		var req TestRequest
		err := DecodeJSONBody(body, &req)

		require.NoError(t, err)
		assert.Equal(t, "test", req.Name)
		assert.Equal(t, 42, req.Count)
	})

	t.Run("returns friendly error for empty body", func(t *testing.T) {
		body := strings.NewReader("")
		var req TestRequest
		err := DecodeJSONBody(body, &req)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRequestBodyEmpty)
	})

	t.Run("returns friendly error for syntax error", func(t *testing.T) {
		body := strings.NewReader(`{"name": invalid}`)
		var req TestRequest
		err := DecodeJSONBody(body, &req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed JSON at position")
	})

	t.Run("returns friendly error for type mismatch", func(t *testing.T) {
		body := strings.NewReader(`{"name":"test","count":"not a number"}`)
		var req TestRequest
		err := DecodeJSONBody(body, &req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type for field")
		assert.Contains(t, err.Error(), "count")
	})
}
