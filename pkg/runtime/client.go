package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// HTTPCall is a recorded http call client made.
// Method is the HTTP method used for the request.
// URL is the full URL of the request.
// Path is the masked path of the request, for example "/v1/payments/{id}".
// ResponseCode is the HTTP response code received. In case of failed request, this will be zero.
// Latency is the time it took to complete the request.
type HTTPCall struct {
	Method       string
	URL          string
	Path         string
	ResponseCode int
	Latency      time.Duration
}

type HTTPCallRecorder interface {
	Record(HTTPCall)
}

type RequestOptions interface {
	GetPathParams() (map[string]any, error)
	GetQuery() (map[string]any, error)
	GetBody() any
	GetHeader() (map[string]string, error)
}

// RequestOptionsParameters holds the parameters for creating a request.
type RequestOptionsParameters struct {
	Options      RequestOptions
	RequestURL   string
	Method       string
	ContentType  string
	BodyEncoding map[string]FieldEncoding
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
	baseURL          string
	httpClient       HttpRequestDoer
	requestEditors   []RequestEditorFn
	httpCallRecorder HTTPCallRecorder
	logger           Logger
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

	if c.logger != nil {
		body, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
		c.logger(ctx, LogEntry{
			Message: "Outgoing request",
			Prefix:  "request.",
			Data: &LogFields{
				Headers: req.Header,
				Body:    body,
				Extras: map[string]any{
					"method": req.Method,
					"url":    req.URL.String(),
				},
			},
		})
	}

	return req, nil
}

// ExecuteRequest sends the HTTP request and returns the response.
// It records the HTTP call with latency if an HTTPCallRecorder is set.
func (c *Client) ExecuteRequest(ctx context.Context, req *http.Request, operationPath string) (*Response, error) {
	start := time.Now()
	resp, err := c.httpClient.Do(ctx, req)
	if c.httpCallRecorder != nil {
		responseCode := 0
		if resp != nil {
			responseCode = resp.StatusCode
		}
		c.httpCallRecorder.Record(HTTPCall{
			Latency:      time.Since(start),
			Method:       req.Method,
			Path:         operationPath,
			ResponseCode: responseCode,
			URL:          req.URL.String(),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	return c.CreateResponse(ctx, resp)
}

// CreateResponse creates a Response object from the HTTP response.
// It reads the response body and logs the response if a logger is set.
func (c *Client) CreateResponse(ctx context.Context, resp *http.Response) (*Response, error) {
	if resp == nil {
		return nil, nil
	}

	var bodyBytes []byte
	if resp.Body != nil {
		defer resp.Body.Close()
		var err error
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response body: %w", err)
		}
	}

	if c.logger != nil {
		c.logger(ctx, LogEntry{
			Message: "Received response",
			Prefix:  "response.",
			Data: &LogFields{
				Headers: resp.Header,
				Body:    bodyBytes,
				Extras: map[string]any{
					"method": resp.Request.Method,
					"url":    resp.Request.URL.String(),
				},
			},
		})
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

func WithHTTPCallRecorder(httpCallRecorder HTTPCallRecorder) APIClientOption {
	return func(c *Client) error {
		c.httpCallRecorder = httpCallRecorder
		return nil
	}
}

func WithLogger(logger Logger) APIClientOption {
	return func(c *Client) error {
		c.logger = logger
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
		values := url.Values{}
		for k, v := range queryParams {
			values.Set(k, fmt.Sprintf("%v", v))
		}
		reqURL = fmt.Sprintf("%s?%s", reqURL, values.Encode())
	}

	contentType := "application/json"
	if params.ContentType != "" {
		contentType = params.ContentType
	}

	if len(headers) == 0 {
		headers = map[string]string{
			"Content-Type": contentType,
		}
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
		body       []byte
		bodyReader io.Reader
	)

	if payload != nil {
		// Check if the request should be form-encoded
		if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
			encodedPayload, err := EncodeFormFields(payload, params.BodyEncoding)
			if err != nil {
				return nil, fmt.Errorf("error encoding form values: %w", err)
			}
			bodyReader = strings.NewReader(encodedPayload)
		} else {
			// Default to JSON encoding
			body, err = json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			bodyReader = bytes.NewBuffer(body)
		}
	}

	req, err := http.NewRequestWithContext(ctx, params.Method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header = httpHeaders

	return req, nil
}

func replacePathPlaceholders(reqURL string, pathParams map[string]any) string {
	for k, v := range pathParams {
		reqURL = strings.Replace(reqURL, fmt.Sprintf("{%s}", k), fmt.Sprintf("%v", v), -1)
	}
	return reqURL
}

var _ APIClient = (*Client)(nil)
