package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/beego/beego/v2/server/web"
	beegoapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/beego/testcase"
	chiapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/chi/testcase"
	echoapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/echo/testcase"
	fasthttpapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/fasthttp/testcase"
	fiberapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/fiber/testcase"
	ginapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/gin/testcase"
	gozeroapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/go-zero/testcase"
	goframeapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/goframe/testcase"
	gorillamuxapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/gorilla-mux/testcase"
	hertzapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/hertz/testcase"
	irisapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/iris/testcase"
	kratosapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/kratos/testcase"
	stdhttpapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/test/std-http/testcase"
	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v3"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type serverTestCase struct {
	name    string
	handler testHandler
}

// testHandler abstracts the different ways to test HTTP handlers.
// For net/http compatible handlers, use httpHandler.
// For Fiber, use fiberHandler.
type testHandler interface {
	Do(req *http.Request) (*http.Response, error)
}

// httpHandler wraps an http.Handler for testing.
type httpHandler struct {
	h http.Handler
}

func (h httpHandler) Do(req *http.Request) (*http.Response, error) {
	rr := httptest.NewRecorder()
	h.h.ServeHTTP(rr, req)
	return rr.Result(), nil
}

// fiberHandler wraps a Fiber app for testing.
type fiberHandler struct {
	app *fiber.App
}

func (f fiberHandler) Do(req *http.Request) (*http.Response, error) {
	return f.app.Test(req)
}

// fasthttpHandler wraps a fasthttp.RequestHandler for testing.
type fasthttpHandler struct {
	h fasthttp.RequestHandler
}

func (f fasthttpHandler) Do(req *http.Request) (*http.Response, error) {
	// Convert http.Request to fasthttp.RequestCtx
	ctx := &fasthttp.RequestCtx{}

	// Set method and URI
	ctx.Request.Header.SetMethod(req.Method)
	ctx.Request.SetRequestURI(req.URL.String())

	// Copy headers
	for k, v := range req.Header {
		for _, vv := range v {
			ctx.Request.Header.Add(k, vv)
		}
	}

	// Copy body
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		ctx.Request.SetBody(body)
	}

	// Call the handler
	f.h(ctx)

	// Convert response back to http.Response
	resp := &http.Response{
		StatusCode: ctx.Response.StatusCode(),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(ctx.Response.Body())),
	}

	// Copy response headers
	for key, value := range ctx.Response.Header.All() {
		resp.Header.Add(string(key), string(value))
	}

	return resp, nil
}

func testServers() []serverTestCase {
	// Initialize beego for testing
	web.BConfig.RunMode = web.DEV

	return []serverTestCase{
		{"beego", httpHandler{beegoapi.NewRouter(beegoapi.NewService())}},
		{"chi", httpHandler{chiapi.NewRouter(chiapi.NewService())}},
		{"std-http", httpHandler{stdhttpapi.NewRouter(stdhttpapi.NewService())}},
		{"echo", httpHandler{func() http.Handler {
			e := echo.New()
			echoapi.NewRouter(e, echoapi.NewService())
			return e
		}()}},
		{"gin", httpHandler{func() http.Handler {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			ginapi.NewRouter(r, ginapi.NewService())
			return r
		}()}},
		{"fiber", fiberHandler{func() *fiber.App {
			app := fiber.New()
			fiberapi.NewRouter(app, fiberapi.NewService())
			return app
		}()}},
		{"go-zero", httpHandler{gozeroapi.NewRouter(gozeroapi.NewService())}},
		{"goframe", httpHandler{goframeapi.Handler(goframeapi.NewService())}},
		{"gorilla-mux", httpHandler{gorillamuxapi.NewRouter(gorillamuxapi.NewService())}},
		{"hertz", httpHandler{hertzapi.Handler(hertzapi.NewService())}},
		{"iris", httpHandler{irisapi.Handler(irisapi.NewService())}},
		{"kratos", httpHandler{kratosapi.NewRouter(kratosapi.NewService())}},
		{"fasthttp", fasthttpHandler{fasthttpapi.Handler(fasthttpapi.NewService())}},
	}
}

func TestHealthCheck(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/health", nil)
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.Equal(t, "OK", string(body))
		})
	}
}

func TestListUsers(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-Request-ID", "test-123")
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var users []map[string]any
			err = json.NewDecoder(resp.Body).Decode(&users)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(users), 1)
		})
	}
}

