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

func TestModelsQualifierQualify(t *testing.T) {
	names := map[string]GoSchema{
		"User":     {},
		"NewUser":  {},
		"Response": {}, // deliberately collides with a struct field-ish word
	}

	tests := []struct {
		name  string
		alias string
		expr  string
		want  string
	}{
		{
			name:  "no alias is a no-op",
			alias: "",
			expr:  "*User",
			want:  "*User",
		},
		{
			name:  "bare identifier",
			alias: "models",
			expr:  "User",
			want:  "models.User",
		},
		{
			name:  "pointer",
			alias: "models",
			expr:  "*User",
			want:  "*models.User",
		},
		{
			name:  "slice",
			alias: "models",
			expr:  "[]User",
			want:  "[]models.User",
		},
		{
			name:  "slice of pointer",
			alias: "models",
			expr:  "[]*User",
			want:  "[]*models.User",
		},
		{
			name:  "map value",
			alias: "models",
			expr:  "map[string]User",
			want:  "map[string]models.User",
		},
		{
			name:  "primitive is untouched",
			alias: "models",
			expr:  "string",
			want:  "string",
		},
		{
			name:  "empty struct is untouched",
			alias: "models",
			expr:  "struct{}",
			want:  "struct{}",
		},
		{
			name:  "already-qualified selector is left alone",
			alias: "models",
			expr:  "time.Time",
			want:  "time.Time",
		},
		{
			name:  "unrelated type name is untouched",
			alias: "models",
			expr:  "int64",
			want:  "int64",
		},
		{
			name:  "two model types in one expression",
			alias: "models",
			expr:  "map[string]NewUser",
			want:  "map[string]models.NewUser",
		},
		{
			name:  "unparsable expression is returned unchanged",
			alias: "models",
			expr:  "not[[a valid(( type",
			want:  "not[[a valid(( type",
		},
		{
			name:  "empty expression is returned unchanged",
			alias: "models",
			expr:  "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := newModelsQualifier(tc.alias, names)
			assert.Equal(t, tc.want, q.qualify(tc.expr))
		})
	}
}

func TestModelsQualifierPrefix(t *testing.T) {
	assert.Equal(t, "", newModelsQualifier("", nil).prefix())
	assert.Equal(t, "models.", newModelsQualifier("models", nil).prefix())
}

func TestModelsQualifierNilReceiver(t *testing.T) {
	var q *modelsQualifier
	assert.Equal(t, "*User", q.qualify("*User"))
	assert.Equal(t, "", q.prefix())
}
