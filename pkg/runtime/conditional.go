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
	"bytes"
	"encoding/json"
)

// Conditional represents a JSON Schema if/then/else result.
// At most one of Then or Else is populated, indicated by N.
// N == 1 means Then matched, N == 2 means Else matched, N == 0 means neither.
type Conditional[T, E any] struct {
	Then T `validate:"-"`
	Else E `validate:"-"`

	N int
}

func NewConditionalFromThen[T any, E any](then T) Conditional[T, E] {
	var e E
	return Conditional[T, E]{Then: then, Else: e, N: 1}
}

func NewConditionalFromElse[T any, E any](els E) Conditional[T, E] {
	var t T
	return Conditional[T, E]{Then: t, Else: els, N: 2}
}

func (c *Conditional[T, E]) IsThen() bool {
	return c.N == 1
}

func (c *Conditional[T, E]) IsElse() bool {
	return c.N == 2
}

func (c *Conditional[T, E]) Value() any {
	if c.IsThen() {
		return c.Then
	}
	if c.IsElse() {
		return c.Else
	}
	return nil
}

// MarshalJSON implements json.Marshaler interface
func (c Conditional[T, E]) MarshalJSON() ([]byte, error) {
	switch c.N {
	case 1:
		return json.Marshal(c.Then)
	case 2:
		return json.Marshal(c.Else)
	default:
		return []byte("null"), nil
	}
}

func (c *Conditional[T, E]) UnmarshalJSON(data []byte) error {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		var zeroT T
		var zeroE E
		c.Then, c.Else, c.N = zeroT, zeroE, 0
		return nil
	}

	var t T
	errT := json.Unmarshal(data, &t)

	var e E
	errE := json.Unmarshal(data, &e)

	switch {
	case errT == nil && errE != nil:
		var zeroE E
		c.Then, c.Else, c.N = t, zeroE, 1
		return nil

	case errE == nil && errT != nil:
		var zeroT T
		c.Then, c.Else, c.N = zeroT, e, 2
		return nil

	case errT == nil:
		// Both decoded; try validation first to disambiguate
		var errValidateT, errValidateE error
		if v, ok := any(t).(Validator); ok {
			errValidateT = v.Validate()
		}
		if v, ok := any(e).(Validator); ok {
			errValidateE = v.Validate()
		}

		if errValidateT == nil && errValidateE != nil {
			var zeroE E
			c.Then, c.Else, c.N = t, zeroE, 1
			return nil
		}
		if errValidateE == nil && errValidateT != nil {
			var zeroT T
			c.Then, c.Else, c.N = zeroT, e, 2
			return nil
		}

		// Apply zero/meaningfulness heuristics, then tie-break to Then.
		nt := isNonZero(t)
		ne := isNonZero(e)

		if nt && !ne {
			var zeroE E
			c.Then, c.Else, c.N = t, zeroE, 1
			return nil
		}
		if ne && !nt {
			var zeroT T
			c.Then, c.Else, c.N = zeroT, e, 2
			return nil
		}

		// Tie: pick Then
		{
			var zeroE E
			c.Then, c.Else, c.N = t, zeroE, 1
			return nil
		}
	default:
		return ErrFailedToUnmarshalAsAOrB
	}
}

func (c *Conditional[T, E]) Validate() error {
	if c.IsThen() {
		if v, ok := any(c.Then).(Validator); ok {
			return v.Validate()
		}
		return nil
	}

	if c.IsElse() {
		if v, ok := any(c.Else).(Validator); ok {
			return v.Validate()
		}
		return nil
	}

	return nil
}