func TestCreateUser_JSONBody(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"name": "Charlie", "email": "charlie@example.com"}`
			req := httptest.NewRequest("POST", "/users", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusCreated, resp.StatusCode)

			var user map[string]any
			err = json.NewDecoder(resp.Body).Decode(&user)
			require.NoError(t, err)
			assert.Equal(t, "Charlie", user["name"])
		})
	}
}

func TestGetUser_PathParam(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/users/user-123", nil)
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var user map[string]any
			err = json.NewDecoder(resp.Body).Decode(&user)
			require.NoError(t, err)
			assert.Equal(t, "user-123", user["id"])
		})
	}
}

func TestDeleteUser(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/users/1", nil)
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		})
	}
}

func TestSubmitContactForm(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{}
			form.Set("name", "John")
			form.Set("email", "john@example.com")
			form.Set("message", "Hello!")

			req := httptest.NewRequest("POST", "/contact", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestGetItemsByType_ReservedKeyword(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/items/electronics", nil)
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var items []string
			err = json.NewDecoder(resp.Body).Decode(&items)
			require.NoError(t, err)
			assert.Contains(t, items[0], "electronics")
		})
	}
}

func TestGetStatus_ReusableResponse(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/status", nil)
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var result map[string]any
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			assert.NotEmpty(t, result["status"])
		})
	}
}

func TestUploadImage_WildcardContentType(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			imageData := []byte("fake-png-image-data")
			req := httptest.NewRequest("POST", "/images", bytes.NewReader(imageData))
			req.Header.Set("Content-Type", "image/png")
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusCreated, resp.StatusCode)

			var result map[string]any
			err = json.NewDecoder(resp.Body).Decode(&result)
			require.NoError(t, err)
			assert.NotEmpty(t, result["id"])
		})
	}
}

func TestListUsersResponseHeaders(t *testing.T) {
	// Only chi implementation sets response headers
	h := chiapi.NewRouter(chiapi.NewService())
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("X-Request-ID", "test-123")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "3", rr.Header().Get("X-Total-Count"))
	assert.Equal(t, "next-page-token", rr.Header().Get("X-Page-Token"))
}

func TestSearch_UnionTypeResponse(t *testing.T) {
	// Only chi implementation has full union type handling
	h := chiapi.NewRouter(chiapi.NewService())

	// Test returning a SearchItem
	req := httptest.NewRequest("GET", "/search?q=test-query", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var result map[string]any
	err := json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "test-query", result["title"])

	// Test returning a User (query starts with "user:")
	req = httptest.NewRequest("GET", "/search?q=user:Alice", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	err = json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "Alice", result["name"])
}

func TestUploadAndGetAvatar(t *testing.T) {
	// Only chi implementation has full avatar handling
	h := chiapi.NewRouter(chiapi.NewService())

	avatarData := []byte("fake-image-data")
	req := httptest.NewRequest("PUT", "/users/1/avatar", bytes.NewReader(avatarData))
	req.Header.Set("Content-Type", "application/octet-stream")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Get avatar
	req = httptest.NewRequest("GET", "/users/1/avatar", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, avatarData, rr.Body.Bytes())
}

func TestGetOAuthToken(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{}
			form.Set("grant_type", "client_credentials")
			form.Set("client_id", "my-client")

			req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			assert.Contains(t, string(body), "access_token")
		})
	}
}

func TestGetCategory_IntegerPathParam(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/categories/123", nil)
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			var category map[string]any
			body, _ := io.ReadAll(resp.Body)
			err = json.Unmarshal(body, &category)
			require.NoError(t, err)
			assert.Equal(t, float64(123), category["id"])
			assert.Equal(t, "Test Category", category["name"])
		})
	}
}

func TestListProducts_QueryParams(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			// Test with boolean and integer array query params
			req := httptest.NewRequest("GET", "/products?active=true&categoryIds=1&categoryIds=2", nil)
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			var products []map[string]any
			body, _ := io.ReadAll(resp.Body)
			err = json.Unmarshal(body, &products)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(products), 1)
		})
	}
}

func TestGetItemsByStatus_TypeAndRatingPathParams(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			// Test with string and float path params
			req := httptest.NewRequest("GET", "/items/electronics/4.5", nil)
			resp, err := tc.handler.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			var items []string
			body, _ := io.ReadAll(resp.Body)
			err = json.Unmarshal(body, &items)
			require.NoError(t, err)
			assert.Len(t, items, 1)
			assert.Contains(t, items[0], "type-electronics")
			assert.Contains(t, items[0], "rating-4.5")
		})
	}
}
