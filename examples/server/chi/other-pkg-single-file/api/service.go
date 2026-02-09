// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your business logic.
// To regenerate, delete this file or set output.scaffold-once-overwrite: true in config.
package api

import (
	"context"
	"github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/chi/other-pkg-single-file/types"
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

// HealthCheck handles GET /health - Health check endpoint
func (s *Service) HealthCheck(ctx context.Context) (*HealthCheckResponseData, error) {
	// TODO: Implement your business logic here
	return NewHealthCheckResponseData(new(types.HealthCheckResponse)), nil
}

// ListUsers handles GET /users - List all users
func (s *Service) ListUsers(ctx context.Context, opts *ListUsersServiceRequestOptions) (*ListUsersResponseData, error) {
	// TODO: Implement your business logic here
	return NewListUsersResponseData(new(types.ListUsersResponse)), nil
}

// CreateUser handles POST /users - Create a new user via JSON
func (s *Service) CreateUser(ctx context.Context, opts *CreateUserServiceRequestOptions) (*CreateUserResponseData, error) {
	// TODO: Implement your business logic here
	return NewCreateUserResponseData(new(types.CreateUserResponse)), nil
}

// ImportUsers handles POST /users/import - Import users from CSV file
func (s *Service) ImportUsers(ctx context.Context, opts *ImportUsersServiceRequestOptions) (*ImportUsersResponseData, error) {
	// TODO: Implement your business logic here
	return NewImportUsersResponseData(new(types.ImportUsersResponse)), nil
}

// GetUser handles GET /users/{id} - Get a user by ID
func (s *Service) GetUser(ctx context.Context, opts *GetUserServiceRequestOptions) (*GetUserResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetUserResponseData(new(types.GetUserResponse)), nil
}

// DeleteUser handles DELETE /users/{id} - Delete a user
func (s *Service) DeleteUser(ctx context.Context, opts *DeleteUserServiceRequestOptions) (*DeleteUserResponseData, error) {
	// TODO: Implement your business logic here
	return NewDeleteUserResponseData(nil), nil
}

// GetUserAvatar handles GET /users/{id}/avatar - Get user avatar image
func (s *Service) GetUserAvatar(ctx context.Context, opts *GetUserAvatarServiceRequestOptions) (*GetUserAvatarResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetUserAvatarResponseData(new(types.GetUserAvatarResponse)), nil
}

// UploadUserAvatar handles PUT /users/{id}/avatar - Upload user avatar
func (s *Service) UploadUserAvatar(ctx context.Context, opts *UploadUserAvatarServiceRequestOptions) (*UploadUserAvatarResponseData, error) {
	// TODO: Implement your business logic here
	return NewUploadUserAvatarResponseData(nil), nil
}

// SubmitContactForm handles POST /contact - Submit contact form
func (s *Service) SubmitContactForm(ctx context.Context, opts *SubmitContactFormServiceRequestOptions) (*SubmitContactFormResponseData, error) {
	// TODO: Implement your business logic here
	return NewSubmitContactFormResponseData(new(types.SubmitContactFormResponse)), nil
}

// CreateNote handles POST /notes - Create a note from plain text
func (s *Service) CreateNote(ctx context.Context, opts *CreateNoteServiceRequestOptions) (*CreateNoteResponseData, error) {
	// TODO: Implement your business logic here
	return NewCreateNoteResponseData(new(types.CreateNoteResponse)), nil
}

// ProcessXMLData handles POST /xml-data - Process XML data (demonstrates custom content type handling)
func (s *Service) ProcessXMLData(ctx context.Context, opts *ProcessXMLDataServiceRequestOptions) (*ProcessXMLDataResponseData, error) {
	// TODO: Implement your business logic here
	return NewProcessXMLDataResponseData([]byte("TODO: marshal response")), nil
}

// ExportData handles GET /export - Export all data as binary archive
func (s *Service) ExportData(ctx context.Context) (*ExportDataResponseData, error) {
	// TODO: Implement your business logic here
	return NewExportDataResponseData(new(types.ExportDataResponse)), nil
}

// GetOAuthToken handles POST /oauth/token - Get OAuth token (form-encoded response)
func (s *Service) GetOAuthToken(ctx context.Context, opts *GetOAuthTokenServiceRequestOptions) (*GetOAuthTokenResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetOAuthTokenResponseData(new(types.GetOAuthTokenResponse)), nil
}

// GetItemsByType handles GET /items/{type} - Get items by type (tests reserved Go keyword as path param)
func (s *Service) GetItemsByType(ctx context.Context, opts *GetItemsByTypeServiceRequestOptions) (*GetItemsByTypeResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetItemsByTypeResponseData(new(types.GetItemsByTypeResponse)), nil
}

// Search handles GET /search - Search with union type response (oneOf)
func (s *Service) Search(ctx context.Context, opts *SearchServiceRequestOptions) (*SearchResponseData, error) {
	// TODO: Implement your business logic here
	return NewSearchResponseData(new(types.SearchResponse)), nil
}

// GetStatus handles GET /status - Get status (uses reusable response)
func (s *Service) GetStatus(ctx context.Context) (*GetStatusResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetStatusResponseData(new(types.GetStatusResponse)), nil
}

// UploadImage handles POST /images - Upload image (wildcard content type)
func (s *Service) UploadImage(ctx context.Context, opts *UploadImageServiceRequestOptions) (*UploadImageResponseData, error) {
	// TODO: Implement your business logic here
	return NewUploadImageResponseData(new(types.UploadImageResponse)), nil
}

// ListProducts handles GET /products - List products with various query param types
func (s *Service) ListProducts(ctx context.Context, opts *ListProductsServiceRequestOptions) (*ListProductsResponseData, error) {
	// TODO: Implement your business logic here
	return NewListProductsResponseData(new(types.ListProductsResponse)), nil
}

// GetCategory handles GET /categories/{categoryId} - Get a category by ID (integer path param)
func (s *Service) GetCategory(ctx context.Context, opts *GetCategoryServiceRequestOptions) (*GetCategoryResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetCategoryResponseData(new(types.GetCategoryResponse)), nil
}

// GetItemsByStatus handles GET /items/{active}/{rating} - Get items by active status and rating (boolean + number path params)
func (s *Service) GetItemsByStatus(ctx context.Context, opts *GetItemsByStatusServiceRequestOptions) (*GetItemsByStatusResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetItemsByStatusResponseData(new(types.GetItemsByStatusResponse)), nil
}

// GetUserPost handles GET /users/{userId}/posts/{postId} - Get a specific post by a user
func (s *Service) GetUserPost(ctx context.Context, opts *GetUserPostServiceRequestOptions) (*GetUserPostResponseData, error) {
	// TODO: Implement your business logic here
	return NewGetUserPostResponseData(new(types.GetUserPostResponse)), nil
}

// CreateOrder handles POST /orders - Create an order (demonstrates typed error responses)
func (s *Service) CreateOrder(ctx context.Context, opts *CreateOrderServiceRequestOptions) (*CreateOrderResponseData, error) {
	// TODO: Implement your business logic here
	return NewCreateOrderResponseData(new(types.CreateOrderResponse)), nil
}

// CreateCompany handles POST /companies - Create a company with nested address
func (s *Service) CreateCompany(ctx context.Context, opts *CreateCompanyServiceRequestOptions) (*CreateCompanyResponseData, error) {
	// TODO: Implement your business logic here
	return NewCreateCompanyResponseData(new(types.CreateCompanyResponse)), nil
}
