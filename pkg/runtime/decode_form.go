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
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// maxFormDepth stops typing through a type that hands its value on to itself.
const maxFormDepth = 256

var (
	formMemberedType = reflect.TypeFor[formMembered]()
	unmarshalerType  = reflect.TypeFor[json.Unmarshaler]()
	rawMessageType   = reflect.TypeFor[json.RawMessage]()
	errorType        = reflect.TypeFor[error]()

	formShapes sync.Map
)

// formMembered is a union that can list its member types.
type formMembered interface {
	formMembers() []reflect.Type
}

// formShape is how a Go type takes a decoded value, as far as typing a form value needs.
type formShape struct {
	fields  map[string]reflect.Type // by JSON name
	parts   []reflect.Type          // json:"-" fields a type that decodes itself hands the whole value to
	extra   reflect.Type            // the value type of AdditionalProperties, or of a map
	members []reflect.Type          // union members
	opaque  bool                    // decodes itself in a way none of the above describes
}

// UnmarshalForm decodes a form-encoded body into target, typing each value by the field it lands in rather than by its text.
func UnmarshalForm(data []byte, target any) error {
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return fmt.Errorf("error parsing form-encoded body: %w", err)
	}

	typed := typeFormValue(reflect.TypeOf(target), decodeFormData(values, keepFormString), 0)
	jsonBytes, err := json.Marshal(typed)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, target)
}

// lookup finds the type a key of an object lands in, through the parts and members that receive the object too.
func (shape *formShape) lookup(key string) reflect.Type {
	return shape.lookupSeen(key, map[*formShape]bool{})
}

func (shape *formShape) lookupSeen(key string, seen map[*formShape]bool) reflect.Type {
	if seen[shape] {
		return nil
	}
	seen[shape] = true

	if t, ok := shape.fields[key]; ok {
		return t
	}
	for _, group := range [][]reflect.Type{shape.parts, shape.members} {
		for _, t := range group {
			if found := formShapeOf(derefType(t)).lookupSeen(key, seen); found != nil {
				return found
			}
		}
	}
	return shape.extra
}

func keepFormString(s string) any {
	return s
}

func typeFormValue(t reflect.Type, v any, depth int) any {
	if v == nil {
		return nil
	}
	if t != nil {
		t = derefType(t)
	}
	if t == nil || t.Kind() == reflect.Interface || depth > maxFormDepth {
		return guessFormValue(v)
	}

	shape := formShapeOf(t)
	switch {
	case shape.opaque:
		return guessFormValue(v)
	case len(shape.members) > 0:
		return typeFormUnion(shape, v, depth)
	case len(shape.parts) == 1 && len(shape.fields) == 0 && shape.extra == nil:
		return typeFormValue(shape.parts[0], v, depth+1)
	}

	switch val := v.(type) {
	case map[string]any:
		return typeFormObject(t, shape, val, depth)
	case []any:
		return typeFormList(t, val, depth)
	case string:
		return typeFormString(t, val, depth)
	default:
		return v
	}
}

func typeFormObject(t reflect.Type, shape *formShape, obj map[string]any, depth int) any {
	if t.Kind() != reflect.Struct && t.Kind() != reflect.Map {
		return guessFormValue(obj)
	}

	out := make(map[string]any, len(obj))
	for key, v := range obj {
		out[key] = typeFormValue(shape.lookup(key), v, depth+1)
	}
	return out
}

func typeFormList(t reflect.Type, list []any, depth int) any {
	if t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
		return guessFormValue(list)
	}

	out := make([]any, len(list))
	for i, v := range list {
		out[i] = typeFormValue(t.Elem(), v, depth+1)
	}
	return out
}

// typeFormString leaves s a string when it does not parse as the kind t holds, so the decoder names the field.
func typeFormString(t reflect.Type, s string, depth int) any {
	switch t.Kind() {
	case reflect.Bool:
		switch s {
		case "true":
			return true
		case "false":
			return false
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			return u
		}
	case reflect.Float32, reflect.Float64:
		if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return f
		}
	case reflect.Slice, reflect.Array:
		// A lone value of an exploded array; []byte is a base64 string instead.
		if t.Elem().Kind() != reflect.Uint8 {
			return []any{typeFormValue(t.Elem(), s, depth+1)}
		}
	default:
		// Every other kind takes the string as it is.
	}
	return s
}

