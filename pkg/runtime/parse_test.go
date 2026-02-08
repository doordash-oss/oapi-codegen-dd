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
	"github.com/stretchr/testify/require"
)

func TestParseString(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		v, err := ParseString[int]("42")
		require.NoError(t, err)
		assert.Equal(t, 42, v)
	})

	t.Run("int8", func(t *testing.T) {
		v, err := ParseString[int8]("127")
		require.NoError(t, err)
		assert.Equal(t, int8(127), v)
	})

	t.Run("int16", func(t *testing.T) {
		v, err := ParseString[int16]("32767")
		require.NoError(t, err)
		assert.Equal(t, int16(32767), v)
	})

	t.Run("int32", func(t *testing.T) {
		v, err := ParseString[int32]("2147483647")
		require.NoError(t, err)
		assert.Equal(t, int32(2147483647), v)
	})

	t.Run("int64", func(t *testing.T) {
		v, err := ParseString[int64]("9223372036854775807")
		require.NoError(t, err)
		assert.Equal(t, int64(9223372036854775807), v)
	})

	t.Run("uint", func(t *testing.T) {
		v, err := ParseString[uint]("42")
		require.NoError(t, err)
		assert.Equal(t, uint(42), v)
	})

	t.Run("uint8", func(t *testing.T) {
		v, err := ParseString[uint8]("255")
		require.NoError(t, err)
		assert.Equal(t, uint8(255), v)
	})

	t.Run("uint16", func(t *testing.T) {
		v, err := ParseString[uint16]("65535")
		require.NoError(t, err)
		assert.Equal(t, uint16(65535), v)
	})

	t.Run("uint32", func(t *testing.T) {
		v, err := ParseString[uint32]("4294967295")
		require.NoError(t, err)
		assert.Equal(t, uint32(4294967295), v)
	})

	t.Run("uint64", func(t *testing.T) {
		v, err := ParseString[uint64]("18446744073709551615")
		require.NoError(t, err)
		assert.Equal(t, uint64(18446744073709551615), v)
	})

	t.Run("float32", func(t *testing.T) {
		v, err := ParseString[float32]("3.14")
		require.NoError(t, err)
		assert.InDelta(t, float32(3.14), v, 0.001)
	})

	t.Run("float64", func(t *testing.T) {
		v, err := ParseString[float64]("3.14159265359")
		require.NoError(t, err)
		assert.InDelta(t, 3.14159265359, v, 0.0000001)
	})

	t.Run("bool true", func(t *testing.T) {
		v, err := ParseString[bool]("true")
		require.NoError(t, err)
		assert.True(t, v)
	})

	t.Run("bool false", func(t *testing.T) {
		v, err := ParseString[bool]("false")
		require.NoError(t, err)
		assert.False(t, v)
	})

	t.Run("string", func(t *testing.T) {
		v, err := ParseString[string]("hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", v)
	})

	t.Run("invalid int", func(t *testing.T) {
		_, err := ParseString[int]("not-a-number")
		assert.Error(t, err)
	})

	t.Run("invalid bool", func(t *testing.T) {
		_, err := ParseString[bool]("not-a-bool")
		assert.Error(t, err)
	})
}

func TestParseStringSlice(t *testing.T) {
	t.Run("int slice", func(t *testing.T) {
		result, err := ParseStringSlice[int]([]string{"1", "2", "3"})
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, result)
	})

	t.Run("int slice with invalid", func(t *testing.T) {
		result, err := ParseStringSlice[int]([]string{"1", "invalid", "3"})
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("float64 slice", func(t *testing.T) {
		result, err := ParseStringSlice[float64]([]string{"1.1", "2.2", "3.3"})
		assert.NoError(t, err)
		assert.InDeltaSlice(t, []float64{1.1, 2.2, 3.3}, result, 0.001)
	})

	t.Run("bool slice", func(t *testing.T) {
		result, err := ParseStringSlice[bool]([]string{"true", "false", "true"})
		assert.NoError(t, err)
		assert.Equal(t, []bool{true, false, true}, result)
	})

	t.Run("string slice", func(t *testing.T) {
		result, err := ParseStringSlice[string]([]string{"a", "b", "c"})
		assert.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result, err := ParseStringSlice[int]([]string{})
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}
