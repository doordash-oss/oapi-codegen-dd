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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type formAddress struct {
	City       string `json:"city"`
	PostalCode string `json:"postal_code"`
}

// formAddressAnyOf mirrors a generated anyOf of an object and the empty string.
type formAddressAnyOf struct {
	Either[formAddress, string]
}

// formAddressField mirrors the wrapper generated around a union property.
type formAddressField struct {
	AnyOf *formAddressAnyOf `json:"-"`
}

func (f *formAddressField) UnmarshalJSON(data []byte) error {
	f.AnyOf = &formAddressAnyOf{}
	return UnmarshalJSON(data, f.AnyOf)
}

// formNameAnyOf mirrors a generated union kept as raw JSON.
type formNameAnyOf struct {
	union json.RawMessage
}

func (p *formNameAnyOf) AsString() (string, error) {
	return UnmarshalAs[string](p.union)
}

func (p *formNameAnyOf) AsValidatedString() (string, error) {
	return p.AsString()
}

func (p *formNameAnyOf) UnmarshalJSON(data []byte) error {
	return p.union.UnmarshalJSON(data)
}

// formLabelledUnion mirrors a raw union that also declares its own properties.
type formLabelledUnion struct {
	Label *string `json:"label,omitempty"`
	union json.RawMessage
}

func (p *formLabelledUnion) AsFormAddress() (formAddress, error) {
	return UnmarshalAs[formAddress](p.union)
}

func (p *formLabelledUnion) UnmarshalJSON(data []byte) error {
	if err := p.union.UnmarshalJSON(data); err != nil {
		return err
	}
	var named struct {
		Label *string `json:"label"`
	}
	if err := json.Unmarshal(data, &named); err != nil {
		return err
	}
	p.Label = named.Label
	return nil
}

// formMixed mirrors a generated struct with named properties and an inline part.
type formMixed struct {
	Label string            `json:"label"`
	Part  *formAddressAnyOf `json:"-"`
}

func (m *formMixed) UnmarshalJSON(data []byte) error {
	type alias formMixed
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*m = formMixed(tmp)
	m.Part = &formAddressAnyOf{}
	return UnmarshalJSON(data, m.Part)
}

// formCounts mirrors a generated struct with additionalProperties.
type formCounts struct {
	Kind                 string           `json:"kind"`
	AdditionalProperties map[string]int64 `json:"-"`
}

func (f *formCounts) UnmarshalJSON(data []byte) error {
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	if raw, ok := object["kind"]; ok {
		if err := json.Unmarshal(raw, &f.Kind); err != nil {
			return err
		}
		delete(object, "kind")
	}
	f.AdditionalProperties = map[string]int64{}
	for key, raw := range object {
		var v int64
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		f.AdditionalProperties[key] = v
	}
	return nil
}

type formItem struct {
	ID  int64  `json:"id"`
	SKU string `json:"sku"`
}

type formEmbedded struct {
	Code string `json:"code"`
}

type formBody struct {
	formEmbedded

	Amount      int64                             `json:"amount"`
	Quantity    uint32                            `json:"quantity"`
	Rate        float64                           `json:"rate"`
	Confirm     bool                              `json:"confirm"`
	Description string                            `json:"description"`
	Reference   *string                           `json:"reference,omitempty"`
	Address     *formAddressField                 `json:"address,omitempty"`
	Name        *formNameAnyOf                    `json:"name,omitempty"`
	Labelled    *formLabelledUnion                `json:"labelled,omitempty"`
	Mixed       *formMixed                        `json:"mixed,omitempty"`
	Counts      *formCounts                       `json:"counts,omitempty"`
	Limit       *Either[int64, string]            `json:"limit,omitempty"`
	Flag        *Either[bool, string]             `json:"flag,omitempty"`
	Shipping    *Conditional[formAddress, string] `json:"shipping,omitempty"`
	Tags        []string                          `json:"tags,omitempty"`
	Items       []formItem                        `json:"items,omitempty"`
	Metadata    map[string]string                 `json:"metadata,omitempty"`
	Extra       any                               `json:"extra,omitempty"`
	Raw         []byte                            `json:"raw,omitempty"`
	Day         *Date                             `json:"day,omitempty"`
	At          *time.Time                        `json:"at,omitempty"`
}