// typeFormUnion types v for the first member it decodes into and validates as, else the first it decodes into, and the union's own fields over that.
func typeFormUnion(shape *formShape, v any, depth int) any {
	var typed, fallback any
	for _, member := range shape.members {
		candidate := typeFormValue(member, v, depth+1)
		decodes, valid := decodesAs(member, candidate)
		if decodes && valid {
			typed = candidate
			break
		}
		if decodes && fallback == nil {
			fallback = candidate
		}
	}
	if typed == nil {
		typed = fallback
	}
	if typed == nil {
		typed = guessFormValue(v)
	}

	obj, isObj := v.(map[string]any)
	out, isOut := typed.(map[string]any)
	if isObj && isOut {
		for key, fv := range obj {
			if ft, ok := shape.fields[key]; ok {
				out[key] = typeFormValue(ft, fv, depth+1)
			}
		}
	}
	return typed
}

// decodesAs reports whether v decodes into t, and whether the result passes t's Validate if it has one.
func decodesAs(t reflect.Type, v any) (decodes, valid bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return false, false
	}
	target := reflect.New(t).Interface()
	if json.Unmarshal(b, target) != nil {
		return false, false
	}
	if validator, ok := target.(Validator); ok {
		return true, validator.Validate() == nil
	}
	return true, true
}

func formShapeOf(t reflect.Type) *formShape {
	if shape, ok := formShapes.Load(t); ok {
		return shape.(*formShape)
	}
	shape, _ := formShapes.LoadOrStore(t, buildFormShape(t))
	return shape.(*formShape)
}

func buildFormShape(t reflect.Type) *formShape {
	shape := &formShape{}
	if reflect.PointerTo(t).Implements(formMemberedType) {
		shape.members = reflect.New(t).Interface().(formMembered).formMembers()
	}

	switch t.Kind() {
	case reflect.Map:
		shape.extra = t.Elem()
		return shape
	case reflect.Struct:
	default:
		return shape
	}

	if shape.members == nil {
		shape.members = rawUnionMembers(t)
	}
	decodesItself := reflect.PointerTo(t).Implements(unmarshalerType)
	shape.fields = map[string]reflect.Type{}
	collectFormFields(t, shape, decodesItself, map[reflect.Type]bool{})
	shape.opaque = decodesItself && len(shape.members) == 0 && len(shape.fields) == 0 &&
		len(shape.parts) == 0 && shape.extra == nil
	return shape
}

// collectFormFields reads t's fields the way encoding/json does: an outer field wins over one promoted from an embedded struct.
func collectFormFields(t reflect.Type, shape *formShape, decodesItself bool, seen map[reflect.Type]bool) {
	if seen[t] {
		return
	}
	seen[t] = true

	var embedded []reflect.Type
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		inner := derefType(f.Type)

		switch {
		case name == "-":
			if !decodesItself || !f.IsExported() {
				continue
			}
			if f.Name == "AdditionalProperties" && f.Type.Kind() == reflect.Map {
				shape.extra = f.Type.Elem()
			} else {
				shape.parts = append(shape.parts, f.Type)
			}
		case f.Anonymous && name == "" && inner.Kind() == reflect.Struct:
			if !reflect.PointerTo(inner).Implements(formMemberedType) {
				embedded = append(embedded, inner)
			}
		case f.IsExported():
			if name == "" {
				name = f.Name
			}
			if _, ok := shape.fields[name]; !ok {
				shape.fields[name] = f.Type
			}
		}
	}

	for _, inner := range embedded {
		collectFormFields(inner, shape, false, seen)
	}
}

// rawUnionMembers reads the members of a generated union kept as raw JSON off its As<Member> accessors.
func rawUnionMembers(t reflect.Type) []reflect.Type {
	if f, ok := t.FieldByName("union"); !ok || f.Type != rawMessageType {
		return nil
	}

	var members []reflect.Type
	seen := map[reflect.Type]bool{}
	pt := reflect.PointerTo(t)
	for i := 0; i < pt.NumMethod(); i++ {
		m := pt.Method(i)
		if !strings.HasPrefix(m.Name, "As") || m.Type.NumIn() != 1 || m.Type.NumOut() != 2 || m.Type.Out(1) != errorType {
			continue
		}
		if member := m.Type.Out(0); !seen[member] {
			seen[member] = true
			members = append(members, member)
		}
	}
	return members
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// guessFormValue types v as ConvertFormFields does, for a target that does not say.
func guessFormValue(v any) any {
	switch val := v.(type) {
	case string:
		return convertFormStringValue(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = guessFormValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(val))
		for key, item := range val {
			out[key] = guessFormValue(item)
		}
		return out
	}
	return v
}
