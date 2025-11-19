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
