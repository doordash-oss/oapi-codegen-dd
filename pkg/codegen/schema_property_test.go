// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProperty_GoTypeDef(t *testing.T) {
	type fields struct {
		Schema      GoSchema
		Constraints Constraints
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			// When pointer is skipped by setting flag SkipOptionalPointer, the
			// flag will never be pointer irrespective of other flags.
			name: "Set skip optional pointer type for go type",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: true,
					RefType:             "",
					GoType:              "int",
				},
				Constraints: Constraints{
					Nullable: ptr(true),
				},
			},
			want: "int",
		},

		{
			// if the field is optional, it will always be pointer irrespective of other
			// flags, given that pointer type is not skipped by setting SkipOptionalPointer
			// flag to true
			name: "When the field is optional",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: false,
					RefType:             "",
					GoType:              "int",
				},
				Constraints: Constraints{
					Nullable: ptr(true),
				},
			},
			want: "*int",
		},

		{
			// if the field(custom-type) is optional, it will NOT be a pointer if
			// SkipOptionalPointer flag is set to true
			name: "Set skip optional pointer type for ref type",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: true,
					RefType:             "CustomType",
					GoType:              "int",
				},
				Constraints: Constraints{
					Nullable: ptr(true),
				},
			},
			want: "CustomType",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Property{
				Schema:      tt.fields.Schema,
				Constraints: tt.fields.Constraints,
			}
			assert.Equal(t, tt.want, p.GoTypeDef())
		})
	}
}

func TestGenFieldsFromProperties_AdditionalTags(t *testing.T) {
	tests := []struct {
		name        string
		props       []Property
		options     ParseOptions
		contains    []string
		notContains []string
	}{
		{
			name: "yaml tag mirrors json tag",
			props: []Property{
				{
					GoName:        "ClosedAt",
					JsonFieldName: "closed_at",
					Schema:        GoSchema{GoType: "string"},
					Constraints:   Constraints{Required: ptr(true)},
				},
			},
			options: ParseOptions{
				AdditionalTags: []string{"yaml"},
				SkipValidation: true,
			},
			contains: []string{`json:"closed_at"`, `yaml:"closed_at"`},
		},
		{
			name: "yaml tag includes omitempty when nullable",
			props: []Property{
				{
					GoName:        "LabelName",
					JsonFieldName: "label_name",
					Schema:        GoSchema{GoType: "*string"},
					Constraints:   Constraints{Nullable: ptr(true)},
				},
			},
			options: ParseOptions{
				AdditionalTags: []string{"yaml"},
				SkipValidation: true,
			},
			contains: []string{`json:"label_name,omitempty"`, `yaml:"label_name,omitempty"`},
		},
		{
			name: "extra-tags take priority over additional-tags",
			props: []Property{
				{
					GoName:        "ExtraOverride",
					JsonFieldName: "extra_override",
					Schema:        GoSchema{GoType: "*string"},
					Constraints:   Constraints{Nullable: ptr(true)},
					Extensions: map[string]any{
						"x-oapi-codegen-extra-tags": map[string]any{
							"yaml": "custom_override",
						},
					},
				},
			},
			options: ParseOptions{
				AdditionalTags: []string{"yaml"},
				SkipValidation: true,
			},
			contains:    []string{`json:"extra_override,omitempty"`, `yaml:"custom_override"`},
			notContains: []string{`yaml:"extra_override,omitempty"`},
		},
		{
			name: "json ignored fields get yaml ignored too",
			props: []Property{
				{
					GoName:        "IgnoredField",
					JsonFieldName: "ignored_field",
					Schema:        GoSchema{GoType: "*string"},
					Extensions: map[string]any{
						"x-go-json-ignore": true,
					},
				},
			},
			options: ParseOptions{
				AdditionalTags: []string{"yaml"},
				SkipValidation: true,
			},
			contains: []string{`json:"-"`, `yaml:"-"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := genFieldsFromProperties(tt.props, tt.options)
			assert.Len(t, fields, 1)
			for _, s := range tt.contains {
				assert.Contains(t, fields[0], s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, fields[0], s)
			}
		})
	}
}

func TestProperty_GoTypeDef_nullable(t *testing.T) {
	type fields struct {
		GlobalStateDisableRequiredReadOnlyAsPointer bool
		Schema                                      GoSchema
		Constraints                                 Constraints
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			// Field not nullable.
			// When pointer is skipped by setting flag SkipOptionalPointer, the
			// flag will never be pointer irrespective of other flags.
			name: "Set skip optional pointer type for go type",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: true,
					RefType:             "",
					GoType:              "int",
				},
			},
			want: "int",
		},

		{
			// Field not nullable.
			// if the field is optional, it will always be pointer irrespective of other
			// flags, given that pointer type is not skipped by setting SkipOptionalPointer
			// flag to true
			name: "When the field is optional",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: false,
					RefType:             "",
					GoType:              "int",
				},
			},
			want: "int",
		},

		{
			// Field not nullable.
			// if the field(custom type) is optional, it will NOT be a pointer if
			// SkipOptionalPointer flag is set to true
			name: "Set skip optional pointer type for ref type",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: true,
					RefType:             "CustomType",
					GoType:              "int",
				},
			},
			want: "CustomType",
		},

		// Field not nullable.
		// For the following test case, SkipOptionalPointer flag is false.
		{
			name: "When field is required and not nullable",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: false,
					GoType:              "int",
				},
				Constraints: Constraints{
					Nullable: ptr(false),
					Required: ptr(true),
				},
			},
			want: "int",
		},

		{
			name: "When field is required and nullable",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: false,
					GoType:              "int",
				},
				Constraints: Constraints{
					Nullable: ptr(true),
					Required: ptr(true),
				},
			},
			want: "*int",
		},

		{
			name: "When field is optional and not nullable",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: false,
					GoType:              "*int",
				},
			},
			want: "*int",
		},

		{
			name: "When field is optional and nullable",
			fields: fields{
				Schema: GoSchema{
					SkipOptionalPointer: false,
					GoType:              "int",
				},
				Constraints: Constraints{
					Nullable: ptr(true),
				},
			},
			want: "*int",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Property{
				Schema:      tt.fields.Schema,
				Constraints: tt.fields.Constraints,
			}
			assert.Equal(t, tt.want, p.GoTypeDef())
		})
	}
}
