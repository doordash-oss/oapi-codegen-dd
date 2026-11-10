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
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"go.yaml.in/yaml/v4"
)

// clientTestsFileName is the GeneratedCode key of the generated contract tests.
// Its "_test" suffix makes the CLI write it as client_test.go.
const clientTestsFileName = "client_test"

// maxFixtureDepth bounds how deep fixtures recurse into nested schemas. It
// keeps circular schemas finite: past it, only required properties are kept,
// and a schema nested deeper than twice the limit is dropped.
const maxFixtureDepth = 4

// goTypeFixtureFormats maps x-go-type suffixes to the string format whose
// fixture they decode, for schemas that have no example.
var goTypeFixtureFormats = []struct{ goType, format string }{
	{"uuid.UUID", "uuid"},
	{"time.Time", "date-time"},
	{"netip.Addr", "ipv4"},
	{"net.IP", "ipv4"},
}

// ClientTestCase is everything client-tests.tmpl needs for one operation: the
// request fixtures to send and the wire format the server should observe.
type ClientTestCase struct {
	Op OperationDefinition

	// JSON fixtures unmarshalled into the generated request option types.
	// Empty when the operation has no such part.
	PathParamsJSON string
	QueryJSON      string
	HeaderJSON     string
	BodyJSON       string

	// ExpectedPath is the request path the client should send. When some path
	// parameter is not a primitive, its value has no single expected encoding
	// and PathPattern is used instead to check the literal segments.
	ExpectedPath string
	PathPattern  string

	// QueryValues are exact values the server should see for primitive form
	// parameters; QueryKeys are parameters that only need to be present.
	QueryValues []ClientTestKeyValue
	QueryKeys   []string
	Headers     []ClientTestKeyValue

	// BodyMediaType is the media type the request must carry. BodyIsJSON
	// enables the round-trip comparison of the body the server received.
	BodyMediaType string
	BodyIsJSON    bool

	SuccessStatus    int
	SuccessMediaType string
	SuccessJSON      string
	// SuccessDecode is "json", "raw" or "" (nothing to compare).
	SuccessDecode string

	// ErrorStatus is a documented error status to reply with, or 0.
	ErrorStatus    int
	ErrorMediaType string
	ErrorJSON      string
}

// ClientTestKeyValue is an expected query value or header.
type ClientTestKeyValue struct {
	Key   string
	Value string
}

type fixtureBuilder struct {
	// active tracks the schemas and references on the current path, to stop
	// at cycles. A proxy resolves to a fresh *base.Schema on every call, so a
	// recursive reference is only recognizable by its reference string.
	active    map[*base.Schema]bool
	activeRef map[string]bool
}

// IsTestFile reports whether a GeneratedCode entry is a Go test file. Test
// files are never part of the combined single-file output; the CLI writes
// them next to it instead.
func IsTestFile(name string) bool {
	return strings.HasSuffix(name, "_test")
}

// IsGeneratedTestFile reports whether an existing client_test.go was written
// by the generator, so regenerating may replace it: it carries the standard
// "Code generated ... DO NOT EDIT." line, or (with a custom copyright-header)
// defines the generated contract-test helpers.
func IsGeneratedTestFile(existing string) bool {
	if strings.Contains(existing, "func decodeClientContractFixture(") {
		return true
	}
	for _, line := range strings.Split(existing, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			break
		}
		if strings.HasPrefix(line, "// Code generated ") && strings.HasSuffix(line, " DO NOT EDIT.") {
			return true
		}
	}
	return false
}

// buildClientTestCases derives one ClientTestCase per operation. Fixture
// values are chosen at generation time, so the generated tests contain the
// exact wire format the spec implies.
func buildClientTestCases(operations []OperationDefinition, typeSchemaMap map[string]GoSchema) ([]ClientTestCase, error) {
	cases := make([]ClientTestCase, 0, len(operations))
	for _, op := range operations {
		tc, err := buildClientTestCase(op, typeSchemaMap)
		if err != nil {
			return nil, fmt.Errorf("error building client test for %s: %w", op.ID, err)
		}
		cases = append(cases, tc)
	}
	return cases, nil
}

