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
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type FieldEncoding struct {
	Style       string
	Explode     *bool
	ContentType string
}

// EncodeFormFields encodes the given data into a URL-encoded form string.
func EncodeFormFields(data any, encoding map[string]FieldEncoding) (string, error) {
	values := url.Values{}

	// Marshal input to map[string]any
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	var root map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		return "", err
	}

	for key, val := range root {
		fieldEnc, found := encoding[key]

		// Apply OpenAPI defaults if not explicitly defined
		style := "form"
		explode := true
		if found {
			if fieldEnc.Style != "" {
				style = fieldEnc.Style
			}
			if fieldEnc.Style == "form" && fieldEnc.Explode != nil {
				explode = *fieldEnc.Explode
			}
		}

		switch style {
		case "deepObject":
			encodeDeepObject(key, val, values)

		default:
			encodeForm(key, val, values, explode)
		}
	}

	return values.Encode(), nil
}

// ConvertFormFields converts a raw form-encoded request body to JSON format.
// It handles deepObject encoding (e.g., "obj[key][nested]=value") and converts
// string values to appropriate types (bool, number, etc.).
func ConvertFormFields(resp []byte) ([]byte, error) {
	values, err := url.ParseQuery(string(resp))
	if err != nil {
		return nil, fmt.Errorf("error parsing form-encoded body: %w", err)
	}

	data := decodeFormData(values)
	return json.Marshal(data)
}

// decodeFormData decodes URL-encoded form data into a nested map structure.
// It handles deepObject encoding (e.g., "obj[key][nested]=value") and converts
// string values to appropriate types (bool, number, etc.).
func decodeFormData(values url.Values) map[string]any {
	result := make(map[string]any)

	for key, vals := range values {
		// Check if this is a deepObject encoded key (contains brackets)
		if strings.Contains(key, "[") {
			setNestedValue(result, key, vals)
		} else {
			// Simple key-value pair
			if len(vals) == 1 {
				result[key] = convertFormStringValue(vals[0])
			} else {
				// Multiple values for the same key (array)
				converted := make([]any, len(vals))
				for i, v := range vals {
					converted[i] = convertFormStringValue(v)
				}
				result[key] = converted
			}
		}
	}

	return result
}

// setNestedValue sets a value in a nested map structure based on a deepObject encoded key.
// Example: "obj[key][0][nested]=value" sets result["obj"]["key"][0]["nested"] = value
func setNestedValue(result map[string]any, key string, values []string) {
	// Parse the key to extract the path
	// Example: "flow_data[subscription_update_confirm][items][0][id]"
	// becomes ["flow_data", "subscription_update_confirm", "items", "0", "id"]
	parts := parseDeepObjectKey(key)
	if len(parts) == 0 {
		return
	}

	// Special case: if only 2 parts and second is numeric, it's a simple array
	// Example: "expand[0]" should become expand: ["value"]
	if len(parts) == 2 && isFormNumeric(parts[1]) {
		arr, ok := result[parts[0]].([]any)
		if !ok {
			arr = make([]any, 0)
		}

		idx := mustFormAtoi(parts[1])
		arr = ensureArraySize(arr, idx, false)
		arr[idx] = convertFormValues(values)
		result[parts[0]] = arr
		return
	}

	// Navigate/create the nested structure
	current := result
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		nextPart := parts[i+1]

		// Check if next part is a number (array index)
		isNextArray := isFormNumeric(nextPart)

		if isNextArray {
			// Current should be an array
			arr, ok := current[part].([]any)
			if !ok {
				arr = make([]any, 0)
				current[part] = arr
			}

			// Get the index
			idx := mustFormAtoi(nextPart)

			// Check if this is the last level (nextPart is the last part)
			if i+2 == len(parts) {
				// This is the final value - just set it in the array
				arr = ensureArraySize(arr, idx, false)
				arr[idx] = convertFormValues(values)
				current[part] = arr
				return
			}

			// Not the final value - need to navigate deeper
			arr = ensureArraySize(arr, idx, true)
			current[part] = arr

			// Move to the array element
			elem, ok := arr[idx].(map[string]any)
			if !ok {
				elem = make(map[string]any)
				arr[idx] = elem
			}
			current = elem
			i++ // Skip the index part
		} else {
			// Current should be an object
			next, ok := current[part].(map[string]any)
			if !ok {
				next = make(map[string]any)
				current[part] = next
			}
			current = next
		}
	}

	// Set the final value
	lastPart := parts[len(parts)-1]
	current[lastPart] = convertFormValues(values)
}

