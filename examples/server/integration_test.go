package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	chiapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/chi/other-pkg-mult-files/api"
	stdhttpapi "github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/stdhttp/same-pkg-single-file/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serverTestCase struct {
	name      string
	newRouter func() http.Handler
}

func testServers() []serverTestCase {
	return []serverTestCase{
		{"chi", func() http.Handler { return chiapi.NewRouter(chiapi.NewHandler()) }},
		{"std-http", func() http.Handler { return stdhttpapi.NewRouter(stdhttpapi.NewHandler()) }},
	}
}

func TestHealthCheck(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			req := httptest.NewRequest("GET", "/health", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, "OK", rr.Body.String())
		})
	}
}

func TestListUsers(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-Request-ID", "test-123")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var users []map[string]any
			err := json.NewDecoder(rr.Body).Decode(&users)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(users), 1)
		})
	}
}

func TestCreateUser_JSONBody(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			body := `{"name": "Charlie", "email": "charlie@example.com"}`
			req := httptest.NewRequest("POST", "/users", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var user map[string]any
			err := json.NewDecoder(rr.Body).Decode(&user)
			require.NoError(t, err)
			assert.Equal(t, "Charlie", user["name"])
		})
	}
}

func TestGetUser_PathParam(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			req := httptest.NewRequest("GET", "/users/user-123", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var user map[string]any
			err := json.NewDecoder(rr.Body).Decode(&user)
			require.NoError(t, err)
			assert.Equal(t, "user-123", user["id"])
		})
	}
}

func TestDeleteUser(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			req := httptest.NewRequest("DELETE", "/users/1", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusNoContent, rr.Code)
		})
	}
}

func TestSubmitContactForm(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			form := url.Values{}
			form.Set("name", "John")
			form.Set("email", "john@example.com")
			form.Set("message", "Hello!")

			req := httptest.NewRequest("POST", "/contact", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestGetItemsByType_ReservedKeyword(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			req := httptest.NewRequest("GET", "/items/electronics", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var items []string
			err := json.NewDecoder(rr.Body).Decode(&items)
			require.NoError(t, err)
			assert.Contains(t, items[0], "electronics")
		})
	}
}

func TestGetStatus_ReusableResponse(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			req := httptest.NewRequest("GET", "/status", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			var result map[string]any
			err := json.NewDecoder(rr.Body).Decode(&result)
			require.NoError(t, err)
			assert.NotEmpty(t, result["status"])
		})
	}
}

func TestUploadImage_WildcardContentType(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			imageData := []byte("fake-png-image-data")
			req := httptest.NewRequest("POST", "/images", bytes.NewReader(imageData))
			req.Header.Set("Content-Type", "image/png")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusCreated, rr.Code)

			var result map[string]any
			err := json.NewDecoder(rr.Body).Decode(&result)
			require.NoError(t, err)
			assert.NotEmpty(t, result["id"])
		})
	}
}

func TestListUsersResponseHeaders(t *testing.T) {
	// Only chi implementation sets response headers
	h := chiapi.NewRouter(chiapi.NewHandler())
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
	h := chiapi.NewRouter(chiapi.NewHandler())

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
	h := chiapi.NewRouter(chiapi.NewHandler())

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
			h := tc.newRouter()
			form := url.Values{}
			form.Set("grant_type", "client_credentials")
			form.Set("client_id", "my-client")

			req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Contains(t, rr.Body.String(), "access_token")
		})
	}
}

func TestGetCategory_IntegerPathParam(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			req := httptest.NewRequest("GET", "/categories/123", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			var category map[string]any
			err := json.Unmarshal(rr.Body.Bytes(), &category)
			require.NoError(t, err)
			assert.Equal(t, float64(123), category["id"])
			assert.Equal(t, "Test Category", category["name"])
		})
	}
}

func TestListProducts_QueryParams(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			// Test with boolean and integer array query params
			req := httptest.NewRequest("GET", "/products?active=true&categoryIds=1&categoryIds=2", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			var products []map[string]any
			err := json.Unmarshal(rr.Body.Bytes(), &products)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(products), 1)
		})
	}
}

func TestGetItemsByStatus_BoolAndFloatPathParams(t *testing.T) {
	for _, tc := range testServers() {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.newRouter()
			// Test with boolean and float path params
			req := httptest.NewRequest("GET", "/items/true/4.5", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			var items []string
			err := json.Unmarshal(rr.Body.Bytes(), &items)
			require.NoError(t, err)
			assert.Len(t, items, 1)
			assert.Contains(t, items[0], "active-true")
			assert.Contains(t, items[0], "rating-4.5")
		})
	}
}
