// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

// Copyright 2019 DeepMap, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		str  string
		want string
	}{{
		str:  "",
		want: "",
	}, {
		str:  " foo_bar ",
		want: "FooBar",
	}, {
		str:  "hi hello-hey-hallo",
		want: "HiHelloHeyHallo",
	}, {
		str:  "foo#bar",
		want: "FooBar",
	}, {
		str:  "foo2bar",
		want: "Foo2Bar",
	}, {
		// Test that each substitution works
		str:  "word.word-WORD+Word_word~word(Word)Word{Word}Word[Word]Word:Word;",
		want: "WordWordWORDWordWordWordWordWordWordWordWordWordWord",
	}, {
		// Make sure numbers don't interact in a funny way.
		str:  "number-1234",
		want: "Number1234",
	},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.str, func(t *testing.T) {
			require.Equal(t, tt.want, toCamelCase(tt.str))
		})
	}
}

func TestToCamelCaseWithInitialisms(t *testing.T) {
	tests := []struct {
		str  string
		want string
	}{{
		str:  "",
		want: "",
	}, {
		str:  "hello",
		want: "Hello",
	}, {
		str:  "DBError",
		want: "DBError",
	}, {
		str:  "httpOperationId",
		want: "HTTPOperationID",
	}, {
		str:  "OperationId",
		want: "OperationID",
	}, {
		str:  "peer2peer",
		want: "Peer2Peer",
	}, {
		str:  "makeUtf8",
		want: "MakeUTF8",
	}, {
		str:  "utf8Hello",
		want: "UTF8Hello",
	}, {
		str:  "myDBError",
		want: "MyDBError",
	}, {
		str:  " DbLayer ",
		want: "DBLayer",
	}, {
		str:  "FindPetById",
		want: "FindPetByID",
	}, {
		str:  "MyHttpUrl",
		want: "MyHTTPURL",
	}, {
		str:  "find_user_by_uuid",
		want: "FindUserByUUID",
	}, {
		str:  "HelloПриветWorldМир42",
		want: "HelloПриветWorldМир42",
	}, {
		str:  "пир2пир",
		want: "Пир2Пир",
	}}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.str, func(t *testing.T) {
			require.Equal(t, tt.want, toCamelCaseWithInitialism(tt.str))
		})
	}
}

func TestRefPathToGoType(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		goType string
	}{
		{
			name:   "local-schemas",
			path:   "#/components/schemas/Foo",
			goType: "Foo",
		},
		{
			name:   "local-parameters",
			path:   "#/components/parameters/foo_bar",
			goType: "FooBar",
		},
		{
			name:   "local-responses",
			path:   "#/components/responses/wibble",
			goType: "Wibble",
		},
		{
			name: "local-too-deep",
			path: "#/components/parameters/foo/components/bar",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			goType, err := refPathToGoType(tc.path)
			if tc.goType == "" {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.goType, goType)
		})
	}
}

func TestOrderedParamsFromUri(t *testing.T) {
	result := orderedParamsFromUri("/path/{param1}/{.param2}/{;param3*}/foo")
	assert.EqualValues(t, []string{"param1", "param2", "param3"}, result)

	result = orderedParamsFromUri("/path/foo")
	assert.EqualValues(t, []string{}, result)
}

func TestReplacePathParamsWithStr(t *testing.T) {
	result := replacePathParamsWithStr("/path/{param1}/{.param2}/{;param3*}/foo")
	assert.EqualValues(t, "/path/%s/%s/%s/foo", result)
}

func TestStringToGoComment(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
		message  string
	}{
		{
			input:    "",
			expected: "",
			message:  "blank string should be ignored due to human unreadable",
		},
		{
			input:    " ",
			expected: "",
			message:  "whitespace should be ignored due to human unreadable",
		},
		{
			input:    "Single Line",
			expected: "// Single Line",
			message:  "single line comment",
		},
		{
			input:    "    Single Line",
			expected: "//     Single Line",
			message:  "single line comment preserving whitespace",
		},
		{
			input: `Multi
Line
  With
    Spaces
	And
		Tabs
`,
			expected: `// Multi
// Line
//   With
//     Spaces
// 	And
// 		Tabs`,
			message: "multi line preserving whitespaces using tabs or spaces",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.message, func(t *testing.T) {
			result := stringToGoComment(testCase.input)
			assert.EqualValues(t, testCase.expected, result, testCase.message)
		})
	}
}

func TestStringWithTypeNameToGoComment(t *testing.T) {
	testCases := []struct {
		input     string
		inputName string
		expected  string
		message   string
	}{
		{
			input:     "",
			inputName: "",
			expected:  "",
			message:   "blank string should be ignored due to human unreadable",
		},
		{
			input:    " ",
			expected: "",
			message:  "whitespace should be ignored due to human unreadable",
		},
		{
			input:     "Single Line",
			inputName: "SingleLine",
			expected:  "// SingleLine Single Line",
			message:   "single line comment",
		},
		{
			input:     "    Single Line",
			inputName: "SingleLine",
			expected:  "// SingleLine     Single Line",
			message:   "single line comment preserving whitespace",
		},
		{
			input: `Multi
Line
  With
    Spaces
	And
		Tabs
`,
			inputName: "MultiLine",
			expected: `// MultiLine Multi
// Line
//   With
//     Spaces
// 	And
// 		Tabs`,
			message: "multi line preserving whitespaces using tabs or spaces",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.message, func(t *testing.T) {
			result := stringWithTypeNameToGoComment(testCase.input, testCase.inputName)
			assert.EqualValues(t, testCase.expected, result, testCase.message)
		})
	}
}

