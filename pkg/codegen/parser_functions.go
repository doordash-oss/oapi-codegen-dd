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
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/iancoleman/strcase"
)

// TemplateFunctions is passed to the template engine, and we can call each
// function here by keyName from the template code.
var TemplateFunctions = template.FuncMap{
	"genTypeName":    nameNormalizer,
	"lcFirst":        lowercaseFirstCharacter,
	"ucFirst":        uppercaseFirstCharacter,
	"caps":           strings.ToUpper,
	"lower":          strings.ToLower,
	"snake":          strcase.ToSnake,
	"toGoComment":    stringToGoCommentWithPrefix,
	"ternary":        ternary,
	"join":           join,
	"fst":            fst,
	"hasPrefix":      strings.HasPrefix,
	"hasSuffix":      strings.HasSuffix,
	"str":            str,
	"dict":           dict,
	"slice":          func() []any { return []any{} },
	"escapeGoString": escapeGoString,
	"append": func(slice []any, val any) []any {
		return append(slice, val)
	},
}

// uppercaseFirstCharacter Uppercases the first character in a string.
func uppercaseFirstCharacter(v any) string {
	str, ok := v.(string)
	if !ok || str == "" {
		return ""
	}

	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// lowercaseFirstCharacter Lowercases the first character in a string. This assumes UTF-8, so we have
// to be careful with unicode, don't treat it as a byte array.
func lowercaseFirstCharacter(str string) string {
	if str == "" {
		return ""
	}
	runes := []rune(str)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// Ternary function
func ternary(cond bool, trueVal, falseVal string) string {
	if cond {
		return trueVal
	}
	return falseVal
}

func join(sep string, values []string) string {
	return strings.Join(values, sep)
}

func fst(v any) string {
	switch val := v.(type) {
	case string:
		if len(val) > 0 {
			_, size := utf8.DecodeRuneInString(val)
			return val[:size]
		}
	case []string:
		if len(val) > 0 {
			return fst(val[0])
		}
	case []any:
		if len(val) > 0 {
			return fst(val[0])
		}
	}
	return fst(fmt.Sprintf("%v", v))
}

func str(v any) string {
	return fmt.Sprintf("%v", v)
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("invalid call to dict: uneven arguments")
	}

	d := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		d[key] = values[i+1]
	}
	return d, nil
}

// escapeGoString escapes a string for use in a Go string literal.
// It uses strconv.Quote to properly escape all special characters including backslashes.
func escapeGoString(s string) string {
	// strconv.Quote adds surrounding quotes, so we need to remove them
	quoted := strconv.Quote(s)
	// Remove the first and last character (the quotes)
	if len(quoted) >= 2 {
		return quoted[1 : len(quoted)-1]
	}
	return quoted
}
