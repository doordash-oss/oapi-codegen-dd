package codegen

import (
	"fmt"
	"strings"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// RequestBodyDefinition describes a request body.
// Name is the name of the body.
// Required is whether the body is required.
// GoSchema is the GoSchema object describing the body.
// NameTag is the tag used to generate the type name,
// i.e. JSON, in which case we will produce "JSONBody".
// ContentType is the content type of the body.
// Default is whether this is the default body type.
// Encoding is the encoding options for formdata.
type RequestBodyDefinition struct {
	Name        string
	Required    bool
	Schema      GoSchema
	NameTag     string
	ContentType string
	Default     bool
	Encoding    map[string]*RequestBodyEncoding
}

// TypeDef returns the Go type definition for a request body
func (r RequestBodyDefinition) TypeDef(opID string) TypeDefinition {
	return TypeDefinition{
		Name:   fmt.Sprintf("%s%sRequestBody", opID, r.NameTag),
		Schema: r.Schema,
	}
}

func (r RequestBodyDefinition) IsOptional() bool {
	return !r.Schema.Constraints.Required
}

// RequestBodyEncoding describes the encoding options for a request body.
// @see https://spec.openapis.org/oas/v3.1.0#fixed-fields-12
type RequestBodyEncoding struct {
	ContentType string
	Style       string
	Explode     *bool
}

// createBodyDefinition turns the OpenAPI body definitions into a list of our body definitions
// which will be used for code generation.
func createBodyDefinition(operationID string, body *v3high.RequestBody, options ParseOptions) (*RequestBodyDefinition, *TypeDefinition, error) {
	if body == nil {
		return nil, nil, nil
	}

	required := false
	if body.Required != nil {
		required = *body.Required
	}

	pair := body.Content.First()
	if pair == nil {
		return nil, nil, nil
	}

	contentType, content := pair.Key(), pair.Value()

	schemaProxy := content.Schema
	if schemaProxy == nil {
		return nil, nil, nil
	}

	var tag string
	var defaultBody bool

	switch {
	case contentType == "application/json":
		tag = "JSON"
		defaultBody = true
	case isMediaTypeJson(contentType):
		tag = mediaTypeToCamelCase(contentType)
	case strings.HasPrefix(contentType, "multipart/"):
		tag = "Multipart"
	case contentType == "application/x-www-form-urlencoded":
		tag = "Formdata"
	case contentType == "text/plain":
		tag = "Text"
	default:
		return nil, nil, nil
	}

	bodyTypeName := operationID + "Body"
	ref := schemaProxy.GoLow().GetReference()

	bodySchema, err := GenerateGoSchema(schemaProxy, ref, []string{bodyTypeName}, options)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating request body definition: %w", err)
	}

	td := TypeDefinition{
		Name:         bodyTypeName,
		Schema:       bodySchema,
		SpecLocation: SpecLocationBody,
	}

	// If the request has a body, but it's not a user defined
	// type under #/components, we'll define a type for it, so
	// that we have an easy-to-use type for marshaling.
	if !bodySchema.DefineViaAlias {
		bodySchema.RefType = bodyTypeName
	}

	bodySchema.Constraints.Required = required

	bd := &RequestBodyDefinition{
		Name:        bodyTypeName,
		Required:    required,
		Schema:      bodySchema,
		NameTag:     tag,
		ContentType: contentType,
		Default:     defaultBody,
	}

	if content.Encoding.Len() != 0 {
		bd.Encoding = make(map[string]*RequestBodyEncoding)
		for k, v := range content.Encoding.FromOldest() {
			enc := &RequestBodyEncoding{
				ContentType: v.ContentType,
				Style:       v.Style,
				Explode:     v.Explode,
			}
			bd.Encoding[k] = enc
		}
	}

	return bd, &td, nil
}
