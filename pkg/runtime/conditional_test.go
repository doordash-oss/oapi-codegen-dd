// Copyright 2026 DoorDash, Inc.
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConditionalFromThen(t *testing.T) {
	res := NewConditionalFromThen[string, int]("test")

	assert.True(t, res.IsThen())
	assert.Equal(t, "test", res.Then)
	assert.False(t, res.IsElse())
	assert.Equal(t, 1, res.N)
	assert.Equal(t, 0, res.Else)
}

func TestNewConditionalFromElse(t *testing.T) {
	res := NewConditionalFromElse[string, int](10)

	assert.False(t, res.IsThen())
	assert.Equal(t, "", res.Then)
	assert.True(t, res.IsElse())
	assert.Equal(t, 2, res.N)
	assert.Equal(t, 10, res.Else)
}

func TestConditional_Value(t *testing.T) {
	t.Run("returns Then when IsThen", func(t *testing.T) {
		res := NewConditionalFromThen[string, int]("test")
		assert.Equal(t, "test", res.Value())
	})

	t.Run("returns Else when IsElse", func(t *testing.T) {
		res := NewConditionalFromElse[string, int](10)
		assert.Equal(t, 10, res.Value())
	})

	t.Run("returns nil when neither", func(t *testing.T) {
		var res Conditional[string, int]
		assert.Nil(t, res.Value())
	})
}

func TestConditional_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected Conditional[string, int]
	}{
		{
			name:     "string matches Then",
			input:    []byte(`"test"`),
			expected: NewConditionalFromThen[string, int]("test"),
		},
		{
			name:     "int matches Else",
			input:    []byte(`10`),
			expected: NewConditionalFromElse[string, int](10),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var res Conditional[string, int]
			err := res.UnmarshalJSON(test.input)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, res)
		})
	}
}

func TestConditional_MarshalJSON(t *testing.T) {
	t.Run("marshals Then variant", func(t *testing.T) {
		c := NewConditionalFromThen[string, int]("hello")
		data, err := c.MarshalJSON()
		assert.NoError(t, err)
		assert.JSONEq(t, `"hello"`, string(data))
	})

	t.Run("marshals Else variant", func(t *testing.T) {
		c := NewConditionalFromElse[string, int](42)
		data, err := c.MarshalJSON()
		assert.NoError(t, err)
		assert.JSONEq(t, `42`, string(data))
	})

	t.Run("marshals null when neither", func(t *testing.T) {
		var c Conditional[string, int]
		data, err := c.MarshalJSON()
		assert.NoError(t, err)
		assert.Equal(t, "null", string(data))
	})
}

func TestConditional_UnmarshalJSON_Null(t *testing.T) {
	t.Run("null input resets to zero", func(t *testing.T) {
		c := NewConditionalFromThen[string, int]("existing")
		err := c.UnmarshalJSON([]byte(`null`))
		assert.NoError(t, err)
		assert.Equal(t, 0, c.N)
		assert.Equal(t, "", c.Then)
		assert.Equal(t, 0, c.Else)
	})

	t.Run("empty input resets to zero", func(t *testing.T) {
		c := NewConditionalFromElse[string, int](5)
		err := c.UnmarshalJSON([]byte(``))
		assert.NoError(t, err)
		assert.Equal(t, 0, c.N)
	})
}

// Wrapper type mimicking generated code
type ThenBranch struct {
	FieldA string `json:"fieldA"`
}

type ElseBranch struct {
	FieldB int `json:"fieldB"`
}

type ResourceCondition struct {
	Conditional[ThenBranch, ElseBranch]
}

