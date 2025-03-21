package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/doordash/oapi-codegen/v2/pkg/util"
	"github.com/getkin/kin-openapi/openapi3"
)

// RequestBodyDefinition describes a request body.
// Required is whether the body is required.
// Schema is the GoSchema object describing the body.
// NameTag is the tag used to generate the type name,
// i.e. JSON, in which case we will produce "JSONBody".
// ContentType is the content type of the body.
// Default is whether this is the default body type.
// Encoding is the encoding options for formdata.
type RequestBodyDefinition struct {
	Required    bool
	Schema      Schema
	NameTag     string
	ContentType string
	Default     bool
	Encoding    map[string]RequestBodyEncoding
}

// TypeDef returns the Go type definition for a request body
func (r RequestBodyDefinition) TypeDef(opID string) *TypeDefinition {
	return &TypeDefinition{
		TypeName: fmt.Sprintf("%s%sRequestBody", opID, r.NameTag),
		Schema:   r.Schema,
	}
}

// CustomType returns whether the body is a custom inline type, or pre-defined. This is
// poorly named, but it's here for compatibility reasons post-refactoring
// TODO: clean up the templates code, it can be simpler.
func (r RequestBodyDefinition) CustomType() bool {
	return r.Schema.RefType == ""
}

// Suffix is needed when we're generating multiple functions which relate to request bodies,
// this generates the suffix. Such as Operation DoFoo would be suffixed with
// DoFooWithXMLBody.
func (r RequestBodyDefinition) Suffix() string {
	// The default response is never suffixed.
	if r.Default {
		return ""
	}
	return "With" + r.NameTag + "Body"
}

// IsSupportedByClient returns true if we support this content type for client. Otherwise only generic method will ge generated
func (r RequestBodyDefinition) IsSupportedByClient() bool {
	return r.IsJSON() || r.NameTag == "Formdata" || r.NameTag == "Text"
}

// IsJSON returns whether this is a JSON media type, for instance:
// - application/json
// - application/vnd.api+json
// - application/*+json
func (r RequestBodyDefinition) IsJSON() bool {
	return util.IsMediaTypeJson(r.ContentType)
}

// IsSupported returns true if we support this content type for server. Otherwise io.Reader will be generated
func (r RequestBodyDefinition) IsSupported() bool {
	return r.NameTag != ""
}

// IsFixedContentType returns true if content type has fixed content type, i.e. contains no "*" symbol
func (r RequestBodyDefinition) IsFixedContentType() bool {
	return !strings.Contains(r.ContentType, "*")
}

type RequestBodyEncoding struct {
	ContentType string
	Style       string
	Explode     *bool
}

// GenerateBodyDefinitions turns the Swagger body definitions into a list of our body
// definitions which will be used for code generation.
func GenerateBodyDefinitions(operationID string, bodyOrRef *openapi3.RequestBodyRef) ([]RequestBodyDefinition, []TypeDefinition, error) {
	if bodyOrRef == nil {
		return nil, nil, nil
	}
	body := bodyOrRef.Value

	var bodyDefinitions []RequestBodyDefinition
	var typeDefinitions []TypeDefinition

	for _, contentType := range SortedMapKeys(body.Content) {
		content := body.Content[contentType]
		var tag string
		var defaultBody bool

		switch {
		case contentType == "application/json":
			tag = "JSON"
			defaultBody = true
		case util.IsMediaTypeJson(contentType):
			tag = mediaTypeToCamelCase(contentType)
		case strings.HasPrefix(contentType, "multipart/"):
			tag = "Multipart"
		case contentType == "application/x-www-form-urlencoded":
			tag = "Formdata"
		case contentType == "text/plain":
			tag = "Text"
		default:
			bd := RequestBodyDefinition{
				Required:    body.Required,
				ContentType: contentType,
			}
			bodyDefinitions = append(bodyDefinitions, bd)
			continue
		}

		bodyTypeName := operationID + tag + "Body"
		bodySchema, err := GenerateGoSchema(content.Schema, []string{bodyTypeName})
		if err != nil {
			return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
		}

		// If the body is a pre-defined type
		if content.Schema != nil && IsGoTypeReference(content.Schema.Ref) {
			// Convert the reference path to Go type
			refType, err := RefPathToGoType(content.Schema.Ref)
			if err != nil {
				return nil, nil, fmt.Errorf("error turning reference (%s) into a Go type: %w", content.Schema.Ref, err)
			}
			bodySchema.RefType = refType
		}

		// If the request has a body, but it's not a user defined
		// type under #/components, we'll define a type for it, so
		// that we have an easy to use type for marshaling.
		if bodySchema.RefType == "" {
			if contentType == "application/x-www-form-urlencoded" {
				// Apply the appropriate structure tag if the request
				// schema was defined under the operations' section.
				for i := range bodySchema.Properties {
					bodySchema.Properties[i].NeedsFormTag = true
				}

				// Regenerate the Golang struct adding the new form tag.
				bodySchema.GoType = GenStructFromSchema(bodySchema)
			}

			td := TypeDefinition{
				TypeName: bodyTypeName,
				Schema:   bodySchema,
			}
			typeDefinitions = append(typeDefinitions, td)
			// The body schema now is a reference to a type
			bodySchema.RefType = bodyTypeName
		}

		bd := RequestBodyDefinition{
			Required:    body.Required,
			Schema:      bodySchema,
			NameTag:     tag,
			ContentType: contentType,
			Default:     defaultBody,
		}

		if len(content.Encoding) != 0 {
			bd.Encoding = make(map[string]RequestBodyEncoding)
			for k, v := range content.Encoding {
				encoding := RequestBodyEncoding{ContentType: v.ContentType, Style: v.Style, Explode: v.Explode}
				bd.Encoding[k] = encoding
			}
		}

		bodyDefinitions = append(bodyDefinitions, bd)
	}
	sort.Slice(bodyDefinitions, func(i, j int) bool {
		return bodyDefinitions[i].ContentType < bodyDefinitions[j].ContentType
	})
	return bodyDefinitions, typeDefinitions, nil
}
