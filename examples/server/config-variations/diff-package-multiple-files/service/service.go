// Package service This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your business logic.
// To regenerate, delete this file or set generate.handler.output.overwrite: true in config.
package service

import (
	"context"

	"github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/config-variations/diff-package-multiple-files/models"
)

// Service implements the ServiceInterface.
// Add your dependencies here (database, clients, etc.)
type Service struct {
}

// NewService creates a new Service.
func NewService() *Service {
	return &Service{}
}

// Ensure Service implements models.ServiceInterface.
var _ models.ServiceInterface = (*Service)(nil)

// HealthCheck handles GET /health
// Health check endpoint
func (s *Service) HealthCheck(ctx context.Context) (*models.HealthCheckResponseData, error) {
	// TODO: Implement your business logic here
	return models.NewHealthCheckResponseData(new(models.HealthCheckResponse)), nil
}

// ListUsers handles GET /users
// List all users
func (s *Service) ListUsers(ctx context.Context, opts *models.ListUsersServiceRequestOptions) (*models.ListUsersResponseData, error) {
	// TODO: Implement your business logic here
	return models.NewListUsersResponseData(new(models.ListUsersResponse)), nil
}

// CreateUser handles POST /users
// Create a new user
func (s *Service) CreateUser(ctx context.Context, opts *models.CreateUserServiceRequestOptions) (*models.CreateUserResponseData, error) {
	// TODO: Implement your business logic here
	return models.NewCreateUserResponseData(new(models.CreateUserResponse)), nil
}

// GetUser handles GET /users/{id}
// Get a user by ID
func (s *Service) GetUser(ctx context.Context, opts *models.GetUserServiceRequestOptions) (*models.GetUserResponseData, error) {
	// TODO: Implement your business logic here
	return models.NewGetUserResponseData(new(models.GetUserResponse)), nil
}

// DeleteUser handles DELETE /users/{id}
// Delete a user
func (s *Service) DeleteUser(ctx context.Context, opts *models.DeleteUserServiceRequestOptions) (*models.DeleteUserResponseData, error) {
	// TODO: Implement your business logic here
	return models.NewDeleteUserResponseData(nil), nil
}
