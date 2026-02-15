package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockService implements ServiceInterface for testing
type mockService struct{}

func (m *mockService) HealthCheck(_ context.Context) (*HealthCheckResponseData, error) {
	return nil, nil
}

func (m *mockService) ListUsers(_ context.Context, _ *ListUsersServiceRequestOptions) (*ListUsersResponseData, error) {
	return nil, nil
}

func (m *mockService) CreateUser(_ context.Context, _ *CreateUserServiceRequestOptions) (*CreateUserResponseData, error) {
	return nil, nil
}

func (m *mockService) GetUser(_ context.Context, _ *GetUserServiceRequestOptions) (*GetUserResponseData, error) {
	return nil, nil
}

func (m *mockService) DeleteUser(_ context.Context, _ *DeleteUserServiceRequestOptions) (*DeleteUserResponseData, error) {
	return nil, nil
}

func TestCreateUser_InvalidJSON_ReturnsTypedError(t *testing.T) {
	adapter := NewHTTPAdapter(&mockService{}, nil)
	r := chi.NewRouter()
	r.Post("/users", adapter.CreateUser)

	// Send invalid JSON
	req := httptest.NewRequest("POST", "/users", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Should return 400 with typed error (Error type via CreateUserErrorResponse = Error alias)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp Error
	err := json.Unmarshal(rr.Body.Bytes(), &errResp)
	require.NoError(t, err)

	// Verify the typed error structure - message field should contain the JSON parse error
	require.NotNil(t, errResp.Message)
	assert.Contains(t, *errResp.Message, "invalid character")

	// Code should be nil (not set by NewError constructor)
	assert.Nil(t, errResp.Code)
}
