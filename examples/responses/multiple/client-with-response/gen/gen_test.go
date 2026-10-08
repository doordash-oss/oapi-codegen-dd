package gen

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := NewDefaultClient(srv.URL, runtime.WithStdHTTPClient(srv.Client()))
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

// requireAPIError asserts err is the client's API error for the given status.
func requireAPIError(t *testing.T, err error, status int) {
	t.Helper()
	var apiErr *runtime.ClientAPIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, status, apiErr.StatusCode())
}

func getDocumentOpts() *GetDocumentRequestOptions {
	return &GetDocumentRequestOptions{PathParams: &GetDocumentPath{ID: "doc-1"}}
}

func TestGetDocumentWithResponse(t *testing.T) {
	t.Run("an exact code takes precedence over the range covering it", func(t *testing.T) {
		srv := newServer(t, http.StatusNotFound, NotFoundError{ID: "doc-1"}, nil)
		defer srv.Close()

		resp, err := newClient(t, srv).GetDocumentWithResponse(t.Context(), getDocumentOpts())
		requireAPIError(t, err, http.StatusNotFound)
		require.NotNil(t, resp.JSON404)
		assert.Equal(t, "doc-1", resp.JSON404.ID)
		assert.Nil(t, resp.JSON4XX)
	})

	t.Run("an exact code without a body is not decoded as its range", func(t *testing.T) {
		srv := newServer(t, http.StatusGone, ValidationError{Code: "gone", Message: "deleted"}, nil)
		defer srv.Close()

		resp, err := newClient(t, srv).GetDocumentWithResponse(t.Context(), getDocumentOpts())
		requireAPIError(t, err, http.StatusGone)
		assert.Nil(t, resp.JSON4XX)
	})

	t.Run("4XX decodes any other client error", func(t *testing.T) {
		srv := newServer(t, http.StatusConflict, ValidationError{Code: "conflict", Message: "busy"}, nil)
		defer srv.Close()

		resp, err := newClient(t, srv).GetDocumentWithResponse(t.Context(), getDocumentOpts())
		requireAPIError(t, err, http.StatusConflict)
		require.NotNil(t, resp.JSON4XX)
		assert.Equal(t, "conflict", resp.JSON4XX.Code)
		assert.Nil(t, resp.JSON404)
		assert.Nil(t, resp.JSON5XX)
	})

	// Earlier versions exposed 4XX as JSON400. The field stays, deprecated, so
	// code written against it keeps compiling and now sees every 4xx.
	t.Run("the field earlier versions used for 4XX stays as an alias", func(t *testing.T) {
		for _, status := range []int{http.StatusBadRequest, http.StatusConflict} {
			srv := newServer(t, status, ValidationError{Code: "invalid", Message: "nope"}, nil)

			resp, err := newClient(t, srv).GetDocumentWithResponse(t.Context(), getDocumentOpts())
			srv.Close()
			requireAPIError(t, err, status)
			require.NotNil(t, resp.JSON4XX)
			assert.Same(t, resp.JSON4XX, resp.JSON400)
		}
	})

	t.Run("5XX decodes into its own type and headers", func(t *testing.T) {
		srv := newServer(t, http.StatusServiceUnavailable, ServiceError{Message: "overloaded"}, map[string]string{
			"Retry-After": "30",
		})
		defer srv.Close()

		resp, err := newClient(t, srv).GetDocumentWithResponse(t.Context(), getDocumentOpts())
		requireAPIError(t, err, http.StatusServiceUnavailable)
		require.NotNil(t, resp.JSON5XX)
		assert.Equal(t, "overloaded", resp.JSON5XX.Message)
		require.NotNil(t, resp.Headers5XX)
		assert.Equal(t, "30", resp.Headers5XX.RetryAfter)
		assert.Nil(t, resp.JSON4XX)
	})

	t.Run("a status no response covers is unexpected", func(t *testing.T) {
		srv := newServer(t, http.StatusNotModified, nil, nil)
		defer srv.Close()

		resp, err := newClient(t, srv).GetDocumentWithResponse(t.Context(), getDocumentOpts())
		requireAPIError(t, err, http.StatusNotModified)
		assert.Nil(t, resp.JSON200)
		assert.Nil(t, resp.JSON4XX)
		assert.Nil(t, resp.JSON5XX)
	})
}

func TestDeleteDocumentWithResponse(t *testing.T) {
	opts := &DeleteDocumentRequestOptions{PathParams: &DeleteDocumentPath{ID: "doc-1"}}

	t.Run("204 is a success", func(t *testing.T) {
		srv := newServer(t, http.StatusNoContent, nil, nil)
		defer srv.Close()

		resp, err := newClient(t, srv).DeleteDocumentWithResponse(t.Context(), opts)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Nil(t, resp.JSONDefault)
	})

	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(fmt.Sprintf("default decodes %d", status), func(t *testing.T) {
			srv := newServer(t, status, ServiceError{Message: "cannot delete"}, nil)
			defer srv.Close()

			resp, err := newClient(t, srv).DeleteDocumentWithResponse(t.Context(), opts)
			requireAPIError(t, err, status)
			require.NotNil(t, resp.JSONDefault)
			assert.Equal(t, "cannot delete", resp.JSONDefault.Message)
		})
	}
}

func TestGetHealthWithResponse(t *testing.T) {
	t.Run("default standing in for the success reports success for a 2xx", func(t *testing.T) {
		srv := newServer(t, http.StatusOK, Health{Status: "ok"}, nil)
		defer srv.Close()

		resp, err := newClient(t, srv).GetHealthWithResponse(t.Context())
		require.NoError(t, err)
		require.NotNil(t, resp.JSONDefault)
		assert.Equal(t, "ok", resp.JSONDefault.Status)
	})

	t.Run("and an error for any other status", func(t *testing.T) {
		srv := newServer(t, http.StatusServiceUnavailable, Health{Status: "degraded"}, nil)
		defer srv.Close()

		resp, err := newClient(t, srv).GetHealthWithResponse(t.Context())
		requireAPIError(t, err, http.StatusServiceUnavailable)
		require.NotNil(t, resp.JSONDefault)
		assert.Equal(t, "degraded", resp.JSONDefault.Status)
	})
}
