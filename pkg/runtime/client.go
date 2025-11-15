package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type RequestOptions interface {
	GetPathParams() (map[string]any, error)
	GetQuery() (map[string]any, error)
	GetBody() any
	GetHeader() (map[string]string, error)
}

// RequestOptionsParameters holds the parameters for creating a request.
type RequestOptionsParameters struct {
	Options       RequestOptions
	RequestURL    string
	Method        string
	ContentType   string
	BodyEncoding  map[string]FieldEncoding
	QueryEncoding map[string]QueryEncoding
}

// RequestEditorFn is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error

type HttpRequestDoer interface {
	Do(context context.Context, req *http.Request) (*http.Response, error)
}

type Response struct {
	Content    []byte
	StatusCode int
	Headers    http.Header
	Raw        *http.Response
}

type APIClient interface {
	GetBaseURL() string
	CreateRequest(ctx context.Context, params RequestOptionsParameters, reqEditors ...RequestEditorFn) (*http.Request, error)
	ExecuteRequest(ctx context.Context, req *http.Request, operationPath string) (*Response, error)
}

// Client is a client for making API requests.
// BaseURL is the base URL for the API.
// httpClient is the HTTP client to use for making requests.
// requestEditors is a list of callbacks for modifying requests which are generated before sending over the network.
type Client struct {
	baseURL        string
	httpClient     HttpRequestDoer
	requestEditors []RequestEditorFn
}

// GetBaseURL returns the base URL of the API client.
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// CreateRequest creates a new HTTP request with the given parameters and applies any request editors.
// It returns the created request or an error if the request could not be created.
func (c *Client) CreateRequest(ctx context.Context, params RequestOptionsParameters, reqEditors ...RequestEditorFn) (*http.Request, error) {
	req, err := createRequest(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	if err = c.applyEditors(ctx, req, reqEditors); err != nil {
		return nil, fmt.Errorf("error applying request editors: %w", err)
	}

	return req, nil
}

// ExecuteRequest sends the HTTP request and returns the response.
// It records the HTTP call with latency if an HTTPCallRecorder is set.
func (c *Client) ExecuteRequest(ctx context.Context, req *http.Request, operationPath string) (*Response, error) {
	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	if resp == nil {
		return nil, nil
	}

	var bodyBytes []byte
	if resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
		var err error
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response body: %w", err)
		}
	}

	return &Response{
		Content:    bodyBytes,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Raw:        resp,
	}, nil
}

// applyEditors applies all the request editors to the request.
func (c *Client) applyEditors(ctx context.Context, req *http.Request, additionalEditors []RequestEditorFn) error {
	for _, r := range c.requestEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}

	for _, r := range additionalEditors {
		if err := r(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// APIClientOption allows setting custom parameters during construction.
type APIClientOption func(*Client) error

// NewAPIClient creates a new client, with reasonable defaults.
func NewAPIClient(baseURL string, opts ...APIClientOption) (*Client, error) {
	res := &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
	}

	// mutate client and add all optional params
	for _, opt := range opts {
		if err := opt(res); err != nil {
			return nil, err
		}
	}

	return res, nil
}

// WithHTTPClient allows overriding the default Doer, which is
// automatically created using http.Client.
func WithHTTPClient(doer HttpRequestDoer) APIClientOption {
	return func(c *Client) error {
		c.httpClient = doer
		return nil
	}
}

// WithRequestEditorFn allows setting up a callback function, which will be
// called right before sending the request. This can be used to mutate the request.
func WithRequestEditorFn(fn RequestEditorFn) APIClientOption {
	return func(c *Client) error {
		c.requestEditors = append(c.requestEditors, fn)
		return nil
	}
}

// createRequest creates a new POST request with the given URL, payload and headers.
func createRequest(ctx context.Context, params RequestOptionsParameters) (*http.Request, error) {
	options := params.Options

	var (
		err         error
		pathParams  map[string]any
		queryParams map[string]any
		headers     map[string]string
		payload     any
	)

	if options != nil {
		pathParams, err = options.GetPathParams()
		if err != nil {
			return nil, err
		}

		queryParams, err = options.GetQuery()
		if err != nil {
			return nil, err
		}

		headers, err = options.GetHeader()
		if err != nil {
			return nil, err
		}

		payload = options.GetBody()
	}

	reqURL := strings.TrimSuffix(params.RequestURL, "/")
	reqURL = replacePathPlaceholders(reqURL, pathParams)

	if len(queryParams) > 0 {
		queryValue, err := EncodeQueryFields(queryParams, params.QueryEncoding)
		if err != nil {
			return nil, fmt.Errorf("error encoding query params: %w", err)
		}
		reqURL = fmt.Sprintf("%s?%s", reqURL, queryValue)
	}

	var contentType string
	if params.ContentType != "" {
		contentType = params.ContentType
	}

	if headers == nil {
		headers = map[string]string{}
	}

	// if header exists, prefer that value as contentType for encoding decision
	if _, ok := headers["Content-Type"]; ok {
		contentType = headers["Content-Type"]
	}

	httpHeaders := http.Header{}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		httpHeaders.Set(k, headers[k])
	}

	var (
		bodyBytes  []byte
		bodyReader io.Reader
	)

	// - If params.BodyEncoding is non-nil and non-empty, we treat the payload as form-encodable
	// - Otherwise fall back to the contentType hint (application/x-www-form-urlencoded)
	isForm := false
	if len(params.BodyEncoding) > 0 {
		// we have per-field encoding hints -> treat as form payload
		isForm = true
	} else if strings.HasPrefix(strings.ToLower(contentType), "application/x-www-form-urlencoded") {
		isForm = true
	}

	if payload != nil {
		if isForm {
			// Encode using EncodeFormFields which can use params.BodyEncoding map for rules
			encodedPayload, err := EncodeFormFields(payload, params.BodyEncoding)
			if err != nil {
				return nil, fmt.Errorf("error encoding form values: %w", err)
			}
			bodyBytes = []byte(encodedPayload)
			bodyReader = bytes.NewReader(bodyBytes)
			if contentType == "" {
				contentType = "application/x-www-form-urlencoded"
			}
		} else {
			// Default to JSON encoding
			bodyBytes, err = json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewBuffer(bodyBytes)
			if contentType == "" {
				contentType = "application/json"
			}
		}
	}

	if bodyBytes != nil {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, params.Method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}

	httpHeaders.Set("Content-Type", contentType)
	req.Header = httpHeaders

	if bodyBytes != nil {
		req.ContentLength = int64(len(bodyBytes))
		req.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))

		// NewRequest already wrapped the reader into a ReadCloser, but set GetBody
		// so transports can recreate a fresh reader when needed.
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	return req, nil
}

func replacePathPlaceholders(reqURL string, pathParams map[string]any) string {
	for k, v := range pathParams {
		reqURL = strings.ReplaceAll(reqURL, fmt.Sprintf("{%s}", k), fmt.Sprintf("%v", v))
	}
	return reqURL
}

var _ APIClient = (*Client)(nil)