func TestConditional_MarshalJSON_WithWrapper(t *testing.T) {
	tests := []struct {
		name     string
		input    ResourceCondition
		expected string
	}{
		{
			name:     "then branch",
			input:    ResourceCondition{Conditional: NewConditionalFromThen[ThenBranch, ElseBranch](ThenBranch{FieldA: "hello"})},
			expected: `{"fieldA":"hello"}`,
		},
		{
			name:     "else branch",
			input:    ResourceCondition{Conditional: NewConditionalFromElse[ThenBranch, ElseBranch](ElseBranch{FieldB: 42})},
			expected: `{"fieldB":42}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := test.input.MarshalJSON()
			assert.NoError(t, err)
			assert.JSONEq(t, test.expected, string(res))
		})
	}
}

func TestConditional_UnmarshalJSON_WithWrapper(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ResourceCondition
	}{
		{
			name:     "then branch",
			input:    `{"fieldA":"hello"}`,
			expected: ResourceCondition{Conditional: NewConditionalFromThen[ThenBranch, ElseBranch](ThenBranch{FieldA: "hello"})},
		},
		{
			name:     "else branch",
			input:    `{"fieldB":42}`,
			expected: ResourceCondition{Conditional: NewConditionalFromElse[ThenBranch, ElseBranch](ElseBranch{FieldB: 42})},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var res ResourceCondition
			err := res.UnmarshalJSON([]byte(test.input))
			assert.NoError(t, err)
			assert.Equal(t, test.expected, res)
		})
	}
}

// Validatable types for disambiguation tests
type StrictConfig struct {
	Mode    string `json:"mode" validate:"required"`
	Timeout int    `json:"timeout" validate:"required,min=1"`
}

func (s StrictConfig) Validate() error {
	if s.Mode == "" || s.Timeout < 1 {
		return assert.AnError
	}
	return nil
}

type LaxConfig struct {
	Mode    string `json:"mode"`
	Timeout int    `json:"timeout"`
}

func (l LaxConfig) Validate() error {
	return nil
}

func TestConditional_Validate(t *testing.T) {
	t.Run("validates Then variant when active", func(t *testing.T) {
		valid := StrictConfig{Mode: "fast", Timeout: 30}
		c := NewConditionalFromThen[StrictConfig, string](valid)
		assert.NoError(t, c.Validate())
	})

	t.Run("validates Else variant when active", func(t *testing.T) {
		c := NewConditionalFromElse[StrictConfig, string]("fallback")
		assert.NoError(t, c.Validate())
	})

	t.Run("fails validation for invalid Then variant", func(t *testing.T) {
		invalid := StrictConfig{Mode: "", Timeout: 0}
		c := NewConditionalFromThen[StrictConfig, string](invalid)
		assert.Error(t, c.Validate())
	})

	t.Run("does not validate inactive Else variant", func(t *testing.T) {
		valid := StrictConfig{Mode: "fast", Timeout: 30}
		c := NewConditionalFromThen[StrictConfig, string](valid)
		assert.NoError(t, c.Validate())
	})

	t.Run("returns nil when neither is active", func(t *testing.T) {
		var c Conditional[StrictConfig, string]
		assert.NoError(t, c.Validate())
	})
}

func TestConditional_UnmarshalJSON_Disambiguation(t *testing.T) {
	t.Run("prefers type that validates when both unmarshal", func(t *testing.T) {
		// Empty mode/timeout - StrictConfig fails validation, LaxConfig passes
		data := []byte(`{"mode":"","timeout":0}`)
		var c Conditional[StrictConfig, LaxConfig]

		err := c.UnmarshalJSON(data)
		assert.NoError(t, err)
		assert.True(t, c.IsElse(), "should choose Else (LaxConfig) because it validates")
		assert.Equal(t, "", c.Else.Mode)
		assert.Equal(t, 0, c.Else.Timeout)
	})

	t.Run("defaults to Then when both validate", func(t *testing.T) {
		data := []byte(`{"mode":"fast","timeout":30}`)
		var c Conditional[StrictConfig, LaxConfig]

		err := c.UnmarshalJSON(data)
		assert.NoError(t, err)
		assert.True(t, c.IsThen(), "should default to Then when both validate")
		assert.Equal(t, "fast", c.Then.Mode)
		assert.Equal(t, 30, c.Then.Timeout)
	})
}

func TestConditional_UnmarshalJSON_FailsBoth(t *testing.T) {
	// Neither string nor int can unmarshal an object with a nested array
	data := []byte(`[1,2,3]`)
	var c Conditional[string, int]

	err := c.UnmarshalJSON(data)
	assert.Error(t, err)
}