func buildClientTestCase(op OperationDefinition, typeSchemaMap map[string]GoSchema) (ClientTestCase, error) {
	tc := ClientTestCase{Op: op}

	// The client trims a trailing slash from the request URL.
	expectedPath := op.Path
	if len(expectedPath) > 1 {
		expectedPath = strings.TrimSuffix(expectedPath, "/")
	}
	exactPath := true

	if op.PathParams != nil {
		values := map[string]any{}
		for _, param := range pathParamDefinitions(op) {
			v := paramFixture(param)
			values[param.ParamName] = v
			s, ok := primitiveString(v)
			if !ok {
				exactPath = false
				continue
			}
			expectedPath = strings.ReplaceAll(expectedPath, "{"+param.ParamName+"}", s)
		}
		js, err := json.Marshal(values)
		if err != nil {
			return tc, err
		}
		tc.PathParamsJSON = string(js)
	}
	if exactPath {
		tc.ExpectedPath = expectedPath
	} else {
		tc.PathPattern = op.Path
	}

	if op.Query != nil {
		values := map[string]any{}
		for _, param := range op.Query.Params {
			if !param.Required && !isPrimitiveOrPrimitiveArray(paramSchema(param)) {
				continue
			}
			v := paramFixture(param)
			if v == nil {
				continue
			}
			values[param.ParamName] = v
			enc := op.Query.Encoding[param.ParamName]
			formStyle := enc.Style == "" || enc.Style == "form"
			if s, ok := primitiveString(v); ok && formStyle {
				tc.QueryValues = append(tc.QueryValues, ClientTestKeyValue{Key: param.ParamName, Value: s})
				continue
			}
			// An exploded form object spreads its properties as keys, so
			// the parameter name itself never appears.
			if _, isObject := v.(map[string]any); isObject && formStyle {
				continue
			}
			tc.QueryKeys = append(tc.QueryKeys, param.ParamName)
		}
		js, err := json.Marshal(values)
		if err != nil {
			return tc, err
		}
		tc.QueryJSON = string(js)
	}

	if op.Header != nil {
		values := map[string]any{}
		for _, param := range op.Header.Params {
			v := paramFixture(param)
			s, ok := primitiveString(v)
			if !ok {
				// Header options are sent as strings; leave structured
				// headers to the zero value.
				continue
			}
			values[param.ParamName] = v
			tc.Headers = append(tc.Headers, ClientTestKeyValue{Key: param.ParamName, Value: s})
		}
		js, err := json.Marshal(values)
		if err != nil {
			return tc, err
		}
		tc.HeaderJSON = string(js)
	}

	if op.Body != nil {
		tc.BodyMediaType = mediaTypeOf(op.Body.ContentType)
		if tc.BodyMediaType == "" {
			tc.BodyMediaType = "application/json"
		}
		tc.BodyIsJSON = isMediaTypeJson(tc.BodyMediaType)
		if tc.BodyIsJSON {
			js, err := fixtureJSON(openAPISchemaOf(op.Body.Schema, typeSchemaMap))
			if err != nil {
				return tc, err
			}
			tc.BodyJSON = js
		}
	}

	tc.SuccessStatus = op.Response.SuccessStatusCode
	if tc.SuccessStatus == 0 {
		tc.SuccessStatus = http.StatusOK
	}
	if success := op.Response.Success; success != nil && tc.SuccessStatus != http.StatusNoContent {
		tc.SuccessMediaType = mediaTypeOf(success.ContentType)
		switch {
		case success.IsRaw:
			tc.SuccessDecode = "raw"
			tc.SuccessJSON = "contract-test-payload"
			if tc.SuccessMediaType == "" {
				tc.SuccessMediaType = "application/octet-stream"
			}
		case isMediaTypeJson(tc.SuccessMediaType) || tc.SuccessMediaType == "":
			tc.SuccessDecode = "json"
			if tc.SuccessMediaType == "" {
				tc.SuccessMediaType = "application/json"
			}
			js, err := fixtureJSON(openAPISchemaOf(success.Schema, typeSchemaMap))
			if err != nil {
				return tc, err
			}
			tc.SuccessJSON = js
		}
	}

	if errResp := op.Response.Error; errResp != nil && errResp.ResponseName != "" {
		status := errorFixtureStatus(op.Response, errResp)
		mediaType := mediaTypeOf(errResp.ContentType)
		if status != 0 && (mediaType == "" || isMediaTypeJson(mediaType)) {
			js, err := fixtureJSON(openAPISchemaOf(errResp.Schema, typeSchemaMap))
			if err != nil {
				return tc, err
			}
			if mediaType == "" {
				mediaType = "application/json"
			}
			tc.ErrorStatus = status
			tc.ErrorMediaType = mediaType
			tc.ErrorJSON = js
		}
	}

	return tc, nil
}

