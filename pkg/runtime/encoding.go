package runtime

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
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
	if err := json.Unmarshal(b, &root); err != nil {
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

// ConvertFormFields converts a raw form-encoded response body to JSON format.
// It parses the form-encoded data, converts it to a map, and then marshals it to JSON.
func ConvertFormFields(resp []byte) ([]byte, error) {
	values, err := url.ParseQuery(string(resp))
	if err != nil {
		return nil, fmt.Errorf("error parsing form-encoded body: %w", err)
	}

	data := make(map[string]any, len(values))
	for key := range values {
		data[key] = values.Get(key)
	}

	// Convert back to JSON
	return json.Marshal(data)
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
