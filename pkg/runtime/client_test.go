// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRequestOptions struct {
	pathParams map[string]any
	query      map[string]any
	body       any
	header     map[string]string
}

func (m mockRequestOptions) GetPathParams() (map[string]any, error) { return m.pathParams, nil }
func (m mockRequestOptions) GetQuery() (map[string]any, error)      { return m.query, nil }
func (m mockRequestOptions) GetBody() any                           { return m.body }
func (m mockRequestOptions) GetHeader() (map[string]string, error)  { return m.header, nil }

type MockHttpRequestDoer struct {
	response *http.Response
	err      error
}

func (m *MockHttpRequestDoer) Do(_ context.Context, _ *http.Request) (*http.Response, error) {
	return m.response, m.err
}

func ptr[T any](v T) *T {
	return &v
}

// closeTrackingBody records how many times the response body was closed.
type closeTrackingBody struct {
	io.Reader
	closes int
}

func (b *closeTrackingBody) Close() error {
	b.closes++
	return nil
}

func TestClient_GetBaseURL(t *testing.T) {
	client := &Client{baseURL: "https://foo.bar"}
	assert.Equal(t, "https://foo.bar", client.GetBaseURL())
}

func TestClient_CreateRequest(t *testing.T) {
	tests := []struct {
		name                string
		params              RequestOptionsParameters
		expectedMethod      string
		expectedURL         string
		expectedError       bool
		expectedContentType string
	}{
		{
			name: "creates GET request successfully",
			params: RequestOptionsParameters{
				Options: mockRequestOptions{
					pathParams: map[string]any{"id": "123"},
					query:      map[string]any{"filter": "active", "tags": []string{"a", "b"}},
				},
				RequestURL:  "https://api.example.com/users/{id}",
				Method:      "GET",
				ContentType: "application/json",
				QueryEncoding: map[string]QueryEncoding{
					"tags": {Style: "deepObject", Explode: ptr(true)},
				},
			},
			expectedMethod:      "GET",
			expectedURL:         "https://api.example.com/users/123?filter=active&tags%5B%5D=a&tags%5B%5D=b",
			expectedError:       false,
			expectedContentType: "application/json",
		},
		{
			name: "creates POST request with body",
			params: RequestOptionsParameters{
				Options: mockRequestOptions{
					body:  map[string]string{"name": "test"},
					query: map[string]any{"foo": "bar", "tags": []string{"a", "b"}},
				},
				RequestURL:  "https://api.example.com/users",
				Method:      "POST",
				ContentType: "application/json",
			},
			expectedMethod:      "POST",
			expectedURL:         "https://api.example.com/users?foo=bar&tags=a&tags=b",
			expectedError:       false,
			expectedContentType: "application/json",
		},
		{
			name: "creates POST request without body",
			params: RequestOptionsParameters{
				Options:     mockRequestOptions{},
				RequestURL:  "https://api.example.com/users",
				Method:      "POST",
				ContentType: "application/json",
			},
			expectedMethod:      "POST",
			expectedURL:         "https://api.example.com/users",
			expectedError:       false,
			expectedContentType: "application/json",
		},
		{
			name: "creates POST request with body and sets Content-Type header from body encoding",
			params: RequestOptionsParameters{
				Options: mockRequestOptions{
					body: map[string]string{"name": "test"},
				},
				RequestURL: "https://api.example.com/users",
				Method:     "POST",
				BodyEncoding: map[string]FieldEncoding{
					"name": {Style: "form"},
				},
			},
			expectedMethod:      "POST",
			expectedURL:         "https://api.example.com/users",
			expectedError:       false,
			expectedContentType: "application/x-www-form-urlencoded",
		},
		{
			name: "creates POST request with body and uses header Content-Type",
			params: RequestOptionsParameters{
				Options: mockRequestOptions{
					body:   map[string]string{"name": "test"},
					header: map[string]string{"Content-Type": "application/x-www-form-urlencoded+foo"},
				},
				RequestURL: "https://api.example.com/users",
				Method:     "POST",
			},
			expectedMethod:      "POST",
			expectedURL:         "https://api.example.com/users",
			expectedError:       false,
			expectedContentType: "application/x-www-form-urlencoded+foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			req, err := client.CreateRequest(context.Background(), tt.params)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedMethod, req.Method)
			assert.Equal(t, tt.expectedURL, req.URL.String())
			assert.Equal(t, tt.expectedContentType, req.Header.Get("Content-Type"))
		})
	}
}