// pathParamDefinitions returns the operation's path parameters. The path
// params struct keeps them as its properties' source parameters.
func pathParamDefinitions(op OperationDefinition) []ParameterDefinition {
	var params []ParameterDefinition
	if op.PathParams == nil {
		return nil
	}
	for _, prop := range op.PathParams.Schema.Properties {
		params = append(params, ParameterDefinition{
			ParamName: prop.JsonFieldName,
			In:        "path",
			Required:  true,
			Schema:    prop.Schema,
		})
	}
	return params
}

// errorFixtureStatus picks the status to reply with when exercising an
// operation's error response: the first exact error status the client maps
// to that response, else a code inside its range, else 500 for `default`.
func errorFixtureStatus(resp ResponseDefinition, errResp *ResponseContentDefinition) int {
	for _, rcd := range resp.Errors {
		if rcd.ResponseName == errResp.ResponseName && rcd.StatusCode >= 400 {
			return rcd.StatusCode
		}
	}
	if errResp.StatusRange >= 4 {
		return errResp.StatusRange * 100
	}
	if errResp.StatusCode >= 400 {
		return errResp.StatusCode
	}
	if errResp.IsDefault {
		return http.StatusInternalServerError
	}
	return 0
}

// openAPISchemaOf returns the spec schema behind a GoSchema. A reference to
// a declared type carries only the type name, so look the type up.
func openAPISchemaOf(s GoSchema, typeSchemaMap map[string]GoSchema) *base.Schema {
	seen := map[string]bool{}
	for s.OpenAPISchema == nil && s.RefType != "" && !seen[s.RefType] {
		seen[s.RefType] = true
		next, ok := typeSchemaMap[s.RefType]
		if !ok {
			return nil
		}
		s = next
	}
	return s.OpenAPISchema
}

func paramSchema(param ParameterDefinition) *base.Schema {
	if param.Spec != nil && param.Spec.Schema != nil {
		if s := param.Spec.Schema.Schema(); s != nil {
			return s
		}
	}
	return param.Schema.OpenAPISchema
}

func paramFixture(param ParameterDefinition) any {
	return newFixtureBuilder().value(paramSchema(param), 0)
}

func fixtureJSON(schema *base.Schema) (string, error) {
	v := newFixtureBuilder().value(schema, 0)
	if v == nil {
		v = map[string]any{}
	}
	js, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(js), nil
}

func mediaTypeOf(contentType string) string {
	if contentType == "" {
		return ""
	}
	parsed, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return contentType
	}
	return parsed
}

// fixtureLiteral quotes s for generated code, as a raw string when that
// keeps it byte-for-byte (raw strings drop carriage returns).
func fixtureLiteral(s string) string {
	if strings.ContainsRune(s, '\r') || !utf8.ValidString(s) {
		return strconv.Quote(s)
	}
	return goStringLiteral(s)
}

// primitiveString renders a fixture value the way the client puts it on the
// wire (fmt's %v of the JSON-decoded value), when it is a primitive.
func primitiveString(v any) (string, bool) {
	switch v := v.(type) {
	case string:
		return v, true
	case bool:
		return fmt.Sprintf("%v", v), true
	case int64:
		return fmt.Sprintf("%d", v), true
	case float64:
		return fmt.Sprintf("%v", v), true
	}
	return "", false
}

func isPrimitiveOrPrimitiveArray(s *base.Schema) bool {
	if s != nil && schemaType(s) == "array" && s.Items != nil && s.Items.IsA() {
		return isPrimitiveSchema(proxySchema(s.Items.A))
	}
	return isPrimitiveSchema(s)
}

func isPrimitiveSchema(s *base.Schema) bool {
	if s == nil {
		return false
	}
	switch schemaType(s) {
	case "string", "integer", "number", "boolean":
		return true
	}
	return false
}

