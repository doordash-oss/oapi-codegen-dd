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
	"time"

	"github.com/google/uuid"
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

	// Tests for format parameter
	t.Run("uuid with format", func(t *testing.T) {
		v, err := ParseString[uuid.UUID]("550e8400-e29b-41d4-a716-446655440000", "uuid")
		require.NoError(t, err)
		assert.Equal(t, uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), v)
	})

	t.Run("uuid invalid", func(t *testing.T) {
		_, err := ParseString[uuid.UUID]("not-a-uuid", "uuid")
		assert.Error(t, err)
	})

	t.Run("date-time with format", func(t *testing.T) {
		v, err := ParseString[time.Time]("2024-01-15T10:30:00Z", "date-time")
		require.NoError(t, err)
		expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
		assert.Equal(t, expected, v)
	})

	t.Run("date-time invalid", func(t *testing.T) {
		_, err := ParseString[time.Time]("not-a-date", "date-time")
		assert.Error(t, err)
	})

	t.Run("date with format", func(t *testing.T) {
		v, err := ParseString[Date]("2024-01-15", "date")
		require.NoError(t, err)
		expected, _ := time.Parse("2006-01-02", "2024-01-15")
		assert.Equal(t, Date{Time: expected}, v)
	})

	t.Run("date invalid", func(t *testing.T) {
		_, err := ParseString[Date]("not-a-date", "date")
		assert.Error(t, err)
	})

	t.Run("uuid without format falls through", func(t *testing.T) {
		// Without format hint, uuid.UUID won't be parsed (returns zero value)
		v, err := ParseString[uuid.UUID]("550e8400-e29b-41d4-a716-446655440000")
		require.NoError(t, err)
		assert.Equal(t, uuid.UUID{}, v) // Zero value since no format hint
	})

	// Tests for type aliases - these are the bug cases that were reported
	// Type switches don't match aliased types, so we use reflection
	t.Run("string alias", func(t *testing.T) {
		type VersionHeader string
		v, err := ParseString[VersionHeader]("2020-04-02")
		require.NoError(t, err)
		assert.Equal(t, VersionHeader("2020-04-02"), v)
	})

	t.Run("int alias", func(t *testing.T) {
		type StatusCode int
		v, err := ParseString[StatusCode]("200")
		require.NoError(t, err)
		assert.Equal(t, StatusCode(200), v)
	})

	t.Run("int64 alias", func(t *testing.T) {
		type UserID int64
		v, err := ParseString[UserID]("9223372036854775807")
		require.NoError(t, err)
		assert.Equal(t, UserID(9223372036854775807), v)
	})

	t.Run("uint alias", func(t *testing.T) {
		type Count uint
		v, err := ParseString[Count]("42")
		require.NoError(t, err)
		assert.Equal(t, Count(42), v)
	})

	t.Run("float32 alias", func(t *testing.T) {
		type Priority float32
		v, err := ParseString[Priority]("2.5")
		require.NoError(t, err)
		assert.InDelta(t, float32(2.5), float32(v), 0.001)
	})

	t.Run("float64 alias", func(t *testing.T) {
		type Score float64
		v, err := ParseString[Score]("3.14159")
		require.NoError(t, err)
		assert.InDelta(t, float64(3.14159), float64(v), 0.0001)
	})

	t.Run("bool alias", func(t *testing.T) {
		type Enabled bool
		v, err := ParseString[Enabled]("true")
		require.NoError(t, err)
		assert.Equal(t, Enabled(true), v)
	})

	t.Run("string alias with enum-like values", func(t *testing.T) {
		type Color string
		const ColorGreen Color = "green"

		v, err := ParseString[Color]("green")
		require.NoError(t, err)
		assert.Equal(t, ColorGreen, v)
	})

	t.Run("int alias with enum-like values", func(t *testing.T) {
		type HTTPStatus int
		const HTTPStatusNotFound HTTPStatus = 404

		v, err := ParseString[HTTPStatus]("404")
		require.NoError(t, err)
		assert.Equal(t, HTTPStatusNotFound, v)
	})

	t.Run("invalid int alias", func(t *testing.T) {
		type StatusCode int
		_, err := ParseString[StatusCode]("not-a-number")
		assert.Error(t, err)
	})

	t.Run("invalid bool alias", func(t *testing.T) {
		type Enabled bool
		_, err := ParseString[Enabled]("maybe")
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

	// Tests for format parameter
	t.Run("uuid slice with format", func(t *testing.T) {
		result, err := ParseStringSlice[uuid.UUID]([]string{
			"550e8400-e29b-41d4-a716-446655440000",
			"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		}, "uuid")
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), result[0])
		assert.Equal(t, uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"), result[1])
	})

	t.Run("uuid slice with invalid", func(t *testing.T) {
		result, err := ParseStringSlice[uuid.UUID]([]string{
			"550e8400-e29b-41d4-a716-446655440000",
			"not-a-uuid",
		}, "uuid")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	// Tests for type alias slices
	t.Run("string alias slice", func(t *testing.T) {
		type Color string
		result, err := ParseStringSlice[Color]([]string{"red", "green", "blue"})
		assert.NoError(t, err)
		assert.Equal(t, []Color{"red", "green", "blue"}, result)
	})

	t.Run("int alias slice", func(t *testing.T) {
		type StatusCode int
		result, err := ParseStringSlice[StatusCode]([]string{"200", "404", "500"})
		assert.NoError(t, err)
		assert.Equal(t, []StatusCode{200, 404, 500}, result)
	})

	t.Run("int alias slice with invalid", func(t *testing.T) {
		type StatusCode int
		result, err := ParseStringSlice[StatusCode]([]string{"200", "invalid", "500"})
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
