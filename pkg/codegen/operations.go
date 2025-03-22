package codegen

import (
	"strings"
)

// OperationDefinition describes an Operation.
// ID The operation_id description from Swagger, used to generate function names.
// Summary string from OpenAPI spec, used to generate a comment.
// Description string from OpenAPI spec.
// Method The HTTP method for this operation.
// Path The path for this operation.
// PathParams Parameters in the path
// Header HTTP headers.
// Query Query
// TypeDefinitions These are all the types we need to define for this operation.
// BodyRequired Whether the body is required for this operation.
type OperationDefinition struct {
	ID          string
	Summary     string
	Description string
	Method      string
	Path        string
	PathParams  *TypeDefinition
	Header      *TypeDefinition
	Query       *TypeDefinition

	TypeDefinitions []TypeDefinition
	BodyRequired    bool

	Body     *RequestBodyDefinition
	Response ResponseDefinition
}

// RequiresParamObject indicates If we have parameters other than path parameters, they're bundled into an
// object. Returns true if we have any of those.
// This is used from the template engine.
func (o OperationDefinition) RequiresParamObject() bool {
	return o.Query != nil || o.Header != nil
}

// SummaryAsComment returns the Operations summary as a multi line comment
func (o OperationDefinition) SummaryAsComment() string {
	if o.Summary == "" {
		return ""
	}
	trimmed := strings.TrimSuffix(o.Summary, "\n")
	parts := strings.Split(trimmed, "\n")
	for i, p := range parts {
		parts[i] = "// " + p
	}
	return strings.Join(parts, "\n")
}

func (o OperationDefinition) HasSuccessResponse() bool {
	return o.Response.Success != nil && o.Response.Success.ResponseName != ""
}

func (o OperationDefinition) GetSuccessResponse() string {
	return o.Response.Success.ResponseName
}

func (o OperationDefinition) HasErrorResponse() bool {
	return o.Response.Error != nil
}

func (o OperationDefinition) GetErrorResponse() string {
	return o.Response.Error.ResponseName
}

func (o OperationDefinition) GetFunctionResponse() string {
	if !o.HasSuccessResponse() {
		return "error"
	}
	return "(" + "*" + o.Response.Success.ResponseName + ", error)"
}

func (o OperationDefinition) HasRequestOptions() bool {
	return o.PathParams != nil || o.Header != nil || o.Query != nil || o.Body != nil
}

// FilterParameterDefinitionByType returns the subset of the specified parameters which are of the
// specified type.
func FilterParameterDefinitionByType(params []ParameterDefinition, in string) []ParameterDefinition {
	var out []ParameterDefinition
	for _, p := range params {
		if p.In == in {
			out = append(out, p)
		}
	}
	return out
}

// createOperationID generates a unique operation ID based on the HTTP method and path.
// If the initial value is provided, it will be used.
// The resulting operation ID is a camel-cased string.
func createOperationID(method, path, initial string) (string, error) {
	if initial != "" {
		return typeNamePrefix(initial) + nameNormalizer(initial), nil
	}

	if method == "" {
		return "", ErrOperationNameEmpty
	}

	if path == "" {
		return "", ErrRequestPathEmpty
	}

	res := strings.ToLower(method)

	for _, part := range strings.Split(path, "/") {
		if part != "" {
			res = res + "-" + part
		}
	}

	return nameNormalizer(res), nil
}
