package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-querystring/query"
)

type RequestOptions interface {
	GetPathParams() (map[string]any, error)
	GetQuery() (map[string]any, error)
	GetPayload() any
	GetHeader() (map[string]string, error)
}

// CreateRequest creates a new POST request with the given URL, payload and headers.
func CreateRequest(ctx context.Context, url, method string, options RequestOptions) (*http.Request, error) {
	pathParams, err := options.GetPathParams()
	if err != nil {
		return nil, err
	}
	url = strings.TrimSuffix(url, "/")
	url = replacePathPlaceholders(url, pathParams)

	queryParams, err := options.GetQuery()
	if err != nil {
		return nil, err
	}
	if len(queryParams) > 0 {
		var pairs []string
		for k, v := range queryParams {
			pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
		}
		url = fmt.Sprintf("%s?%s", url, strings.Join(pairs, "&"))
	}

	payload := options.GetPayload()

	headers, err := options.GetHeader()
	if err != nil {
		return nil, err
	}
	if headers == nil {
		headers = map[string]string{
			"Content-Type": "application/json",
		}
	}
	httpHeaders := http.Header{}
	for k, v := range headers {
		httpHeaders.Set(k, v)
	}

	var bodyReader io.Reader
	var encodedPayload string

	// Check if request should be form-encoded
	if strings.HasPrefix(headers["Content-Type"], "application/x-www-form-urlencoded") {
		formValues, err := query.Values(payload)
		if err != nil {
			return nil, fmt.Errorf("error encoding form values: %w", err)
		}
		encodedPayload = formValues.Encode()
		bodyReader = strings.NewReader(encodedPayload)
	} else {
		// Default to JSON encoding
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		encodedPayload = string(body)
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header = httpHeaders

	return req, nil
}

func replacePathPlaceholders(url string, pathParams map[string]any) string {
	for k, v := range pathParams {
		url = strings.Replace(url, fmt.Sprintf("{%s}", k), fmt.Sprintf("%v", v), -1)
	}
	return url
}