// ensureArraySize grows the array to accommodate the given index.
// If useMap is true, fills new slots with fresh map[string]any instances.
// Otherwise, fills with nil.
func ensureArraySize(arr []any, idx int, useMap bool) []any {
	for len(arr) <= idx {
		if useMap {
			arr = append(arr, make(map[string]any))
		} else {
			arr = append(arr, nil)
		}
	}
	return arr
}

// convertFormValues converts form values to appropriate types.
// Returns a single value if there's only one, or a slice if there are multiple.
func convertFormValues(values []string) any {
	if len(values) == 1 {
		return convertFormStringValue(values[0])
	}
	converted := make([]any, len(values))
	for i, v := range values {
		converted[i] = convertFormStringValue(v)
	}
	return converted
}

// parseDeepObjectKey parses a deepObject encoded key into parts.
// Example: "flow_data[subscription_update_confirm][items][0][id]"
// Returns: ["flow_data", "subscription_update_confirm", "items", "0", "id"]
func parseDeepObjectKey(key string) []string {
	var parts []string

	// Split by '[' and clean up ']'
	segments := strings.Split(key, "[")
	for _, seg := range segments {
		cleaned := strings.TrimSuffix(seg, "]")
		if cleaned != "" {
			parts = append(parts, cleaned)
		}
	}

	return parts
}

func encodeForm(prefix string, value any, values url.Values, explode bool) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		if explode {
			for _, k := range keys {
				fullKey := fmt.Sprintf("%s.%s", prefix, k)
				encodeForm(fullKey, v[k], values, explode)
			}
		} else {
			var parts []string
			for _, k := range keys {
				parts = append(parts, fmt.Sprintf("%s,%v", k, v[k]))
			}
			values.Set(prefix, strings.Join(parts, ","))
		}

	case []any:
		for _, item := range v {
			values.Add(prefix, fmt.Sprintf("%v", item))
		}

	default:
		values.Set(prefix, fmt.Sprintf("%v", v))
	}
}

func encodeDeepObject(prefix string, value any, values url.Values) {
	switch v := value.(type) {
	case map[string]any:
		for k, sub := range v {
			fullKey := fmt.Sprintf("%s[%s]", prefix, k)
			encodeDeepObject(fullKey, sub, values)
		}
	case []any:
		for i, item := range v {
			fullKey := fmt.Sprintf("%s[%d]", prefix, i)
			encodeDeepObject(fullKey, item, values)
		}
	default:
		values.Set(prefix, fmt.Sprintf("%v", v))
	}
}

// convertFormStringValue attempts to convert a string value to its appropriate type.
// Handles: bool, int, float, or keeps as string.
// Note: We're conservative with conversions to avoid misinterpreting strings like phone numbers.
func convertFormStringValue(s string) any {
	// Try boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Don't convert strings that start with + (likely phone numbers)
	if strings.HasPrefix(s, "+") {
		return s
	}

	// Don't convert strings that contain spaces or parentheses (likely formatted values)
	if strings.ContainsAny(s, " ()") {
		return s
	}

	// Try integer (only if it doesn't have leading zeros, which would indicate a string like "001")
	if len(s) > 0 && (s[0] != '0' || s == "0") {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i
		}
	}

	// Try float (only if it contains a decimal point)
	if strings.Contains(s, ".") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}

	// Keep as string
	return s
}

// isFormNumeric checks if a string represents a number.
func isFormNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// mustFormAtoi converts a string to int (should only be called after isFormNumeric check).
func mustFormAtoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
