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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoImports_FiltersHardcodedTemplateImports(t *testing.T) {
	im := importMap{
		"encoding/json": goImport{Path: "encoding/json"},
		"net/http":      goImport{Path: "net/http"},
		"custom/pkg":    goImport{Path: "custom/pkg"},
		"another/pkg":   goImport{Name: "alias", Path: "another/pkg"},
		"aliased-json":  goImport{Name: "myjson", Path: "encoding/json"},
	}

	result := im.GoImports()

	// encoding/json and net/http without aliases are hardcoded in header.tmpl, so they should be filtered out
	assert.NotContains(t, result, `"encoding/json"`)
	assert.NotContains(t, result, `"net/http"`)

	// aliased hardcoded imports should still be present (user explicitly wants the alias)
	assert.Contains(t, result, `myjson "encoding/json"`)

	// custom imports should still be present
	assert.Contains(t, result, `"custom/pkg"`)
	assert.Contains(t, result, `alias "another/pkg"`)
	assert.Len(t, result, 3)
}

func TestGoImports_FiltersCurrentPackage(t *testing.T) {
	im := importMap{
		"-":          goImport{Path: importMappingCurrentPackage},
		"custom/pkg": goImport{Path: "custom/pkg"},
	}

	result := im.GoImports()

	assert.Len(t, result, 1)
	assert.Contains(t, result, `"custom/pkg"`)
}

func TestGoImports_EmptyMap(t *testing.T) {
	im := importMap{}
	result := im.GoImports()
	assert.Empty(t, result)
}

func TestGoTypePackages(t *testing.T) {
	tests := []struct {
		goType string
		want   []string
	}{
		{"uuid.UUID", []string{"uuid"}},
		{"*uuid.UUID", []string{"uuid"}},
		{"[]uuid.UUID", []string{"uuid"}},
		{"map[string]uuid.UUID", []string{"uuid"}},
		{"map[uuid.UUID]uuid.UUID", []string{"uuid"}},
		{"map[a.Key]b.Value", []string{"a", "b"}},
		{"struct{ID uuid.UUID}", []string{"uuid"}},
		{"Pet", nil},
		{"[]string", nil},
		{"uuid.", nil},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			assert.Equal(t, tt.want, goTypePackages(tt.goType))
		})
	}
}

func TestWarnUnimportedGoTypes(t *testing.T) {
	uuidImport := goImport{Path: "github.com/google/uuid"}

	t.Run("lists a few schemas once each and counts the rest", func(t *testing.T) {
		logs := captureLogs(t)

		warnUnimportedGoTypes(importMap{}, nil, map[string][]string{
			"uuid": {"G", "F", "E", "D", "C", "B", "A", "A", "H"},
		})

		out := logs.String()
		assert.Equal(t, 1, strings.Count(out, "WARN"))
		assert.Contains(t, out, `schemas="A, B, C, D, E and 3 more"`)
	})

	t.Run("ignores packages the generator does not import for its own types", func(t *testing.T) {
		logs := captureLogs(t)

		warnUnimportedGoTypes(importMap{}, nil, map[string][]string{"decimal": {"A"}, "time": {"B"}})

		assert.Empty(t, logs.String())
	})

	t.Run("is silent once an import binds the package", func(t *testing.T) {
		tests := []struct {
			name       string
			imports    importMap
			additional []AdditionalImport
		}{
			{"import", importMap{uuidImport.String(): uuidImport}, nil},
			{"additional import", importMap{}, []AdditionalImport{{Package: uuidImport.Path}}},
			{"additional import aliased as the package", importMap{}, []AdditionalImport{{Alias: "uuid", Package: uuidImport.Path}}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				logs := captureLogs(t)

				warnUnimportedGoTypes(tt.imports, tt.additional, map[string][]string{"uuid": {"A"}})

				assert.Empty(t, logs.String())
			})
		}
	})
}
