package example6

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httpClientAdapter adapts http.Client to runtime.HTTPRequestDoer
type httpClientAdapter struct {
	client *http.Client
}

func (a *httpClientAdapter) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return a.client.Do(req)
}

func TestUpdateUser_NilResponseBody(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		responseStatus int
		expectError    bool
		errorContains  string
		expectEmpty    bool // expect empty struct response
	}{
		{
			name:           "valid JSON response",
			responseBody:   `{"name": "John Doe", "type": "individual"}`,
			responseStatus: http.StatusAccepted,
			expectError:    false,
		},
		{
			name:           "empty response body - should return empty struct",
			responseBody:   "",
			responseStatus: http.StatusAccepted,
			expectError:    false,
			expectEmpty:    true,
		},
		{
			name:           "whitespace only response body - should fail",
			responseBody:   "   ",
			responseStatus: http.StatusAccepted,
			expectError:    true,
			errorContains:  "error decoding response",
		},
		{
			name:           "null JSON response",
			responseBody:   "null",
			responseStatus: http.StatusAccepted,
			expectError:    false,
			expectEmpty:    true,
		},
		{
			name:           "invalid JSON response",
			responseBody:   `{"name": "incomplete`,
			responseStatus: http.StatusAccepted,
			expectError:    true,
			errorContains:  "error decoding response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.responseStatus)
				if tt.responseBody != "" {
					_, _ = io.WriteString(w, tt.responseBody)
				}
			}))
			defer server.Close()

			httpClient := &httpClientAdapter{client: server.Client()}
			apiClient, err := runtime.NewAPIClient(server.URL, runtime.WithHTTPClient(httpClient))
			require.NoError(t, err)

			client := NewClient(apiClient)

			options := &UpdateUserRequestOptions{
				Body: &User{Name: "Test User"},
			}

			resp, err := client.UpdateUser(context.Background(), options)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)

			if tt.expectEmpty {
				// Empty body should return zero-value struct
				assert.Equal(t, "", resp.Name)
				assert.Nil(t, resp.Type)
			} else if tt.responseBody != "" && tt.responseBody != "null" {
				assert.Equal(t, "John Doe", resp.Name)
				assert.NotNil(t, resp.Type)
				assert.Equal(t, UserTypeIndividual, *resp.Type)
			}
		})
	}
}

func TestUpdateUser_ErrorResponse(t *testing.T) {
	tests := []struct {
		name          string
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:          "error response with body",
			responseBody:  `{"message": "invalid user data", "code": "INVALID_USER"}`,
			expectError:   true,
			errorContains: "invalid user data",
		},
		{
			name:          "error response with empty body",
			responseBody:  "",
			expectError:   true,
			errorContains: "unknown error", // default from Error() method when message is nil
		},
		{
			name:          "error response with null",
			responseBody:  "null",
			expectError:   true,
			errorContains: "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				if tt.responseBody != "" {
					_, _ = io.WriteString(w, tt.responseBody)
				}
			}))
			defer server.Close()

			httpClient := &httpClientAdapter{client: server.Client()}
			apiClient, err := runtime.NewAPIClient(server.URL, runtime.WithHTTPClient(httpClient))
			require.NoError(t, err)

			client := NewClient(apiClient)

			options := &UpdateUserRequestOptions{
				Body: &User{Name: "Test User"},
			}

			resp, err := client.UpdateUser(context.Background(), options)

			require.Error(t, err)
			assert.Nil(t, resp)
			if tt.errorContains != "" {
				assert.Contains(t, err.Error(), tt.errorContains)
			}
		})
	}
}

func TestUpdateUser_RequestBodyEncoding(t *testing.T) {
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		capturedBody = string(bodyBytes)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"name": "John Doe", "type": "individual"}`)
	}))
	defer server.Close()

	httpClient := &httpClientAdapter{client: server.Client()}
	apiClient, err := runtime.NewAPIClient(server.URL, runtime.WithHTTPClient(httpClient))
	require.NoError(t, err)

	client := NewClient(apiClient)

	userType := UserTypeIndividual
	options := &UpdateUserRequestOptions{
		Body: &User{
			Name: "Test User",
			Type: &userType,
		},
	}

	resp, err := client.UpdateUser(context.Background(), options)

	require.NoError(t, err)
	assert.NotNil(t, resp)

	// Verify request body was properly encoded
	assert.Contains(t, capturedBody, `"name":"Test User"`)
	assert.Contains(t, capturedBody, `"type":"individual"`)
}