func TestEscapePathElements(t *testing.T) {
	p := "/foo/bar/baz"
	assert.Equal(t, p, escapePathElements(p))

	p = "foo/bar/baz"
	assert.Equal(t, p, escapePathElements(p))

	p = "/foo/bar:baz"
	assert.Equal(t, "/foo/bar%3Abaz", escapePathElements(p))
}

func TestSchemaNameToTypeName(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"$":            "DollarSign",
		"$ref":         "Ref",
		"no_prefix~+-": "NoPrefix",
		"123":          "N123",
		"-1":           "Minus1",
		"+1":           "Plus1",
		"@timestamp,":  "Timestamp",
		"&now":         "AndNow",
		"~":            "Tilde",
		"_foo":         "UnderscoreFoo",
		"=3":           "Equal3",
		"#Tag":         "HashTag",
		".com":         "DotCom",
		">=":           "GreaterThanEqual",
		"<=":           "LessThanEqual",
		"<":            "LessThan",
		">":            "GreaterThan",
	} {
		assert.Equal(t, want, schemaNameToTypeName(in))
	}
}

func TestRefPathToObjName(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"#/components/schemas/Foo":                         "Foo",
		"#/components/parameters/Bar":                      "Bar",
		"#/components/responses/baz_baz":                   "baz_baz",
		"document.json#/Foo":                               "Foo",
		"http://deepmap.com/schemas/document.json#/objObj": "objObj",
	} {
		assert.Equal(t, want, refPathToObjName(in))
	}
}

func Test_replaceInitialism(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty string",
			args: args{s: ""},
			want: "",
		},
		{
			name: "no initialism",
			args: args{s: "foo"},
			want: "foo",
		},
		{
			name: "one initialism",
			args: args{s: "fooId"},
			want: "fooID",
		},
		{
			name: "two initialism",
			args: args{s: "fooIdBarApi"},
			want: "fooIDBarAPI",
		},
		{
			name: "already initialism",
			args: args{s: "fooIDBarAPI"},
			want: "fooIDBarAPI",
		},
		{
			name: "one initialism at start",
			args: args{s: "idFoo"},
			want: "idFoo",
		},
		{
			name: "one initialism at start and one in middle",
			args: args{s: "apiIdFoo"},
			want: "apiIDFoo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, replaceInitialism(tt.args.s), "replaceInitialism(%v)", tt.args.s)
		})
	}
}

func TestPathToTypeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path []string
		want string
	}{
		{
			name: "simple path",
			path: []string{"User", "Address"},
			want: "User_Address",
		},
		{
			name: "path with numeric component",
			path: []string{"Response", "400"},
			want: "Response_N400",
		},
		{
			name: "path with multiple numeric components",
			path: []string{"Error", "400", "Issues"},
			want: "Error_N400_Issues",
		},
		{
			name: "path starting with number",
			path: []string{"400", "Issues"},
			want: "N400_Issues",
		},
		{
			name: "path with special characters",
			path: []string{"Response", "$ref"},
			want: "Response_Ref",
		},
		{
			name: "path with underscore prefix",
			path: []string{"Object", "_field"},
			want: "Object_UnderscoreField",
		},
		{
			name: "path with dash prefix",
			path: []string{"Object", "-1"},
			want: "Object_Minus1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathToTypeName(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsMediaTypeJson(t *testing.T) {
	type test struct {
		name      string
		mediaType string
		want      bool
	}

	suite := []test{
		{
			name: "When no MediaType, returns false",
			want: false,
		},
		{
			name:      "When not a JSON MediaType, returns false",
			mediaType: "application/pdf",
			want:      false,
		},
		{
			name:      "When MediaType ends with json, but isn't JSON, returns false",
			mediaType: "application/notjson",
			want:      false,
		},
		{
			name:      "When MediaType is application/json, returns true",
			mediaType: "application/json",
			want:      true,
		},
		{
			name:      "When MediaType is application/json-patch+json, returns true",
			mediaType: "application/json-patch+json",
			want:      true,
		},
		{
			name:      "When MediaType is application/vnd.api+json, returns true",
			mediaType: "application/vnd.api+json",
			want:      true,
		},
		{
			// NOTE that this _technically_ isn't a standard extension to JSON https://www.iana.org/assignments/media-types/application/json but due to the fact that several APIs do use it, we should support it
			name:      "When MediaType is application/json;v=1, returns true",
			mediaType: "application/json;v=1",
			want:      true,
		},
		{
			// NOTE that this _technically_ isn't a standard extension to JSON https://www.iana.org/assignments/media-types/application/json but due to the fact that several APIs do use it, we should support it
			name:      "When MediaType is application/json;version=1, returns true",
			mediaType: "application/json;version=1",
			want:      true,
		},
	}
	for _, test := range suite {
		t.Run(test.name, func(t *testing.T) {
			got := isMediaTypeJson(test.mediaType)

			if got != test.want {
				t.Fatalf("IsJson validation failed. Want [%v] Got [%v]", test.want, got)
			}
		})
	}
}