func TestClient_CreateRequest_no_options_passed(t *testing.T) {
	params := RequestOptionsParameters{
		RequestURL:  "https://api.example.com/users",
		Method:      "POST",
		ContentType: "application/json",
	}
	client := &Client{}

	req, err := client.CreateRequest(context.Background(), params)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://api.example.com/users", req.URL.String())
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestClient_ExecuteRequest(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   *http.Response
		mockError      error
		expectedError  bool
		expectedStatus int
	}{
		{
			name: "successful request",
			mockResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name:          "failed request",
			mockError:     fmt.Errorf("network error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDoer := &MockHttpRequestDoer{
				response: tt.mockResponse,
				err:      tt.mockError,
			}

			client := &Client{
				httpClient: mockDoer,
			}

			req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
			resp, err := client.ExecuteRequest(context.Background(), req, "/test/{id}")

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestClient_CreateRequest_streaming(t *testing.T) {
	tests := []struct {
		name           string
		options        RequestOptions
		stream         string
		expectedAccept string
		expectedMarked bool
	}{
		{
			name:           "marks the request and advertises the media type",
			stream:         "text/event-stream",
			expectedAccept: "text/event-stream",
			expectedMarked: true,
		},
		{
			name:           "leaves a caller-supplied Accept alone",
			options:        mockRequestOptions{header: map[string]string{"Accept": "application/json"}},
			stream:         "text/event-stream",
			expectedAccept: "application/json",
			expectedMarked: true,
		},
		{
			name:           "a non-streaming operation is untouched",
			expectedAccept: "",
			expectedMarked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{baseURL: "https://foo.bar"}
			req, err := client.CreateRequest(context.Background(), RequestOptionsParameters{
				Options:    tt.options,
				RequestURL: "https://foo.bar/events",
				Method:     http.MethodGet,
				Stream:     tt.stream,
			})
			require.NoError(t, err)

			assert.Equal(t, tt.expectedAccept, req.Header.Get("Accept"))
			assert.Equal(t, tt.expectedMarked, IsStreamingResponse(req.Context()))
		})
	}
}

func TestClient_ExecuteRequest_streaming(t *testing.T) {
	tests := []struct {
		name              string
		stream            string
		callCtxOnly       bool
		responseStatus    int
		responseType      string
		expectedStreaming bool
	}{
		{
			name:              "a sequential success is handed back unread",
			stream:            "text/event-stream",
			responseStatus:    http.StatusOK,
			responseType:      "text/event-stream",
			expectedStreaming: true,
		},
		{
			name:              "any 2xx counts, not just the documented one",
			stream:            "text/event-stream",
			responseStatus:    http.StatusAccepted,
			responseType:      "text/event-stream; charset=utf-8",
			expectedStreaming: true,
		},
		{
			name:              "a marker on the call context alone is honoured",
			callCtxOnly:       true,
			responseStatus:    http.StatusOK,
			responseType:      "application/x-ndjson",
			expectedStreaming: true,
		},
		{
			name:              "a non-sequential response is buffered",
			stream:            "text/event-stream",
			responseStatus:    http.StatusOK,
			responseType:      "application/json",
			expectedStreaming: false,
		},
		{
			name:              "an error response is buffered",
			stream:            "text/event-stream",
			responseStatus:    http.StatusInternalServerError,
			responseType:      "text/event-stream",
			expectedStreaming: false,
		},
		{
			name:              "an unmarked request is always buffered",
			responseStatus:    http.StatusOK,
			responseType:      "text/event-stream",
			expectedStreaming: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &closeTrackingBody{Reader: strings.NewReader(`{"status":"ok"}`)}
			client := &Client{
				baseURL: "https://foo.bar",
				httpClient: &MockHttpRequestDoer{response: &http.Response{
					StatusCode: tt.responseStatus,
					Header:     http.Header{"Content-Type": []string{tt.responseType}},
					Body:       body,
				}},
			}

			ctx := context.Background()
			req, err := client.CreateRequest(ctx, RequestOptionsParameters{
				RequestURL: "https://foo.bar/events",
				Method:     http.MethodGet,
				Stream:     tt.stream,
			})
			require.NoError(t, err)
			if tt.callCtxOnly {
				ctx = WithStreamingResponse(ctx)
			}

			resp, err := client.ExecuteRequest(ctx, req, "/events")
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStreaming, resp.Streaming)
			if tt.expectedStreaming {
				// The consumer owns the body, so it must still be open.
				assert.Nil(t, resp.Content)
				assert.Equal(t, 0, body.closes)
				assert.Same(t, body, resp.Raw.Body)
				return
			}
			assert.Equal(t, `{"status":"ok"}`, string(resp.Content))
			assert.Equal(t, 1, body.closes)
		})
	}
}

func TestIsStreamingResponse(t *testing.T) {
	// A nil context is a caller bug, but an exported helper should not panic.
	var missingCtx context.Context
	assert.False(t, IsStreamingResponse(missingCtx))
	assert.False(t, IsStreamingResponse(context.Background()))
	assert.True(t, IsStreamingResponse(WithStreamingResponse(context.Background())))
	assert.False(t, wantsStreamingResponse(context.Background(), nil))
}

func TestNewAPIClient(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		opts        []APIClientOption
		expectError bool
	}{
		{
			name:        "creates client with valid base URL",
			baseURL:     "https://api.example.com",
			expectError: false,
		},
		{
			name:        "creates client with multiple options",
			baseURL:     "https://api.example.com",
			opts:        []APIClientOption{WithHTTPClient(&MockHttpRequestDoer{})},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewAPIClient(tt.baseURL, tt.opts...)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, strings.TrimSuffix(tt.baseURL, "/"), client.baseURL)
		})
	}
}

func TestWithHTTPClient(t *testing.T) {
	mockDoer := &MockHttpRequestDoer{}
	client := &Client{}

	err := WithHTTPClient(mockDoer)(client)
	assert.NoError(t, err)
	assert.Equal(t, mockDoer, client.httpClient)
}

func TestWithRequestEditorFn(t *testing.T) {
	editor := func(ctx context.Context, req *http.Request) error { return nil }
	client := &Client{}

	err := WithRequestEditorFn(editor)(client)
	assert.NoError(t, err)
	assert.Len(t, client.requestEditors, 1)
}

func TestReplacePathPlaceholders(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		pathParams     map[string]any
		expectedResult string
	}{
		{
			name:           "replaces no placeholder",
			url:            "/users",
			pathParams:     map[string]any{"id": "123"},
			expectedResult: "/users",
		},
		{
			name:           "replaces single placeholder",
			url:            "/users/{id}",
			pathParams:     map[string]any{"id": "123"},
			expectedResult: "/users/123",
		},
		{
			name:           "replaces multiple placeholders",
			url:            "/users/{id}/posts/{postId}",
			pathParams:     map[string]any{"id": "123", "postId": "456"},
			expectedResult: "/users/123/posts/456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replacePathPlaceholders(tt.url, tt.pathParams)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
