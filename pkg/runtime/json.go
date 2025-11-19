// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
//
// This is a port of the https://github.com/apapsch/go-jsonmerge/blob/master/merge.go package. The original package licensed under the MIT License.

package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}

// Merger describes result of merge operation and provides configuration.
type Merger struct {
	// Errors is slice of non-critical errors of merge operations
	Errors []error

	// Replaced is describe replacements
	// Key is path in document like
	//   "prop1.prop2.prop3" for object properties or
	//   "arr1.1.prop" for arrays
	// Value is value of replacement
	Replaced map[string]any

	// CopyNonexistent enables setting fields into the result
	// which only exist in the patch.
	CopyNonexistent bool
}

func (m *Merger) mergeValue(path []string, patch map[string]any, key string, value any) any {
	patchValue, patchHasValue := patch[key]

	if !patchHasValue {
		return value
	}

	_, patchValueIsObject := patchValue.(map[string]any)

	path = append(path, key)
	pathStr := strings.Join(path, ".")

	if _, ok := value.(map[string]any); ok {
		if !patchValueIsObject {
			err := fmt.Errorf("patch value must be object for key \"%v\"", pathStr)
			m.Errors = append(m.Errors, err)
			return value
		}

		return m.mergeObjects(value, patchValue, path)
	}

	if _, ok := value.([]interface{}); ok && patchValueIsObject {
		return m.mergeObjects(value, patchValue, path)
	}

	if !reflect.DeepEqual(value, patchValue) {
		m.Replaced[pathStr] = patchValue
	}

	return patchValue
}

func (m *Merger) mergeObjects(data, patch any, path []string) any {
	if patchObject, ok := patch.(map[string]any); ok {
		if dataArray, ok := data.([]any); ok {
			ret := make([]any, len(dataArray))

			for i, val := range dataArray {
				ret[i] = m.mergeValue(path, patchObject, strconv.Itoa(i), val)
			}

			return ret
		} else if dataObject, ok := data.(map[string]any); ok {
			ret := make(map[string]any)

			for k, v := range dataObject {
				ret[k] = m.mergeValue(path, patchObject, k, v)
			}

			if m.CopyNonexistent {
				for k, v := range patchObject {
					if _, ok := dataObject[k]; !ok {
						ret[k] = v
					}
				}
			}

			return ret
		}
	}

	return data
}

// Merge merges patch document to data document
//
// Returning merged document. Result of merge operation can be
// obtained from the Merger. Result information is discarded before
// merging.
func (m *Merger) Merge(data, patch any) any {
	m.Replaced = make(map[string]any)
	m.Errors = make([]error, 0)
	return m.mergeObjects(data, patch, nil)
}

// MergeBytesIndent merges patch document buffer to data document buffer
//
// # Use prefix and indent for set indentation like in json.MarshalIndent
//
// Returning merged document buffer and error if any.
func (m *Merger) MergeBytesIndent(dataBuff, patchBuff []byte, prefix, indent string) ([]byte, error) {
	var data, patch, merged any

	err := unmarshalJSON(dataBuff, &data)
	if err != nil {
		return nil, fmt.Errorf("error in data JSON: %v", err)
	}

	err = unmarshalJSON(patchBuff, &patch)
	if err != nil {
		return nil, fmt.Errorf("error in patch JSON: %v", err)
	}

	merged = m.Merge(data, patch)

	mergedBuff, err := json.MarshalIndent(merged, prefix, indent)
	if err != nil {
		return nil, fmt.Errorf("error writing merged JSON: %v", err)
	}

	return mergedBuff, nil
}

// MergeBytes merges patch document buffer to data document buffer
//
// Returning merged document buffer, merge info and
// error if any
func (m *Merger) MergeBytes(dataBuff, patchBuff []byte) ([]byte, error) {
	var data, patch, merged any

	err := unmarshalJSON(dataBuff, &data)
	if err != nil {
		return nil, fmt.Errorf("error in data JSON: %v", err)
	}

	err = unmarshalJSON(patchBuff, &patch)
	if err != nil {
		return nil, fmt.Errorf("error in patch JSON: %v", err)
	}

	merged = m.Merge(data, patch)

	mergedBuff, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("error writing merged JSON: %v", err)
	}

	return mergedBuff, err
}

