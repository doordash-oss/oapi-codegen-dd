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
					Nullable: true,
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
					Nullable: true,
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
					Nullable: true,
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
					Nullable: false,
					Required: true,
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
					Nullable: true,
					Required: true,
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
					Nullable: true,
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
