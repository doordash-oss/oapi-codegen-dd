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
	"bytes"
	"fmt"
	"go/token"
	"mime"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

var (
	pathParamRE    *regexp.Regexp
	predeclaredSet map[string]struct{}
	separatorSet   map[rune]struct{}
	nameNormalizer = toCamelCaseWithInitialism
	initialismMap  = makeInitialismMap(initialismList)
)

var camelCaseMatchParts = regexp.MustCompile(`[\p{Lu}\d]+([\p{Ll}\d]+|$)`)

var initialismList = []string{
	"ACH",
	"ACL", "API", "ASCII", "CPU", "CSS", "DNS", "EOF", "GUID", "HTML", "HTTP", "HTTPS", "ID", "IP", "JSON",
	"QPS", "RAM", "RPC", "SLA", "SMTP", "SQL", "SSH", "TCP", "TLS", "TTL", "UDP", "UI", "GID", "UID", "UUID",
	"URI", "URL", "UTF8", "VM", "XML", "XMPP", "XSRF", "XSS", "SIP", "RTP", "AMQP", "DB", "TS", "PSP",
}

// targetWordRegex is a regex that matches all initialisms.
var targetWordRegex *regexp.Regexp

func init() {
	pathParamRE = regexp.MustCompile(`{[.;?]?([^{}*]+)\*?}`)

	predeclaredIdentifiers := []string{
		// Types
		"bool",
		"byte",
		"complex64",
		"complex128",
		"error",
		"float32",
		"float64",
		"int",
		"int8",
		"int16",
		"int32",
		"int64",
		"rune",
		"string",
		"uint",
		"uint8",
		"uint16",
		"uint32",
		"uint64",
		"uintptr",
		// Constants
		"true",
		"false",
		"iota",
		// Zero value
		"nil",
		// Functions
		"append",
		"cap",
		"close",
		"complex",
		"copy",
		"delete",
		"imag",
		"len",
		"make",
		"new",
		"panic",
		"print",
		"println",
		"real",
		"recover",
	}

	// for _, acr := range initialismList {
	// 	strcase.ConfigureAcronym(acr, strings.ToLower(acr))
	// }

	predeclaredSet = map[string]struct{}{}
	for _, id := range predeclaredIdentifiers {
		predeclaredSet[id] = struct{}{}
	}

	separators := "-#@!$&=.+:;_~ (){}[]"
	separatorSet = map[rune]struct{}{}
	for _, r := range separators {
		separatorSet[r] = struct{}{}
	}
}

