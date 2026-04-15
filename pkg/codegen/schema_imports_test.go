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
