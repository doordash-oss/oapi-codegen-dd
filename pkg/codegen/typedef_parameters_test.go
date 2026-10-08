// Copyright 2026 DoorDash, Inc.
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

func TestSliceTypeDefinition(t *testing.T) {
	tracker := newTypeTracker()
	tracker.register(TypeDefinition{Name: "TagList", Schema: GoSchema{GoType: "[]string", ArrayType: &GoSchema{GoType: "string"}}}, "")
	tracker.register(TypeDefinition{Name: "Tags", Schema: GoSchema{GoType: "TagList", DefineViaAlias: true}}, "")
	tracker.register(TypeDefinition{Name: "Name", Schema: GoSchema{GoType: "string"}}, "")
	tracker.register(TypeDefinition{Name: "Self", Schema: GoSchema{GoType: "Self", DefineViaAlias: true}}, "")

	tests := []struct {
		name     string
		typeName string
		want     string
	}{
		{name: "slice type", typeName: "TagList", want: "TagList"},
		{name: "alias of a slice type", typeName: "Tags", want: "TagList"},
		{name: "not a slice", typeName: "Name"},
		{name: "unknown name", typeName: "[]string"},
		{name: "alias of itself", typeName: "Self"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := sliceTypeDefinition(tt.typeName, tracker)
			if tt.want == "" {
				assert.Nil(t, td)
				return
			}
			if assert.NotNil(t, td) {
				assert.Equal(t, tt.want, td.Name)
			}
		})
	}
}
