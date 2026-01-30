// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package runtime

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type QueryEncoding struct {
	Style   string
	Explode *bool
}

// queryPair represents a key-value pair for query string encoding.
// If preEncoded is true, the value is already properly encoded and should not
// be encoded again by url.Values.Encode().
type queryPair struct {
	key        string
	value      string
	preEncoded bool
}

// EncodeQueryFields builds a query string for query params per OAS 3.1 style matrix.
//
// Arrays (name=expand, vals=[a,b]):
// - form, explode=true     => expand=a&expand=b
// - form, explode=false    => expand=a,b
// - spaceDelimited         => expand=a%20b
// - pipeDelimited          => expand=a%7Cb
// - deepObject (custom)    => expand%5B%5D=a&expand%5B%5D=b
//
// Objects (name=color, vals={R:100,G:200,B:150}):
// - form, explode=true     => R=100&G=200&B=150
// - form, explode=false    => color=R,100,G,200,B,150
// - spaceDelimited         => color=R%20100%20G%20200%20B%20150
// - pipeDelimited          => color=R%7C100%7CG%7C200%7CB%7C150
// - deepObject (spec)      => color%5BR%5D=100&color%5BG%5D=200&color%5BB%5D=150
//
// Scalars (name=x, val=v): always x=v (style choice irrelevant).
func EncodeQueryFields(data any, encoding map[string]QueryEncoding) (string, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return "", ErrMustBeMap
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []queryPair

	for _, name := range keys {
		val := m[name]
		enc := QueryEncoding{}
		if encoding != nil {
			enc = encoding[name]
		}

		style := strings.ToLower(enc.Style)
		if style == "" {
			style = "form"
		}
		explode := defaultExplode(style, enc.Explode)

		if obj, isObj, err := toStringMap(val); err != nil {
			return "", fmt.Errorf("param %q: %w", name, err)
		} else if isObj {
			propKeys := make([]string, 0, len(obj))
			for k := range obj {
				propKeys = append(propKeys, k)
			}
			sort.Strings(propKeys)

			switch style {
			case "form":
				if explode {
					for _, k := range propKeys {
						pairs = append(pairs, queryPair{key: k, value: obj[k]})
					}
				} else {
					// For explode=false, we need to encode each value individually,
					// then join with unescaped comma delimiter.
					// This ensures commas within values are escaped, but delimiter commas are not.
					pairs = append(pairs, queryPair{
						key:        name,
						value:      joinWithDelimiter(flattenMap(propKeys, obj), ","),
						preEncoded: true,
					})
				}
			case "spacedelimited":
				// Space delimiter should be encoded as %20
				pairs = append(pairs, queryPair{
					key:        name,
					value:      joinWithDelimiter(flattenMap(propKeys, obj), "%20"),
					preEncoded: true,
				})
			case "pipedelimited":
				// Pipe delimiter should be encoded as %7C
				pairs = append(pairs, queryPair{
					key:        name,
					value:      joinWithDelimiter(flattenMap(propKeys, obj), "%7C"),
					preEncoded: true,
				})
			case "deepobject":
				for _, k := range propKeys {
					pairs = append(pairs, queryPair{key: name + "[" + k + "]", value: obj[k]})
				}
			default:
				return "", fmt.Errorf("param %q: unsupported style %q for object", name, style)
			}
			continue
		}

		ss, isArray, err := toStringSlice(val)
		if err != nil {
			return "", fmt.Errorf("param %q: %w", name, err)
		}

		switch style {
		case "form":
			if isArray {
				if explode {
					for _, v := range ss {
						pairs = append(pairs, queryPair{key: name, value: v})
					}
				} else {
					// For explode=false, we need to encode each value individually,
					// then join with unescaped comma delimiter.
					// This ensures commas within values are escaped, but delimiter commas are not.
					pairs = append(pairs, queryPair{
						key:        name,
						value:      joinWithDelimiter(ss, ","),
						preEncoded: true,
					})
				}
			} else {
				pairs = append(pairs, queryPair{key: name, value: ss[0]})
			}
		case "spacedelimited":
			if isArray {
				// Space delimiter should be encoded as %20
				pairs = append(pairs, queryPair{
					key:        name,
					value:      joinWithDelimiter(ss, "%20"),
					preEncoded: true,
				})
			} else {
				pairs = append(pairs, queryPair{key: name, value: ss[0]})
			}
		case "pipedelimited":
			if isArray {
				// Pipe delimiter should be encoded as %7C
				pairs = append(pairs, queryPair{
					key:        name,
					value:      joinWithDelimiter(ss, "%7C"),
					preEncoded: true,
				})
			} else {
				pairs = append(pairs, queryPair{key: name, value: ss[0]})
			}
		case "deepobject":
			// Not defined for arrays in OAS. Custom bracketed repeat as discussed.
			br := name + "[]"
			if isArray {
				for _, v := range ss {
					pairs = append(pairs, queryPair{key: br, value: v})
				}
			} else {
				pairs = append(pairs, queryPair{key: br, value: ss[0]})
			}
		default:
			return "", fmt.Errorf("param %q: unsupported style %q", name, style)
		}
	}

	return buildQueryString(pairs), nil
}