// schemaType returns the schema's first non-null type, inferring "object" or
// "array" from its keywords when no type is declared.
func schemaType(s *base.Schema) string {
	for _, t := range s.Type {
		if t != "null" {
			return t
		}
	}
	switch {
	case s.Properties != nil && s.Properties.Len() > 0, s.AdditionalProperties != nil:
		return "object"
	case s.Items != nil:
		return "array"
	}
	return ""
}

func newFixtureBuilder() *fixtureBuilder {
	return &fixtureBuilder{active: map[*base.Schema]bool{}, activeRef: map[string]bool{}}
}

// proxyValue is value for a referenced (or inline) schema.
func (b *fixtureBuilder) proxyValue(proxy *base.SchemaProxy, depth int) any {
	if proxy == nil {
		return nil
	}
	if ref := proxy.GetReference(); ref != "" {
		if b.activeRef[ref] {
			return nil
		}
		b.activeRef[ref] = true
		defer delete(b.activeRef, ref)
	}
	return b.value(proxy.Schema(), depth)
}

// value builds a JSON-compatible value satisfying the schema's type, format,
// enum and bounds. It returns nil when no value should be sent (unknown or
// circular schemas).
func (b *fixtureBuilder) value(s *base.Schema, depth int) any {
	if s == nil || depth > 2*maxFixtureDepth || b.active[s] {
		return nil
	}
	b.active[s] = true
	defer delete(b.active, s)

	if v, ok := nodeValue(s.Const); ok {
		return v
	}
	for _, n := range s.Enum {
		if v, ok := nodeValue(n); ok && v != nil {
			return v
		}
	}

	if len(s.AllOf) > 0 {
		return b.allOf(s, depth)
	}
	if variants := unionVariants(s); len(variants) > 0 {
		return b.union(s, variants, depth)
	}
	if s.If != nil && s.Then != nil {
		return b.conditional(s, depth)
	}

	switch schemaType(s) {
	case "object":
		return b.object(s, depth)
	case "array":
		return b.array(s, depth)
	case "string":
		if v, ok := exampleOf(s, "string"); ok {
			return v
		}
		return stringFixture(s)
	case "integer":
		if v, ok := exampleOf(s, "integer"); ok {
			return v
		}
		return integerFixture(s)
	case "number":
		if v, ok := exampleOf(s, "number"); ok {
			return v
		}
		return numberFixture(s)
	case "boolean":
		return true
	}
	return nil
}

func (b *fixtureBuilder) object(s *base.Schema, depth int) any {
	out := map[string]any{}
	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}
	if s.Properties != nil {
		for name, proxy := range s.Properties.FromOldest() {
			if depth >= maxFixtureDepth && !required[name] {
				continue
			}
			if v := b.proxyValue(proxy, depth+1); v != nil {
				out[name] = v
			}
		}
	}
	if len(out) == 0 && s.AdditionalProperties != nil && s.AdditionalProperties.IsA() && depth < maxFixtureDepth {
		if v := b.proxyValue(s.AdditionalProperties.A, depth+1); v != nil {
			out["key"] = v
		}
	}
	return out
}

func (b *fixtureBuilder) array(s *base.Schema, depth int) any {
	out := []any{}
	if s.MaxItems != nil && *s.MaxItems == 0 {
		return out
	}
	count := int64(1)
	if s.MinItems != nil && *s.MinItems > count {
		count = *s.MinItems
	}
	if s.Items == nil || !s.Items.IsA() {
		return out
	}
	for i := int64(0); i < count; i++ {
		v := b.proxyValue(s.Items.A, depth+1)
		if v == nil {
			break
		}
		// uniqueItems: vary strings and numbers so repeated items differ.
		switch iv := v.(type) {
		case string:
			if i > 0 {
				v = fmt.Sprintf("%s%d", iv, i)
			}
		case int64:
			v = iv + i
		}
		out = append(out, v)
	}
	return out
}

// allOf merges the object fixtures of every member; a non-object member
// (rare: allOf used to refine a primitive) wins outright.
func (b *fixtureBuilder) allOf(s *base.Schema, depth int) any {
	merged := map[string]any{}
	if own, ok := b.object(s, depth).(map[string]any); ok {
		for k, v := range own {
			merged[k] = v
		}
	}
	for _, proxy := range s.AllOf {
		v := b.proxyValue(proxy, depth)
		obj, ok := v.(map[string]any)
		if !ok {
			if v != nil && len(merged) == 0 {
				return v
			}
			continue
		}
		for k, val := range obj {
			merged[k] = val
		}
	}
	return merged
}

