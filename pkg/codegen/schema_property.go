package codegen

import (
	"fmt"
	"slices"
	"strings"
)

type Property struct {
	GoName        string
	Description   string
	JsonFieldName string
	Schema        GoSchema
	NeedsFormTag  bool
	Extensions    map[string]any
	Deprecated    bool
	Constraints   Constraints
}

func (p Property) IsEqual(other Property) bool {
	return p.JsonFieldName == other.JsonFieldName &&
		p.Schema.TypeDecl() == other.Schema.TypeDecl() &&
		p.Constraints.IsEqual(other.Constraints)
}

func (p Property) GoTypeDef() string {
	typeDef := p.Schema.TypeDecl()

	if p.Schema.OpenAPISchema != nil && slices.Contains(p.Schema.OpenAPISchema.Type, "array") {
		return typeDef
	}

	if p.Schema.OpenAPISchema != nil && slices.Contains(p.Schema.OpenAPISchema.Type, "object") {
		if p.Schema.OpenAPISchema.AdditionalProperties != nil && p.Schema.OpenAPISchema.AdditionalProperties.IsB() {
			return typeDef
		}
	}

	if strings.HasPrefix(typeDef, "map[") {
		return typeDef
	}

	if !p.Schema.SkipOptionalPointer && p.Constraints.Nullable {
		typeDef = "*" + strings.TrimPrefix(typeDef, "*")
	}
	return typeDef
}

func createPropertyGoFieldName(jsonName string, extensions map[string]any) string {
	goFieldName := jsonName
	if extension, ok := extensions[extGoName]; ok {
		if extGoFieldName, err := parseString(extension); err == nil {
			goFieldName = extGoFieldName
		}
	}

	if extension, ok := extensions[extOapiCodegenOnlyHonourGoName]; ok {
		if use, err := parseBooleanValue(extension); err == nil {
			if use {
				return goFieldName
			}
		}
	}

	// convert some special names needed for interfaces
	if goFieldName == "error" {
		goFieldName = "ErrorData"
	}

	return schemaNameToTypeName(goFieldName)
}

// genFieldsFromProperties produce corresponding field names with JSON annotations,
// given a list of schema descriptors
func genFieldsFromProperties(props []Property, options ParseOptions) []string {
	var fields []string

	for i, p := range props {
		field := ""
		goFieldName := p.GoName

		// Add a comment to a field in case we have one, otherwise skip.
		if !options.OmitDescription && p.Description != "" {
			// Separate the comment from a previous-defined, unrelated field.
			// Make sure the actual field is separated by a newline.
			if i != 0 {
				field += "\n"
			}
			field += fmt.Sprintf("%s\n", stringWithTypeNameToGoComment(p.Description, p.GoName))
		}

		if p.Deprecated {
			// This comment has to be on its own line for godoc & IDEs to pick up
			var deprecationReason string
			if extension, ok := p.Extensions[extDeprecationReason]; ok {
				if extOmitEmpty, err := parseString(extension); err == nil {
					deprecationReason = extOmitEmpty
				}
			}

			field += fmt.Sprintf("%s\n", deprecationComment(deprecationReason))
		}

		// Check x-go-type-skip-optional-pointer, which will override if the type
		// should be a pointer or not when the field is optional.
		if extension, ok := p.Extensions[extPropGoTypeSkipOptionalPointer]; ok {
			if skipOptionalPointer, err := parseBooleanValue(extension); err == nil {
				p.Schema.SkipOptionalPointer = skipOptionalPointer
			}
		}

		field += fmt.Sprintf("    %s %s", goFieldName, p.GoTypeDef())

		c := p.Constraints
		omitEmpty := c.Nullable
		if p.Schema.SkipOptionalPointer {
			omitEmpty = false
		}

		// Support x-omitempty
		if extOmitEmptyValue, ok := p.Extensions[extPropOmitEmpty]; ok {
			if extOmitEmpty, err := parseBooleanValue(extOmitEmptyValue); err == nil {
				omitEmpty = extOmitEmpty
			}
		}

		fieldTags := make(map[string]string)

		if len(p.Constraints.ValidationTags) > 0 {
			fieldTags["validate"] = strings.Join(c.ValidationTags, ",")
		}

		if !omitEmpty {
			fieldTags["json"] = p.JsonFieldName
			if p.NeedsFormTag {
				fieldTags["form"] = p.JsonFieldName
			}
		} else {
			fieldTags["json"] = p.JsonFieldName + ",omitempty"
			if p.NeedsFormTag {
				fieldTags["form"] = p.JsonFieldName + ",omitempty"
			}
		}

		// Support x-go-json-ignore
		if extension, ok := p.Extensions[extPropGoJsonIgnore]; ok {
			if goJsonIgnore, err := parseBooleanValue(extension); err == nil && goJsonIgnore {
				fieldTags["json"] = "-"
			}
		}

		// Support x-oapi-codegen-extra-tags
		if extension, ok := p.Extensions[extPropExtraTags]; ok {
			if tags, err := extExtraTags(extension); err == nil {
				keys := sortedMapKeys(tags)
				for _, k := range keys {
					fieldTags[k] = tags[k]
				}
			}
		}
		// Convert the fieldTags map into Go field annotations.
		keys := sortedMapKeys(fieldTags)
		tags := make([]string, len(keys))
		for j, k := range keys {
			tags[j] = fmt.Sprintf(`%s:"%s"`, k, fieldTags[k])
		}
		field += "`" + strings.Join(tags, " ") + "`"
		fields = append(fields, field)
	}

	return fields
}