// joinWithDelimiter encodes each value individually and joins them with the given delimiter.
// The delimiter is NOT encoded, allowing for proper OpenAPI style serialization.
func joinWithDelimiter(values []string, delimiter string) string {
	encoded := make([]string, len(values))
	for i, v := range values {
		encoded[i] = url.QueryEscape(v)
	}
	return strings.Join(encoded, delimiter)
}

// flattenMap flattens a map into a slice of alternating keys and values.
func flattenMap(keys []string, m map[string]string) []string {
	result := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		result = append(result, k, m[k])
	}
	return result
}

// buildQueryString builds a query string from pairs, handling pre-encoded values correctly.
func buildQueryString(pairs []queryPair) string {
	if len(pairs) == 0 {
		return ""
	}

	var parts []string
	for _, p := range pairs {
		encodedKey := url.QueryEscape(p.key)
		var encodedValue string
		if p.preEncoded {
			encodedValue = p.value
		} else {
			encodedValue = url.QueryEscape(p.value)
		}
		parts = append(parts, encodedKey+"="+encodedValue)
	}

	// Sort for consistent output (matching url.Values.Encode() behavior)
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

func defaultExplode(style string, ptr *bool) bool {
	if ptr != nil {
		return *ptr
	}
	return style == "form"
}

func toStringSlice(v any) ([]string, bool, error) {
	switch t := v.(type) {
	case nil:
		return []string{""}, false, nil
	case string:
		return []string{t}, false, nil
	case fmt.Stringer:
		return []string{t.String()}, false, nil
	case bool:
		if t {
			return []string{"true"}, false, nil
		}
		return []string{"false"}, false, nil
	case int, int8, int16, int32, int64:
		return []string{fmt.Sprintf("%d", t)}, false, nil
	case uint, uint8, uint16, uint32, uint64:
		return []string{fmt.Sprintf("%d", t)}, false, nil
	case float32, float64:
		return []string{fmt.Sprintf("%v", t)}, false, nil
	case []string:
		cp := make([]string, len(t))
		copy(cp, t)
		return cp, true, nil
	case []any:
		out := make([]string, len(t))
		for i, e := range t {
			ss, _, err := toStringSlice(e)
			if err != nil {
				return nil, false, err
			}
			out[i] = ss[0]
		}
		return out, true, nil
	default:
		return []string{fmt.Sprintf("%v", t)}, false, nil
	}
}

func toStringMap(v any) (map[string]string, bool, error) {
	switch t := v.(type) {
	case map[string]string:
		cp := make(map[string]string, len(t))
		for k, v := range t {
			cp[k] = v
		}
		return cp, true, nil
	case map[string]any:
		out := make(map[string]string, len(t))
		for k, vv := range t {
			ss, _, err := toStringSlice(vv)
			if err != nil {
				return nil, false, err
			}
			out[k] = ss[0]
		}
		return out, true, nil
	default:
		return nil, false, nil
	}
}
