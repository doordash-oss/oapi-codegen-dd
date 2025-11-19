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
	"bytes"
	"encoding/json"
	"reflect"
)

type Either[A, B any] struct {
	A A
	B B

	N int
}

func NewEitherFromA[A any, B any](a A) Either[A, B] {
	var b B
	return Either[A, B]{A: a, B: b, N: 1}
}

func NewEitherFromB[A any, B any](b B) Either[A, B] {
	var a A
	return Either[A, B]{A: a, B: b, N: 2}
}

func (t *Either[A, B]) IsA() bool {
	return t.N == 1
}

func (t *Either[A, B]) IsB() bool {
	return t.N == 2
}

func (t *Either[A, B]) Value() any {
	if t.IsA() {
		return t.A
	}
	if t.IsB() {
		return t.B
	}
	return nil
}

// MarshalJSON implements json.Marshaler interface
func (t Either[A, B]) MarshalJSON() ([]byte, error) {
	switch t.N {
	case 1:
		return json.Marshal(t.A)
	case 2:
		return json.Marshal(t.B)
	default:
		return []byte("null"), nil
	}
}

func (t *Either[A, B]) UnmarshalJSON(data []byte) error {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		var zeroA A
		var zeroB B
		t.A, t.B, t.N = zeroA, zeroB, 0
		return nil
	}

	var a A
	errA := json.Unmarshal(data, &a)

	var b B
	errB := json.Unmarshal(data, &b)

	switch {
	case errA == nil && errB != nil:
		// Only A fits
		var zeroB B
		t.A, t.B, t.N = a, zeroB, 1
		return nil

	case errB == nil && errA != nil:
		// Only B fits
		var zeroA A
		t.A, t.B, t.N = zeroA, b, 2
		return nil

	case errA == nil:
		// Both decoded; apply zero/meaningfulness heuristics, then tie-break to A.

		na := isNonZero(a)
		nb := isNonZero(b)

		// Prefer the one that looks non-zero if only one does.
		if na && !nb {
			var zeroB B
			t.A, t.B, t.N = a, zeroB, 1
			return nil
		}
		if nb && !na {
			var zeroA A
			t.A, t.B, t.N = zeroA, b, 2
			return nil
		}

		// Tie (both zero or both non-zero/ambiguous): pick A
		{
			var zeroB B
			t.A, t.B, t.N = a, zeroB, 1
			return nil
		}
	default:
		// Neither decoded
		return ErrFailedToUnmarshalAsAOrB
	}
}

type JSONNonZero interface {
	JSONNonZero() bool
}

type isZeroer interface {
	IsZero() bool
}

func isNonZero[T any](v T) bool {
	switch x := any(v).(type) {
	case nil:
		return false

	// hooks first
	case JSONNonZero:
		return x.JSONNonZero()
	case isZeroer:
		return !x.IsZero()

	// primitives
	case bool:
		return x
	case string:
		return x != ""
	case int, int8, int16, int32, int64:
		return x != 0
	case uint, uint8, uint16, uint32, uint64, uintptr:
		return x != 0
	case float32, float64:
		return x != 0

	// pointers to common primitives
	case *bool, *string,
		*int, *int8, *int16, *int32, *int64,
		*uint, *uint8, *uint16, *uint32, *uint64, *uintptr,
		*float32, *float64:
		return x != nil

	// common “any”-shaped collections
	case []byte:
		return len(x) > 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0

	default:
		return !reflect.ValueOf(v).IsZero()
	}
}