// UppercaseFirstCharacter Uppercases the first character in a string. This assumes UTF-8, so we have
// to be careful with unicode, don't treat it as a byte array.
func UppercaseFirstCharacter(str string) string {
	if str == "" {
		return ""
	}
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// toCamelCase will convert query-arg style strings to CamelCase.
func toCamelCase(str string) string {
	res := bytes.NewBuffer(nil)
	capNext := true
	for _, v := range str {
		if unicode.IsUpper(v) {
			res.WriteRune(v)
			capNext = false
			continue
		}
		if unicode.IsDigit(v) {
			res.WriteRune(v)
			capNext = true
			continue
		}
		if unicode.IsLower(v) {
			if capNext {
				res.WriteRune(unicode.ToUpper(v))
			} else {
				res.WriteRune(v)
			}
			capNext = false
			continue
		}
		capNext = true
	}
	return res.String()
}

// toCamelCaseWithInitialism function will convert query-arg style strings to CamelCase with initialisms in uppercase.
// So, httpOperationId would be converted to HTTPOperationID
func toCamelCaseWithInitialism(s string) string {
	parts := camelCaseMatchParts.FindAllString(toCamelCase(s), -1)
	for i := range parts {
		if v, ok := initialismMap[strings.ToLower(parts[i])]; ok {
			parts[i] = v
		}
	}
	return strings.Join(parts, "")
}

func makeInitialismMap(additionalInitialisms []string) map[string]string {
	l := append(initialismList, additionalInitialisms...)

	m := make(map[string]string, len(l))
	for i := range l {
		m[strings.ToLower(l[i])] = l[i]
	}

	// Create a regex to match the initialisms
	targetWordRegex = regexp.MustCompile(`(?i)(` + strings.Join(l, "|") + `)`)

	return m
}

func replaceInitialism(s string) string {
	// These strings do not apply CamelCase
	// Do not do CamelCase when these characters match when the preceding character is lowercase
	return targetWordRegex.ReplaceAllStringFunc(s, func(s string) string {
		// If the preceding character is lowercase, do not do CamelCase
		if unicode.IsLower(rune(s[0])) {
			return s
		}
		return strings.ToUpper(s)
	})
}

// mediaTypeToCamelCase converts a media type to a PascalCase representation
func mediaTypeToCamelCase(s string) string {
	// toCamelCase doesn't - and won't - add `/` to the characters it'll allow word boundary
	s = strings.Replace(s, "/", "_", 1)
	// including a _ to make sure that these are treated as word boundaries by `toCamelCase`
	s = strings.Replace(s, "*", "Wildcard_", 1)
	s = strings.Replace(s, "+", "Plus_", 1)

	return toCamelCaseWithInitialism(s)
}

// sortedMapKeys takes a map with keys of type string and returns a slice of those
// keys sorted lexicographically.
func sortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// refPathToObjName returns the name of referenced object without changes.
//
//	#/components/schemas/Foo -> Foo
//	#/components/parameters/Bar -> Bar
//	#/components/responses/baz_baz -> baz_baz
//
// Does not check refPath correctness.
func refPathToObjName(refPath string) string {
	parts := strings.Split(refPath, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// refPathToGoType takes a $ref value and converts it to a Go typename.
// #/components/schemas/Foo -> Foo
// #/components/parameters/Bar -> Bar
// #/components/responses/Baz -> Baz
// Remote components (document.json#/Foo) are not supported
func refPathToGoType(refPath string) (string, error) {
	pathParts := strings.Split(refPath, "/")
	depth := len(pathParts)

	if depth != 4 {
		return "", fmt.Errorf("unexpected reference depth: %d for ref: %s", depth, refPath)
	}

	// lastPart now stores the final element of the type path. This is what
	// we use as the base for a type name.
	lastPart := pathParts[len(pathParts)-1]
	return schemaNameToTypeName(lastPart), nil
}

// orderedParamsFromUri returns the argument names, in order, in a given URI string, so for
// /path/{param1}/{.param2*}/{?param3}, it would return param1, param2, param3
func orderedParamsFromUri(uri string) []string {
	matches := pathParamRE.FindAllStringSubmatch(uri, -1)
	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m[1]
	}
	return result
}

// replacePathParamsWithStr replaces path parameters of the form {param} with %s
func replacePathParamsWithStr(uri string) string {
	return pathParamRE.ReplaceAllString(uri, "%s")
}

// isGoKeyword returns whether the given string is a go keyword
func isGoKeyword(str string) bool {
	return token.IsKeyword(str)
}

// isPredeclaredGoIdentifier returns whether the given string
// is a predefined go identifier.
//
// See https://golang.org/ref/spec#Predeclared_identifiers
func isPredeclaredGoIdentifier(str string) bool {
	_, exists := predeclaredSet[str]
	return exists
}

// isGoIdentity checks if the given string can be used as an identity
// in the generated code like a type name or constant name.
//
// See https://golang.org/ref/spec#Identifiers
func isGoIdentity(str string) bool {
	for i, c := range str {
		if !isValidRuneForGoID(i, c) {
			return false
		}
	}

	return isGoKeyword(str)
}

func isValidRuneForGoID(index int, char rune) bool {
	if index == 0 && unicode.IsNumber(char) {
		return false
	}

	return unicode.IsLetter(char) || char == '_' || unicode.IsNumber(char)
}

// isValidGoIdentity checks if the given string can be used as a
// name of variable, constant, or type.
func isValidGoIdentity(str string) bool {
	if isGoIdentity(str) {
		return false
	}

	return !isPredeclaredGoIdentifier(str)
}

// sanitizeGoIdentity deletes and replaces the illegal runes in the given
// string to use the string as a valid identity.
func sanitizeGoIdentity(str string) string {
	sanitized := []rune(str)

	for i, c := range sanitized {
		if !isValidRuneForGoID(i, c) {
			sanitized[i] = '_'
		} else {
			sanitized[i] = c
		}
	}

	str = string(sanitized)

	if isGoKeyword(str) || isPredeclaredGoIdentifier(str) {
		str = "_" + str
	}

	if !isValidGoIdentity(str) {
		panic("here is a bug")
	}

	return str
}

func typeNamePrefix(name string) (prefix string) {
	if len(name) == 0 {
		return "Empty"
	}

	for _, r := range name {
		switch r {
		case '$':
			if len(name) == 1 {
				return "DollarSign"
			}
		case '-':
			prefix += "Minus"
		case '+':
			prefix += "Plus"
		case '&':
			prefix += "And"
		case '|':
			prefix += "Or"
		case '~':
			prefix += "Tilde"
		case '=':
			prefix += "Equal"
		case '>':
			prefix += "GreaterThan"
		case '<':
			prefix += "LessThan"
		case '#':
			prefix += "Hash"
		case '.':
			prefix += "Dot"
		case '*':
			prefix += "Asterisk"
		case '^':
			prefix += "Caret"
		case '%':
			prefix += "Percent"
		case '_':
			prefix += "Underscore"
		default:
			// Prepend "N" to schemas starting with a number
			if prefix == "" && unicode.IsDigit(r) {
				return "N"
			}

			// break the loop, done parsing prefix
			return
		}
	}

	return
}

// schemaNameToTypeName converts a GoSchema name to a valid Go type name.
// It converts to camel case, and makes sure the name is valid in Go
func schemaNameToTypeName(name string) string {
	return typeNamePrefix(name) + nameNormalizer(name)
}

// pathToTypeName converts a path, like Object/field1/nestedField into a go
// type name.
func pathToTypeName(path []string) string {
	for i, p := range path {
		path[i] = nameNormalizer(p)
	}
	return strings.Join(path, "_")
}

// stringToGoComment renders a possible multi-line string as a valid Go-Comment.
// Each line is prefixed as a comment.
func stringToGoComment(in string) string {
	return stringToGoCommentWithPrefix(in, "")
}

// stringWithTypeNameToGoComment renders a possible multi-line string as a
// valid Go-Comment, including the name of the type being referenced. Each line
// is prefixed as a comment.
func stringWithTypeNameToGoComment(in, typeName string) string {
	return stringToGoCommentWithPrefix(in, typeName)
}

func deprecationComment(reason string) string {
	content := "Deprecated:" // The colon is required at the end even without reason
	if reason != "" {
		content += fmt.Sprintf(" %s", reason)
	}

	return stringToGoCommentWithPrefix(content, "")
}

func stringToGoCommentWithPrefix(in, prefix string) string {
	if len(in) == 0 || len(strings.TrimSpace(in)) == 0 { // ignore empty comment
		return ""
	}

	// Normalize newlines from Windows/Mac to Linux
	in = strings.ReplaceAll(in, "\r\n", "\n")
	in = strings.ReplaceAll(in, "\r", "\n")

	// Add comment to each line
	var lines []string
	for i, line := range strings.Split(in, "\n") {
		s := "//"
		if i == 0 && len(prefix) > 0 {
			s += " " + prefix
		}
		lines = append(lines, fmt.Sprintf("%s %s", s, line))
	}
	in = strings.Join(lines, "\n")

	// in case we have a multiline string which ends with \n, we would generate
	// empty-line-comments, like `// `. Therefore remove this line comment.
	in = strings.TrimSuffix(in, "\n// ")
	return in
}

// escapePathElements breaks apart a path, and looks at each element. If it's
// not a path parameter, eg, {param}, it will URL-escape the element.
func escapePathElements(path string) string {
	elems := strings.Split(path, "/")
	for i, e := range elems {
		if strings.HasPrefix(e, "{") && strings.HasSuffix(e, "}") {
			// This is a path parameter, we don't want to mess with its value
			continue
		}
		elems[i] = url.QueryEscape(e)
	}
	return strings.Join(elems, "/")
}

// renameComponent takes as input the name of a schema as provided in the spec,
// and the definition of the schema. If the schema overrides the name via
// x-go-name, the new name is returned, otherwise, the original name is
// returned.
func renameComponent(schemaName string, schemaRef *base.SchemaProxy) (string, error) {
	// References will not change type names.
	if schemaRef.IsReference() {
		return schemaNameToTypeName(schemaName), nil
	}
	schema := schemaRef.Schema()
	exts := extractExtensions(schema.Extensions)

	if extension, ok := exts[extGoName]; ok {
		typeName, err := parseString(extension)
		if err != nil {
			return "", fmt.Errorf("invalid value for %q: %w", extPropGoType, err)
		}
		return typeName, nil
	}
	return schemaName, nil
}

// renameParameter generates the name for a parameter, taking x-go-name into account
func renameParameter(parameterName string, parameterRef *v3.Parameter) (string, error) {
	if parameterRef.Schema != nil && parameterRef.Schema.IsReference() {
		return schemaNameToTypeName(parameterName), nil
	}
	parameter := parameterRef

	exts := extractExtensions(parameter.Extensions)
	if extension, ok := exts[extGoName]; ok {
		typeName, err := parseString(extension)
		if err != nil {
			return "", fmt.Errorf("invalid value for %q: %w", extPropGoType, err)
		}
		return typeName, nil
	}
	return schemaNameToTypeName(parameterName), nil
}

func isMediaTypeJson(mediaType string) bool {
	parsed, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return false
	}
	return parsed == "application/json" || strings.HasSuffix(parsed, "+json")
}