// formLoop hands its whole value to itself, which never bottoms out.
type formLoop struct {
	Loop *formLoop `json:"-"`
}

func (l *formLoop) UnmarshalJSON([]byte) error {
	return nil
}

type formCycleA struct {
	B    *formCycleB `json:"-"`
	Name string      `json:"name"`
}

func (a *formCycleA) UnmarshalJSON([]byte) error {
	return nil
}

type formCycleB struct {
	A    *formCycleA `json:"-"`
	Code string      `json:"code"`
}

func (b *formCycleB) UnmarshalJSON([]byte) error {
	return nil
}

type FormSelfEmbed struct {
	*FormSelfEmbed
	Name string `json:"name"`
}

type formInner struct {
	Name  int64  `json:"name"`
	Inner string `json:"inner"`
}

type formOuter struct {
	formInner
	Name string `json:"name"`
}

type formEmbedsUnion struct {
	Either[int64, string]
}

type formEmbedsUnionPointer struct {
	*Either[int64, string]
}

type FormLabel string

type formPlain struct {
	FormLabel
	Skipped  string `json:"-"`
	Name     string `json:"name"`
	Untagged string
}

type formWithHidden struct {
	hidden *formAddressAnyOf `json:"-"`
	Name   string            `json:"name"`
}

func (f *formWithHidden) UnmarshalJSON([]byte) error {
	f.hidden = nil
	return nil
}

// formNotAUnion has a union field of the wrong type and accessors that do not match.
type formNotAUnion struct {
	union string
}

func (f *formNotAUnion) UnmarshalJSON(data []byte) error {
	f.union = string(data)
	return nil
}

type formOddAccessors struct {
	union json.RawMessage
}

func (f *formOddAccessors) Asset() string                { return "" }
func (f *formOddAccessors) AsOne(int) (string, error)    { return "", nil }
func (f *formOddAccessors) AsTwo() (string, string)      { return "", "" }
func (f *formOddAccessors) UnmarshalJSON(b []byte) error { return f.union.UnmarshalJSON(b) }
func (f *formOddAccessors) Marshal() json.RawMessage     { return f.union }

// formEmail and formPhone decode from any object; only validation tells them apart.
type formEmail struct {
	Email string `json:"email"`
}

func (e formEmail) Validate() error {
	if !strings.Contains(e.Email, "@") {
		return assert.AnError
	}
	return nil
}

type formPhone struct {
	Phone string `json:"phone"`
}

func (p formPhone) Validate() error {
	if p.Phone == "" {
		return assert.AnError
	}
	return nil
}

