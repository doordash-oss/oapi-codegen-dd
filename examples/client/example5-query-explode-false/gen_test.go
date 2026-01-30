package example5_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	example5 "github.com/doordash-oss/oapi-codegen-dd/v3/examples/client/example5-query-explode-false"
	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httpClientAdapter wraps http.Client to implement runtime.HttpRequestDoer
type httpClientAdapter struct {
	client *http.Client
}

func (a *httpClientAdapter) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return a.client.Do(req.WithContext(ctx))
}

func TestQueryExplodeFalse(t *testing.T) {
	tests := []struct {
		name          string
		expand        []string
		status        []string
		expectedQuery string
	}{
		{
			name:          "expand with explode=false uses comma-separated values",
			expand:        []string{"customer", "invoice"},
			expectedQuery: "expand=customer,invoice",
		},
		{
			name:          "status with explode=true (default) uses repeated params",
			status:        []string{"pending", "completed"},
			expectedQuery: "status=completed&status=pending",
		},
		{
			name:          "both expand and status",
			expand:        []string{"customer", "invoice"},
			status:        []string{"pending"},
			expectedQuery: "expand=customer,invoice&status=pending",
		},
		{
			name:          "expand with comma in value escapes the comma",
			expand:        []string{"a", "b,c"},
			expectedQuery: "expand=a,b%2Cc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedQuery string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id": "ch_123", "amount": 1000, "currency": "usd"}`))
			}))
			defer server.Close()

			httpClient := &httpClientAdapter{client: server.Client()}
			apiClient, err := runtime.NewAPIClient(server.URL, runtime.WithHTTPClient(httpClient))
			require.NoError(t, err)

			client := example5.NewClient(apiClient)

			options := &example5.GetChargeRequestOptions{
				PathParams: &example5.GetChargePath{ID: "ch_123"},
				Query:      &example5.GetChargeQuery{},
			}

			if len(tt.expand) > 0 {
				options.Query.Expand = tt.expand
			}
			if len(tt.status) > 0 {
				options.Query.Status = tt.status
			}

			_, err = client.GetCharge(context.Background(), options)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedQuery, capturedQuery)
		})
	}
}