// unionVariants lists the schemas a oneOf/anyOf value may take. With both
// keywords, a value has to satisfy each, so only variants listed in both
// qualify (falling back to oneOf when they share none).
func unionVariants(s *base.Schema) []*base.SchemaProxy {
	switch {
	case len(s.OneOf) > 0 && len(s.AnyOf) > 0:
		inAnyOf := map[string]bool{}
		for _, p := range s.AnyOf {
			if ref := p.GetReference(); ref != "" {
				inAnyOf[ref] = true
			}
		}
		var both []*base.SchemaProxy
		for _, p := range s.OneOf {
			if inAnyOf[p.GetReference()] {
				both = append(both, p)
			}
		}
		if len(both) > 0 {
			return both
		}
		return s.OneOf
	case len(s.OneOf) > 0:
		return s.OneOf
	}
	return s.AnyOf
}

// union uses the first variant that yields a value. Object variants merge
// the parent's own properties and carry the discriminator value that selects
// them, so a discriminated union decodes into the intended variant.
func (b *fixtureBuilder) union(s *base.Schema, variants []*base.SchemaProxy, depth int) any {
	for _, proxy := range variants {
		v := b.proxyValue(proxy, depth)
		if v == nil {
			continue
		}
		obj, ok := v.(map[string]any)
		if !ok {
			return v
		}
		if own, ok := b.object(s, depth).(map[string]any); ok {
			for k, val := range own {
				if _, exists := obj[k]; !exists {
					obj[k] = val
				}
			}
		}
		if d := s.Discriminator; d != nil && d.PropertyName != "" {
			if key, ok := discriminatorValue(d, proxy); ok {
				obj[d.PropertyName] = key
			}
		}
		return obj
	}
	return nil
}

// conditional builds a value that takes the `then` branch: the schema's own
// properties, the `then` fixture, and the constant values `if` tests for.
func (b *fixtureBuilder) conditional(s *base.Schema, depth int) any {
	then := b.proxyValue(s.Then, depth)
	thenObj, ok := then.(map[string]any)
	if !ok {
		return then
	}
	out, _ := b.object(s, depth).(map[string]any)
	if out == nil {
		out = map[string]any{}
	}
	for k, v := range thenObj {
		out[k] = v
	}
	if cond := proxySchema(s.If); cond != nil && cond.Properties != nil {
		for name, proxy := range cond.Properties.FromOldest() {
			if ps := proxySchema(proxy); ps != nil {
				if v, ok := nodeValue(ps.Const); ok && v != nil {
					out[name] = v
				} else if len(ps.Enum) > 0 {
					if v, ok := nodeValue(ps.Enum[0]); ok && v != nil {
						out[name] = v
					}
				}
			}
		}
	}
	return out
}

// discriminatorValue picks the discriminator value for a variant: its key in
// an explicit mapping, else the value the variant itself pins with an enum or
// const, else the variant's schema name (the OpenAPI implicit mapping).
func discriminatorValue(d *base.Discriminator, proxy *base.SchemaProxy) (any, bool) {
	ref := proxy.GetReference()
	if d.Mapping != nil && ref != "" {
		for key, target := range d.Mapping.FromOldest() {
			if target == ref || target == refName(ref) {
				return key, true
			}
		}
	}
	if variant := proxy.Schema(); variant != nil && variant.Properties != nil {
		if prop, ok := variant.Properties.Get(d.PropertyName); ok {
			if ps := proxySchema(prop); ps != nil {
				if v, ok := nodeValue(ps.Const); ok && v != nil {
					return v, true
				}
				for _, n := range ps.Enum {
					if v, ok := nodeValue(n); ok && v != nil {
						return v, true
					}
				}
			}
		}
	}
	if ref != "" {
		return refName(ref), true
	}
	return nil, false
}