func TestUnmarshalForm(t *testing.T) {
	decode := func(t *testing.T, form string) formBody {
		t.Helper()
		var body formBody
		require.NoError(t, UnmarshalForm([]byte(form), &body))
		return body
	}

	t.Run("digit strings stay strings where the field is a string", func(t *testing.T) {
		body := decode(t, "description=12345&reference=0042&code=007&address[city]=Berlin&address[postal_code]=10115"+
			"&name=4155551234&metadata[order]=42&tags[0]=7&items[0][id]=1&items[0][sku]=0012")

		assert.Equal(t, "12345", body.Description)
		assert.Equal(t, "0042", *body.Reference)
		assert.Equal(t, "007", body.Code)
		require.True(t, body.Address.AnyOf.IsA())
		assert.Equal(t, formAddress{City: "Berlin", PostalCode: "10115"}, body.Address.AnyOf.A)
		name, err := body.Name.AsString()
		require.NoError(t, err)
		assert.Equal(t, "4155551234", name)
		assert.Equal(t, map[string]string{"order": "42"}, body.Metadata)
		assert.Equal(t, []string{"7"}, body.Tags)
		assert.Equal(t, []formItem{{ID: 1, SKU: "0012"}}, body.Items)
	})

	t.Run("numbers and bools where the field holds them", func(t *testing.T) {
		body := decode(t, "amount=2000&quantity=3&rate=1.5&confirm=true")

		assert.Equal(t, int64(2000), body.Amount)
		assert.Equal(t, uint32(3), body.Quantity)
		assert.Equal(t, 1.5, body.Rate)
		assert.True(t, body.Confirm)
	})

	t.Run("a union takes the member the value fits", func(t *testing.T) {
		body := decode(t, "address=&limit=5&flag=false&shipping[postal_code]=10115")

		require.True(t, body.Address.AnyOf.IsB())
		assert.Equal(t, "", body.Address.AnyOf.B)
		require.True(t, body.Limit.IsA())
		assert.Equal(t, int64(5), body.Limit.A)
		require.True(t, body.Flag.IsA())
		assert.False(t, body.Flag.A)
		require.True(t, body.Shipping.IsThen())
		assert.Equal(t, "10115", body.Shipping.Then.PostalCode)

		body = decode(t, "limit=&flag=")
		require.True(t, body.Limit.IsB())
		require.True(t, body.Flag.IsB())
	})

	t.Run("a union's own properties are typed by their fields", func(t *testing.T) {
		body := decode(t, "labelled[label]=123&labelled[postal_code]=10115")

		assert.Equal(t, "123", *body.Labelled.Label)
		address, err := body.Labelled.AsFormAddress()
		require.NoError(t, err)
		assert.Equal(t, "10115", address.PostalCode)
	})

	t.Run("inline parts and additional properties", func(t *testing.T) {
		body := decode(t, "mixed[label]=123&mixed[postal_code]=10115&counts[kind]=7&counts[apples]=3")

		assert.Equal(t, "123", body.Mixed.Label)
		assert.Equal(t, "10115", body.Mixed.Part.A.PostalCode)
		assert.Equal(t, "7", body.Counts.Kind)
		assert.Equal(t, map[string]int64{"apples": 3}, body.Counts.AdditionalProperties)
	})

	t.Run("arrays", func(t *testing.T) {
		assert.Equal(t, []string{"solo"}, decode(t, "tags=solo").Tags)
		assert.Equal(t, []string{"1", "2"}, decode(t, "tags=1&tags=2").Tags)
		assert.Equal(t, []string{"", "x"}, decode(t, "tags[1]=x").Tags)
		assert.Equal(t, []byte("hi"), decode(t, "raw=aGk=").Raw)
	})

	t.Run("targets that do not say are guessed", func(t *testing.T) {
		body := decode(t, "extra[n]=5&day=2024-01-02&at=2024-01-02T03:04:05Z&unknown=1")

		assert.Equal(t, map[string]any{"n": float64(5)}, body.Extra)
		assert.Equal(t, []any{nil, "x"}, decode(t, "extra[1]=x").Extra)
		assert.Equal(t, "2024-01-02", body.Day.String())
		assert.Equal(t, 2024, body.At.Year())
	})

	t.Run("a value that does not parse names its field", func(t *testing.T) {
		var body formBody
		err := UnmarshalForm([]byte("amount=abc"), &body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "amount")

		for _, form := range []string{"quantity=-1", "rate=Inf", "confirm=yes", "description[x]=1", "description=a&description=b", "items=abc"} {
			assert.Error(t, UnmarshalForm([]byte(form), &body), form)
		}
	})

	t.Run("malformed form", func(t *testing.T) {
		var body formBody
		err := UnmarshalForm([]byte("amount=%zz"), &body)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error parsing form-encoded body")
	})
}

func TestTypeFormValue(t *testing.T) {
	t.Run("a type that hands its value to itself stops", func(t *testing.T) {
		assert.Equal(t, "x", typeFormValue(reflect.TypeFor[formLoop](), "x", 0))
	})

	t.Run("a key found nowhere in a cycle of parts is guessed", func(t *testing.T) {
		typed := typeFormValue(reflect.TypeFor[formCycleA](), map[string]any{"name": "1", "code": "2", "other": "3"}, 0)
		assert.Equal(t, map[string]any{"name": "1", "code": "2", "other": int64(3)}, typed)
	})

	t.Run("a value that is not from a form is left alone", func(t *testing.T) {
		assert.Equal(t, 5, typeFormValue(reflect.TypeFor[string](), 5, 0))
	})

	t.Run("a union no member fits is guessed", func(t *testing.T) {
		assert.Equal(t, []any{int64(1)}, typeFormValue(reflect.TypeFor[Either[int64, bool]](), []any{"1"}, 0))
	})

	t.Run("a union takes the member that validates", func(t *testing.T) {
		typed := typeFormValue(reflect.TypeFor[Either[formEmail, formPhone]](), map[string]any{"phone": "4155551234"}, 0)
		assert.Equal(t, map[string]any{"phone": "4155551234"}, typed)
	})

	t.Run("a union no member validates takes the first it decodes into", func(t *testing.T) {
		typed := typeFormValue(reflect.TypeFor[Either[formEmail, formPhone]](), map[string]any{"email": "123"}, 0)
		assert.Equal(t, map[string]any{"email": "123"}, typed)
	})
}

