package codegen

import (
	"fmt"
	"strconv"

	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
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
	strs, ok := extPropValue.([]string)
	if !ok {
		return nil, fmt.Errorf("expected []string, got %T", extPropValue)
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
			seq := make([]string, len(node.Content))
			for i, n := range node.Content {
				if n.Kind == yaml.ScalarNode {
					seq[i] = n.Value
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
				v := n.Value
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
