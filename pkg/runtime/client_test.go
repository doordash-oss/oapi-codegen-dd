package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

type mockHTTPCallRecorder struct {
	data []HTTPCall
}

type MockHttpRequestDoer struct {
	response *http.Response
	err      error
}

func (m *MockHttpRequestDoer) Do(_ context.Context, _ *http.Request) (*http.Response, error) {
	return m.response, m.err
}

func (m *mockHTTPCallRecorder) Record(call HTTPCall) {
	m.data = append(m.data, call)
}

func TestClient_GetBaseURL(t *testing.T) {
	client := &Client{baseURL: "https://foo.bar"}
	assert.Equal(t, "https://foo.bar", client.GetBaseURL())
}

func TestClient_CreateRequest(t *testing.T) {
	tests := []struct {
		name           string
		params         RequestOptionsParameters
		expectedMethod string
		expectedURL    string
		expectedError  bool
	}{
		{
			name: "creates GET request successfully",
			params: RequestOptionsParameters{
				Options: mockRequestOptions{
					pathParams: map[string]any{"id": "123"},
					query:      map[string]any{"filter": "active"},
				},
				RequestURL:  "https://api.example.com/users/{id}",
				Method:      "GET",
				ContentType: "application/json",
			},
			expectedMethod: "GET",
			expectedURL:    "https://api.example.com/users/123?filter=active",
			expectedError:  false,
		},
		{
			name: "creates POST request with body",
			params: RequestOptionsParameters{
				Options: mockRequestOptions{
					body: map[string]string{"name": "test"},
				},
				RequestURL:  "https://api.example.com/users",
				Method:      "POST",
				ContentType: "application/json",
			},
			expectedMethod: "POST",
			expectedURL:    "https://api.example.com/users",
			expectedError:  false,
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
		})
	}
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
			recorder := &mockHTTPCallRecorder{data: []HTTPCall{}}

			client := &Client{
				httpClient:       mockDoer,
				httpCallRecorder: recorder,
			}

			req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
			resp, err := client.ExecuteRequest(context.Background(), req, "/test/{id}")

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			assert.Equal(t, 1, len(recorder.data))
			assert.Equal(t, "/test/{id}", recorder.data[0].Path)
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
			opts:        []APIClientOption{WithHTTPClient(&MockHttpRequestDoer{}), WithHTTPCallRecorder(&mockHTTPCallRecorder{})},
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

func TestWithHTTPCallRecorder(t *testing.T) {
	recorder := &mockHTTPCallRecorder{}
	client := &Client{}

	err := WithHTTPCallRecorder(recorder)(client)
	assert.NoError(t, err)
	assert.Equal(t, recorder, client.httpCallRecorder)
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