func refName(ref string) string {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func proxySchema(proxy *base.SchemaProxy) *base.Schema {
	if proxy == nil {
		return nil
	}
	return proxy.Schema()
}

func stringFixture(s *base.Schema) string {
	format := s.Format
	// A Go type override decides what decodes, whatever the format says.
	if goType := goTypeOverride(s); goType != "" {
		for _, f := range goTypeFixtureFormats {
			if strings.HasSuffix(goType, f.goType) {
				format = f.format
				break
			}
		}
	}
	var v string
	switch format {
	case "date":
		v = "2026-01-02"
	case "date-time":
		v = "2026-01-02T03:04:05Z"
	case "time":
		v = "03:04:05"
	case "uuid":
		v = "123e4567-e89b-12d3-a456-426614174000"
	case "email":
		v = "user@example.com"
	case "uri", "url":
		v = "https://example.com"
	case "hostname":
		v = "example.com"
	case "ipv4":
		v = "192.0.2.1"
	case "ipv6":
		v = "2001:db8::1"
	case "byte":
		v = "Zml4dHVyZQ=="
	case "duration":
		v = "1s"
	default:
		v = "fixture"
	}
	if s.MaxLength != nil && int64(len(v)) > *s.MaxLength {
		v = v[:*s.MaxLength]
	}
	if s.MinLength != nil && int64(len(v)) < *s.MinLength {
		v += strings.Repeat("a", int(*s.MinLength)-len(v))
	}
	return v
}

func integerFixture(s *base.Schema) int64 {
	v := int64(1)
	if s.Minimum != nil && int64(*s.Minimum) > v {
		v = int64(*s.Minimum)
	}
	if s.ExclusiveMinimum != nil && s.ExclusiveMinimum.IsB() && int64(s.ExclusiveMinimum.B) >= v {
		v = int64(s.ExclusiveMinimum.B) + 1
	}
	if s.Maximum != nil && int64(*s.Maximum) < v {
		v = int64(*s.Maximum)
	}
	if s.MultipleOf != nil && *s.MultipleOf >= 1 {
		m := int64(*s.MultipleOf)
		if v%m != 0 {
			v = (v/m + 1) * m
		}
	}
	return v
}

func numberFixture(s *base.Schema) float64 {
	v := 1.5
	if s.Minimum != nil && *s.Minimum > v {
		v = *s.Minimum
	}
	if s.ExclusiveMinimum != nil && s.ExclusiveMinimum.IsB() && s.ExclusiveMinimum.B >= v {
		v = s.ExclusiveMinimum.B + 1
	}
	if s.Maximum != nil && *s.Maximum < v {
		v = *s.Maximum
	}
	if s.MultipleOf != nil && *s.MultipleOf > 0 {
		v = *s.MultipleOf
	}
	return v
}

// goTypeOverride returns the schema's x-go-type, if any.
func goTypeOverride(s *base.Schema) string {
	if s.Extensions == nil {
		return ""
	}
	n, ok := s.Extensions.Get(extPropGoType)
	if !ok || n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// exampleOf returns the schema's example when it has the schema's type.
func exampleOf(s *base.Schema, typ string) (any, bool) {
	nodes := s.Examples
	if s.Example != nil {
		nodes = append([]*yaml.Node{s.Example}, nodes...)
	}
	for _, n := range nodes {
		v, ok := nodeValue(n)
		if !ok {
			continue
		}
		switch v := v.(type) {
		case string:
			// With an x-go-type the example was written for that type, so it
			// is the best value there is.
			if typ == "string" && (s.Format == "" || goTypeOverride(s) != "") {
				return v, true
			}
		case int64:
			if typ == "integer" || typ == "number" {
				return v, true
			}
		case float64:
			if typ == "number" {
				return v, true
			}
		}
	}
	return nil, false
}

// nodeValue decodes a YAML scalar. Structured nodes are ignored: their shape
// is not checked against the schema.
func nodeValue(n *yaml.Node) (any, bool) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return nil, false
	}
	var v any
	if err := n.Decode(&v); err != nil {
		return nil, false
	}
	switch v := v.(type) {
	case int:
		return int64(v), true
	case uint64:
		// Only values above math.MaxInt64 decode as uint64. Converting them
		// would wrap around to a negative number, so they are skipped.
		if v > math.MaxInt64 {
			return nil, false
		}
		return int64(v), true
	case string, bool, int64, float64, nil:
		return v, true
	}
	return nil, false
}