func TestFormShapeOf(t *testing.T) {
	t.Run("plain struct", func(t *testing.T) {
		shape := formShapeOf(reflect.TypeFor[formPlain]())
		assert.Equal(t, map[string]reflect.Type{
			"FormLabel": reflect.TypeFor[FormLabel](),
			"name":      reflect.TypeFor[string](),
			"Untagged":  reflect.TypeFor[string](),
		}, shape.fields)
		assert.Empty(t, shape.parts)
		assert.False(t, shape.opaque)
	})

	t.Run("an embedded struct does not loop", func(t *testing.T) {
		shape := formShapeOf(reflect.TypeFor[FormSelfEmbed]())
		assert.Equal(t, map[string]reflect.Type{"name": reflect.TypeFor[string]()}, shape.fields)
	})

	t.Run("an outer field wins over one promoted from an embedded struct", func(t *testing.T) {
		shape := formShapeOf(reflect.TypeFor[formOuter]())
		assert.Equal(t, map[string]reflect.Type{"name": reflect.TypeFor[string](), "inner": reflect.TypeFor[string]()}, shape.fields)
	})

	t.Run("an embedded union lists its members and adds no fields", func(t *testing.T) {
		shape := formShapeOf(reflect.TypeFor[formEmbedsUnion]())
		assert.Equal(t, []reflect.Type{reflect.TypeFor[int64](), reflect.TypeFor[string]()}, shape.members)
		assert.Empty(t, shape.fields)
	})

	t.Run("a union embedded by pointer lists its members", func(t *testing.T) {
		shape := formShapeOf(reflect.TypeFor[formEmbedsUnionPointer]())
		assert.Equal(t, []reflect.Type{reflect.TypeFor[int64](), reflect.TypeFor[string]()}, shape.members)
		assert.Empty(t, shape.fields)
	})

	t.Run("an unexported inline part is not one", func(t *testing.T) {
		shape := formShapeOf(reflect.TypeFor[formWithHidden]())
		assert.Empty(t, shape.parts)
		assert.Equal(t, map[string]reflect.Type{"name": reflect.TypeFor[string]()}, shape.fields)
	})

	t.Run("a type that decodes itself in its own way is opaque", func(t *testing.T) {
		assert.True(t, formShapeOf(reflect.TypeFor[formNotAUnion]()).opaque)
		assert.True(t, formShapeOf(reflect.TypeFor[time.Time]()).opaque)
	})

	t.Run("raw union members come from their accessors", func(t *testing.T) {
		assert.Equal(t, []reflect.Type{reflect.TypeFor[string]()}, formShapeOf(reflect.TypeFor[formNameAnyOf]()).members)
		assert.Empty(t, formShapeOf(reflect.TypeFor[formOddAccessors]()).members)
		assert.True(t, formShapeOf(reflect.TypeFor[formOddAccessors]()).opaque)
	})

	t.Run("maps and scalars", func(t *testing.T) {
		assert.Equal(t, reflect.TypeFor[int64](), formShapeOf(reflect.TypeFor[map[string]int64]()).extra)
		assert.Equal(t, &formShape{}, formShapeOf(reflect.TypeFor[string]()))
	})
}

func TestDecodesAs(t *testing.T) {
	decodes, valid := decodesAs(reflect.TypeFor[int64](), int64(1))
	assert.True(t, decodes)
	assert.True(t, valid)

	decodes, _ = decodesAs(reflect.TypeFor[int64](), "x")
	assert.False(t, decodes)

	decodes, _ = decodesAs(reflect.TypeFor[int64](), make(chan int))
	assert.False(t, decodes)

	decodes, valid = decodesAs(reflect.TypeFor[formPhone](), map[string]any{})
	assert.True(t, decodes)
	assert.False(t, valid)
}
