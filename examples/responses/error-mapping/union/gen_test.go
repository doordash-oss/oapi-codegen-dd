package gen

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnionErrorsImplementError(t *testing.T) {
	var _ error = WidgetError{}
	var _ error = OrderError{}
	var _ error = CartError{}
	var _ error = GetPaymentErrorResponse{}
}

func TestWidgetError_Error(t *testing.T) {
	t.Run("returns the message of the decoded variant", func(t *testing.T) {
		err := decode[WidgetError](t, `{"_tag":"NotFound","message":"widget w1 does not exist"}`)
		assert.Equal(t, "widget w1 does not exist", err.Error())
	})

	t.Run("returns unknown error for a variant without the field", func(t *testing.T) {
		err := decode[WidgetError](t, `"widget w1 does not exist"`)
		assert.Equal(t, "unknown error", err.Error())
	})

	t.Run("returns unknown error when empty", func(t *testing.T) {
		assert.Equal(t, "unknown error", WidgetError{}.Error())
	})
}

func TestOrderError_Error(t *testing.T) {
	t.Run("returns the message of the variant named by the discriminator", func(t *testing.T) {
		err := decode[OrderError](t, `{"_tag":"Conflict","message":"order o1 is already paid"}`)
		assert.Equal(t, "order o1 is already paid", err.Error())
	})

	t.Run("returns unknown error for a variant without the field", func(t *testing.T) {
		err := decode[OrderError](t, `{"_tag":"Gone","since":"2026-01-01"}`)
		assert.Equal(t, "unknown error", err.Error())
	})
}

func TestCartError_Error(t *testing.T) {
	t.Run("returns the message of the variant under the envelope field", func(t *testing.T) {
		err := decode[CartError](t, `{"error":{"_tag":"NotFound","message":"cart c1 does not exist"}}`)
		assert.Equal(t, "cart c1 does not exist", err.Error())
	})

	t.Run("returns unknown error without the envelope field", func(t *testing.T) {
		assert.Equal(t, "unknown error", CartError{}.Error())
	})
}

func TestGetPaymentErrorResponse_Error(t *testing.T) {
	t.Run("returns the message of the variant named by the discriminator", func(t *testing.T) {
		err := decode[GetPaymentErrorResponse](t, `{"_tag":"conflict","message":"payment p1 is already captured"}`)
		assert.Equal(t, "payment p1 is already captured", err.Error())
	})

	t.Run("returns unknown error for a variant without the field", func(t *testing.T) {
		err := decode[GetPaymentErrorResponse](t, `{"_tag":"gone","since":"2026-01-01"}`)
		assert.Equal(t, "unknown error", err.Error())
	})
}

func TestClientReturnsVariantMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"_tag":"NotFound","message":"widget w1 does not exist"}`)
	}))
	defer srv.Close()

	client, err := NewDefaultClient(srv.URL, runtime.WithStdHTTPClient(srv.Client()))
	require.NoError(t, err)

	_, err = client.GetWidget(context.Background(), &GetWidgetRequestOptions{PathParams: &GetWidgetPath{ID: "w1"}})
	require.Error(t, err)
	assert.Equal(t, "widget w1 does not exist", err.Error())

	var widgetErr WidgetError
	require.ErrorAs(t, err, &widgetErr)
}

func TestClientReturnsInlineUnionMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = fmt.Fprint(w, `{"_tag":"conflict","message":"payment p1 is already captured"}`)
	}))
	defer srv.Close()

	client, err := NewDefaultClient(srv.URL, runtime.WithStdHTTPClient(srv.Client()))
	require.NoError(t, err)

	_, err = client.GetPayment(context.Background(), &GetPaymentRequestOptions{PathParams: &GetPaymentPath{ID: "p1"}})
	require.Error(t, err)
	assert.Equal(t, "payment p1 is already captured", err.Error())

	var paymentErr GetPaymentErrorResponse
	require.ErrorAs(t, err, &paymentErr)
}

func decode[T any](t *testing.T, body string) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal([]byte(body), &v))
	return v
}
