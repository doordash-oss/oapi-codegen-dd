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

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

const (
	// extPropGoType overrides the generated type definition.
	extPropGoType = "x-go-type"

	// extPropGoTypeSkipOptionalPointer specifies that optional fields should
	// be the type itself instead of a pointer to the type.
	extPropGoTypeSkipOptionalPointer = "x-go-type-skip-optional-pointer"

	// extPropGoImport specifies the module to import which provides above type
	extPropGoImport = "x-go-type-import"

	// extGoName is used to override a field name
	extGoName = "x-go-name"

	// extGoTypeName overrides a generated typename for something.
	extGoTypeName = "x-go-type-name"

	extPropGoJsonIgnore = "x-go-json-ignore"
	extPropOmitEmpty    = "x-omitempty"
	extPropExtraTags    = "x-oapi-codegen-extra-tags"

	// Override generated variable names for enum constants.
	extEnumNames         = "x-enum-names"
	extDeprecationReason = "x-deprecated-reason"

	// extOapiCodegenOnlyHonourGoName explicitly enforces the generation of a
	// field as the `x-go-name` extension describes it.
	extOapiCodegenOnlyHonourGoName = "x-oapi-codegen-only-honour-go-name"

	// extSensitiveData marks a field as containing sensitive data that should be masked
	extSensitiveData = "x-sensitive-data"
)

func extExtraTags(extPropValue any) (map[string]string, error) {
	tagsI, ok := extPropValue.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("failed to convert type: %T", extPropValue)
	}

	tags := make(map[string]string, len(tagsI))
	for k, v := range tagsI {
		vs, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("failed to convert type: %T", v)
		}
		tags[k] = vs
	}
	return tags, nil
}

func extParseEnumVarNames(extPropValue any) ([]string, error) {
	rawSlice, ok := extPropValue.([]any)
	if !ok {
		return nil, fmt.Errorf("expected []any, got %T", extPropValue)
	}

	strs := make([]string, 0, len(rawSlice))
	for _, v := range rawSlice {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expected string in slice, got %T", v)
		}
		strs = append(strs, s)
	}

	return strs, nil
}

func extractExtensions(schemaExtensions *orderedmap.Map[string, *yaml.Node]) map[string]any {
	if schemaExtensions == nil || schemaExtensions.Len() == 0 {
		return nil
	}

	res := make(map[string]any)

	for extType, node := range schemaExtensions.FromOldest() {
		res[extType] = make(map[string]any)

		if node.Kind == yaml.ScalarNode {
			res[extType] = node.Value
			continue
		}

		if node.Kind == yaml.SequenceNode {
			seq := make([]any, len(node.Content))
			for i, n := range node.Content {
				switch n.Kind {
				case yaml.ScalarNode:
					seq[i] = n.Value
				case yaml.MappingNode:
					mKey, mValue := n.Content[0].Value, n.Content[1].Value
					seq[i] = keyValue[string, string]{mKey, mValue}
				}
			}
			res[extType] = seq
			continue
		}

		if node.Kind != yaml.MappingNode {
			continue
		}

		var k string
		inner := make(map[string]any)
		for i, n := range node.Content {
			if i%2 == 0 {
				k = n.Value
			} else {
				if k == "" {
					continue
				}
				// Decode the value based on its type
				var v any
				if err := n.Decode(&v); err != nil {
					v = n.Value // fallback to string value
				}
				inner[k] = v
				k = ""
			}
		}
		res[extType] = inner
	}
	return res
}

func parseString(extPropValue any) (string, error) {
	str, ok := extPropValue.(string)
	if !ok {
		return "", fmt.Errorf("failed to convert type: %T", extPropValue)
	}
	return str, nil
}

func parseBooleanValue(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("failed to convert type: %T", value)
		}
		return b, nil
	}
	return false, fmt.Errorf("failed to convert type: %T", value)
}

// extParseSensitiveData parses the x-sensitive-data extension value into runtime.SensitiveDataConfig
func extParseSensitiveData(extPropValue any) (*runtime.SensitiveDataConfig, error) {
	config := runtime.NewDefaultSensitiveDataConfig()
	if err := config.Unmarshal(extPropValue); err != nil {
		return nil, err
	}
	return config, nil
}
