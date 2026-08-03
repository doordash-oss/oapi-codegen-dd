// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your business logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package api

import (
	"context"
)

// Service implements the ServiceInterface.
// Add your dependencies here (database, clients, etc.)
type Service struct {
}

// NewService creates a new Service.
func NewService() *Service {
	return &Service{}
}

// Ensure Service implements ServiceInterface.
var _ ServiceInterface = (*Service)(nil)

// HealthCheck handles GET /health
// Health check endpoint
func (s *Service) HealthCheck(ctx context.Context) (*HealthCheckResponseData, error) {
	// TODO: Implement your business logic here
	return NewHealthCheckResponseData(new(HealthCheckResponse)), nil
}

// ListUsers handles GET /users
// List all users
func (s *Service) ListUsers(ctx context.Context, opts *ListUsersServiceRequestOptions) (*ListUsersResponseData, error) {
	// TODO: Implement your business logic here
	return NewListUsersResponseData(new(ListUsersResponse)), nil
}

// CreateUser handles POST /users
// Create a new user
func (s *Service) CreateUser(ctx context.Context, opts *CreateUserServiceRequestOptions) (*CreateUserResponseData, error) {
	// TODO: Implement your business logic here
	return NewCreateUserResponseData(new(CreateUserResponse)), nil
}

// GetUser handles GET /users/{id}
// Get a user by ID
func (s *Service) GetUser(ctx context.Context, opts *GetUserServiceRequestOptions) (*GetUserResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetUserResponseData(new(GetUserResponse)), nil
}

// DeleteUser handles DELETE /users/{id}
// Delete a user
func (s *Service) DeleteUser(ctx context.Context, opts *DeleteUserServiceRequestOptions) (*DeleteUserResponseData, error) {
	// TODO: Implement your business logic here
	return NewDeleteUserResponseData(nil), nil
}