func unmarshalJSON(buff []byte, data any) error {
	decoder := json.NewDecoder(bytes.NewReader(buff))
	decoder.UseNumber()

	return decoder.Decode(data)
}

// JSONMerge merges two JSON representation into a single object. `data` is the
// existing representation and `patch` is the new data to be merged in
func JSONMerge(data, patch json.RawMessage) (json.RawMessage, error) {
	merger := Merger{
		CopyNonexistent: true,
	}
	if data == nil {
		data = []byte(`{}`)
	}
	if patch == nil {
		patch = []byte(`{}`)
	}
	merged, err := merger.MergeBytes(data, patch)
	if err != nil {
		return nil, err
	}
	return merged, nil
}

// MarshalJSON marshals value respecting json.Marshaler.
func MarshalJSON(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}

	if m, ok := v.(Marshaler); ok {
		b, err := m.MarshalJSON()
		return b, err
	}
	return json.Marshal(v)
}

// UnmarshalJSON unmarshals data into v, respecting custom Unmarshaler.
func UnmarshalJSON(data []byte, v any) error {
	if v == nil {
		return fmt.Errorf("UnmarshalJSON: target is nil")
	}

	if u, ok := v.(Unmarshaler); ok {
		return u.UnmarshalJSON(data)
	}
	return json.Unmarshal(data, v)
}

type jsonKind int

const (
	kindNull jsonKind = iota
	kindObject
	kindArray
	kindScalar
)

func classify(b []byte) jsonKind {
	t := bytes.TrimSpace(b)
	if len(t) == 0 || bytes.Equal(t, []byte("null")) {
		return kindNull
	}
	switch t[0] {
	case '{':
		return kindObject
	case '[':
		return kindArray
	default:
		return kindScalar
	}
}

// CoalesceOrMerge implements generic wrapper semantics:
// - 0 non-null parts  -> "null"
// - 1 non-null part   -> that value as-is (object/array/scalar)
// - many, all objects -> merged object (last wins)
// - many, all arrays  -> concatenated array
// - otherwise         -> error (ambiguous)
func CoalesceOrMerge(parts ...json.RawMessage) (json.RawMessage, error) {
	nonNull := make([]json.RawMessage, 0, len(parts))
	kinds := make([]jsonKind, 0, len(parts))

	for _, p := range parts {
		k := classify(p)
		if k != kindNull {
			nonNull = append(nonNull, bytes.TrimSpace(p))
			kinds = append(kinds, k)
		}
	}

	switch len(nonNull) {
	case 0:
		return json.RawMessage("null"), nil
	case 1:
		return nonNull[0], nil
	}

	all := func(k jsonKind) bool {
		for _, kk := range kinds {
			if kk != k {
				return false
			}
		}
		return true
	}

	// Merge objects (last-wins) using your existing JSONMerge.
	if all(kindObject) {
		merged := json.RawMessage(`{}`)
		for _, obj := range nonNull {
			m, err := JSONMerge(merged, obj)
			if err != nil {
				return nil, err
			}
			merged = m
		}
		return merged, nil
	}

	// Optional: concat arrays if all arrays.
	if all(kindArray) {
		var out bytes.Buffer
		out.WriteByte('[')
		first := true
		for _, arr := range nonNull {
			var elems []json.RawMessage
			if err := json.Unmarshal(arr, &elems); err != nil {
				return nil, fmt.Errorf("array branch invalid: %w", err)
			}
			for _, e := range elems {
				if !first {
					out.WriteByte(',')
				}
				first = false
				out.Write(e)
			}
		}
		out.WriteByte(']')
		return out.Bytes(), nil
	}

	return nil, fmt.Errorf("cannot combine %d non-null branches of mixed/unsupported kinds", len(nonNull))
}
