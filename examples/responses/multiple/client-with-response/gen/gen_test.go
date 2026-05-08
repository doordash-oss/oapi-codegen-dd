package gen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httpDoer adapts *http.Client to runtime.HttpRequestDoer (which takes ctx as
// its first argument).
type httpDoer struct{ c *http.Client }

func (d httpDoer) Do(_ context.Context, req *http.Request) (*http.Response, error) {
	return d.c.Do(req)
}

func newClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := NewDefaultClient(srv.URL, runtime.WithHTTPClient(httpDoer{srv.Client()}))
	require.NoError(t, err)
	return c
}

// newServer returns a test server that always replies with the given status,
// body and headers.
func newServer(t *testing.T, status int, body any, headers map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func uploadOpts() *UploadDocumentRequestOptions {
	body := UploadDocumentBody{Filename: "doc.pdf", Content: []byte("hi")}
	return &UploadDocumentRequestOptions{Body: &body}
}

func TestUploadDocumentWithResponse(t *testing.T) {
	t.Run("201 sync upload populates JSON201 + Headers201", func(t *testing.T) {
		docID := uuid.New()
		srv := newServer(t, http.StatusCreated, DocumentStored{
			ID:  docID,
			URL: "https://example.com/doc/abc",
		}, map[string]string{
			"Location":     "https://example.com/doc/abc",
			"X-Request-Id": "req-201",
			"X-Custom":     "ad-hoc-undocumented",
		})
		defer srv.Close()

		client := newClient(t, srv)

		resp, err := client.UploadDocumentWithResponse(t.Context(), uploadOpts())
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		require.NotNil(t, resp.JSON201)
		assert.Equal(t, docID, resp.JSON201.ID)
		assert.Equal(t, "https://example.com/doc/abc", resp.JSON201.URL)

		require.NotNil(t, resp.Headers201)
		assert.Equal(t, "https://example.com/doc/abc", resp.Headers201.Location)
		assert.Equal(t, "req-201", resp.Headers201.XRequestID)

		// 202-shaped fields stay nil on a 201 response
		assert.Nil(t, resp.JSON202)
		assert.Nil(t, resp.Headers202)
		assert.Nil(t, resp.JSON422)

		// Undocumented headers reachable via the raw response.
		require.NotNil(t, resp.HTTPResponse)
		assert.Equal(t, "ad-hoc-undocumented", resp.HTTPResponse.Header.Get("X-Custom"))
	})

	t.Run("202 async upload populates JSON202 + Headers202", func(t *testing.T) {
		jobID := uuid.New()
		srv := newServer(t, http.StatusAccepted, DocumentQueued{
			JobID:                      jobID,
			EstimatedCompletionSeconds: 30,
		}, map[string]string{
			"X-Request-Id": "req-202",
			"Retry-After":  "30",
		})
		defer srv.Close()

		client := newClient(t, srv)

		resp, err := client.UploadDocumentWithResponse(t.Context(), uploadOpts())
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		require.NotNil(t, resp.JSON202)
		assert.Equal(t, jobID, resp.JSON202.JobID)
		assert.Equal(t, 30, resp.JSON202.EstimatedCompletionSeconds)

		require.NotNil(t, resp.Headers202)
		assert.Equal(t, "30", resp.Headers202.RetryAfter)
		assert.Equal(t, "req-202", resp.Headers202.XRequestID)

		assert.Nil(t, resp.JSON201)
		assert.Nil(t, resp.Headers201)
	})

	t.Run("422 returns both populated JSON422 and an error", func(t *testing.T) {
		srv := newServer(t, http.StatusUnprocessableEntity, ValidationError{
			Code:    "invalid_filename",
			Message: "filename required",
		}, nil)
		defer srv.Close()

		client := newClient(t, srv)

		resp, err := client.UploadDocumentWithResponse(t.Context(), uploadOpts())
		require.Error(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

		require.NotNil(t, resp.JSON422)
		assert.Equal(t, "invalid_filename", resp.JSON422.Code)
		assert.Equal(t, "filename required", resp.JSON422.Message)
	})

	t.Run("503 text/plain decodes into Text503", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("upstream offline, retry later"))
		}))
		defer srv.Close()

		client := newClient(t, srv)

		resp, err := client.UploadDocumentWithResponse(t.Context(), uploadOpts())
		require.Error(t, err) // documented errors return both envelope and error
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		require.NotNil(t, resp.Text503)
		assert.Equal(t, "upstream offline, retry later", string(*resp.Text503))

		// JSON-typed fields stay nil for text responses
		assert.Nil(t, resp.JSON201)
		assert.Nil(t, resp.JSON422)
	})

	t.Run("undocumented status returns envelope and error", func(t *testing.T) {
		srv := newServer(t, http.StatusTeapot, nil, nil)
		defer srv.Close()

		client := newClient(t, srv)

		resp, err := client.UploadDocumentWithResponse(t.Context(), uploadOpts())
		require.Error(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusTeapot, resp.StatusCode)
		// All typed bodies stay nil for unknown statuses.
		assert.Nil(t, resp.JSON201)
		assert.Nil(t, resp.JSON202)
		assert.Nil(t, resp.JSON422)
	})
}
