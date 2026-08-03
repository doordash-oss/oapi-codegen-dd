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

// TestClassicVsEnvelope demonstrates that *Client satisfies both
// ClientInterface (classic, body-only return) and ClientWithResponseInterface
// (envelope) when generate.client and generate.client-with-response are both
// true. Each call site picks the ergonomics that fit.
func TestClassicVsEnvelope(t *testing.T) {
	t.Run("envelope returns 201 success", func(t *testing.T) {
		docID := uuid.New()
		srv := newServer(t, http.StatusCreated, DocumentStored{
			ID: docID, URL: "https://example.com/doc/abc",
		}, map[string]string{"Location": "https://example.com/doc/abc", "X-Request-Id": "r1"})
		defer srv.Close()
		client := newClient(t, srv)

		resp, err := client.UploadDocumentWithResponse(t.Context(), uploadOpts())
		require.NoError(t, err)
		require.NotNil(t, resp.JSON201)
		assert.Equal(t, docID, resp.JSON201.ID)
		assert.Equal(t, "https://example.com/doc/abc", resp.Headers201.Location)
	})

	t.Run("classic still works for a single 2xx (202 case)", func(t *testing.T) {
		jobID := uuid.New()
		srv := newServer(t, http.StatusAccepted, DocumentQueued{
			JobID: jobID, EstimatedCompletionSeconds: 30,
		}, nil)
		defer srv.Close()
		client := newClient(t, srv)

		// Classic picks 202 (last 2xx in spec order). For specs like ours, this
		// means the classic call works for 202 responses but treats 201 as an
		// error. Callers who need 201 (or headers) should use the envelope sibling.
		resp, err := client.UploadDocument(t.Context(), uploadOpts())
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, jobID, resp.JobID)
	})

	t.Run("a single ClientInterface covers both classic and envelope methods", func(t *testing.T) {
		// When both flags are on, the generated ClientInterface lists every
		// classic method plus every <Op>WithResponse sibling, so a single mock
		// satisfies both shapes. *Client implements it.
		var _ ClientInterface = (*Client)(nil)
	})
}
