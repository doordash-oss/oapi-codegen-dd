// Package testcase implements test service logic for all frameworks.
// This file is hand-written and copied to each framework's testcase package.
package testcase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
)

// Service implements the ServiceInterface with test logic.
type Service struct {
	avatars map[string][]byte
}

// NewService creates a new Service.
func NewService() *Service {
	return &Service{avatars: make(map[string][]byte)}
}

// Ensure Service implements ServiceInterface.
var _ ServiceInterface = (*Service)(nil)

func ptr[T any](v T) *T { return &v }

// HealthCheck handles GET /health
func (s *Service) HealthCheck(ctx context.Context) (*HealthCheckResponseData, error) {
	resp := HealthCheckResponse("OK")
	return NewHealthCheckResponseData(&resp), nil
}

// ListUsers handles GET /users
func (s *Service) ListUsers(ctx context.Context, opts *ListUsersServiceRequestOptions) (*ListUsersResponseData, error) {
	users := ListUsersResponse{
		{ID: "1", Name: "Alice", Email: "alice@example.com"},
		{ID: "2", Name: "Bob", Email: "bob@example.com"},
		{ID: "3", Name: "Charlie", Email: "charlie@example.com"},
	}
	resp := NewListUsersResponseData(&users)
	resp.Headers = http.Header{}
	resp.Headers.Set("X-Total-Count", "3")
	resp.Headers.Set("X-Page-Token", "next-page-token")
	return resp, nil
}

// CreateUser handles POST /users
func (s *Service) CreateUser(ctx context.Context, opts *CreateUserServiceRequestOptions) (*CreateUserResponseData, error) {
	user := User{ID: "new-user-id", Name: opts.Body.Name, Email: opts.Body.Email}
	resp := NewCreateUserResponseData(&user)
	resp.Status = http.StatusCreated
	return resp, nil
}

// ImportUsers handles POST /users/import
func (s *Service) ImportUsers(ctx context.Context, opts *ImportUsersServiceRequestOptions) (*ImportUsersResponseData, error) {
	result := ImportResult{Imported: ptr(5), Skipped: ptr(0)}
	return NewImportUsersResponseData(&result), nil
}

// GetUser handles GET /users/{id}
func (s *Service) GetUser(ctx context.Context, opts *GetUserServiceRequestOptions) (*GetUserResponseData, error) {
	user := User{ID: opts.PathParams.ID, Name: "Test User", Email: "test@example.com"}
	return NewGetUserResponseData(&user), nil
}

// DeleteUser handles DELETE /users/{id}
func (s *Service) DeleteUser(ctx context.Context, opts *DeleteUserServiceRequestOptions) (*DeleteUserResponseData, error) {
	resp := NewDeleteUserResponseData(nil)
	resp.Status = http.StatusNoContent
	return resp, nil
}

// GetUserAvatar handles GET /users/{id}/avatar
func (s *Service) GetUserAvatar(ctx context.Context, opts *GetUserAvatarServiceRequestOptions) (*GetUserAvatarResponseData, error) {
	if data, ok := s.avatars[opts.PathParams.ID]; ok {
		var file runtime.File
		file.InitFromBytes(data, "avatar")
		return NewGetUserAvatarResponseData(&file), nil
	}
	return nil, fmt.Errorf("avatar not found")
}

// UploadUserAvatar handles PUT /users/{id}/avatar
func (s *Service) UploadUserAvatar(ctx context.Context, opts *UploadUserAvatarServiceRequestOptions) (*UploadUserAvatarResponseData, error) {
	body, _ := io.ReadAll(opts.RawRequest.Body)
	s.avatars[opts.PathParams.ID] = body
	resp := NewUploadUserAvatarResponseData(nil)
	resp.Status = http.StatusNoContent
	return resp, nil
}

// SubmitContactForm handles POST /contact
func (s *Service) SubmitContactForm(ctx context.Context, opts *SubmitContactFormServiceRequestOptions) (*SubmitContactFormResponseData, error) {
	result := SubmitContactFormResponse{"success": true}
	return NewSubmitContactFormResponseData(&result), nil
}

// CreateNote handles POST /notes
func (s *Service) CreateNote(ctx context.Context, opts *CreateNoteServiceRequestOptions) (*CreateNoteResponseData, error) {
	noteID := CreateNoteResponse(1) // Return note ID as integer
	resp := NewCreateNoteResponseData(&noteID)
	resp.Status = http.StatusCreated
	return resp, nil
}

// ProcessXMLData handles POST /xml-data
func (s *Service) ProcessXMLData(ctx context.Context, opts *ProcessXMLDataServiceRequestOptions) (*ProcessXMLDataResponseData, error) {
	return NewProcessXMLDataResponseData([]byte("<result>OK</result>")), nil
}

// ExportData handles GET /export
func (s *Service) ExportData(ctx context.Context) (*ExportDataResponseData, error) {
	var data runtime.File
	data.InitFromBytes([]byte("export-data"), "export.bin")
	return NewExportDataResponseData(&data), nil
}

// GetOAuthToken handles POST /oauth/token
func (s *Service) GetOAuthToken(ctx context.Context, opts *GetOAuthTokenServiceRequestOptions) (*GetOAuthTokenResponseData, error) {
	token := TokenResponse{AccessToken: "test-token", TokenType: "Bearer", ExpiresIn: ptr(3600)}
	return NewGetOAuthTokenResponseData(&token), nil
}

// GetItemsByType handles GET /items/{type}
func (s *Service) GetItemsByType(ctx context.Context, opts *GetItemsByTypeServiceRequestOptions) (*GetItemsByTypeResponseData, error) {
	items := GetItemsByTypeResponse{fmt.Sprintf("item-%s-1", opts.PathParams.Type)}
	return NewGetItemsByTypeResponseData(&items), nil
}

// Search handles GET /search
func (s *Service) Search(ctx context.Context, opts *SearchServiceRequestOptions) (*SearchResponseData, error) {
	q := opts.Query.Q
	var result SearchResponse
	if strings.HasPrefix(q, "user:") {
		name := strings.TrimPrefix(q, "user:")
		user := User{ID: "user-1", Name: name, Email: name + "@example.com"}
		result.Search_Response_OneOf = &Search_Response_OneOf{runtime.NewEitherFromA[User, SearchItem](user)}
	} else {
		item := SearchItem{ID: "item-1", Title: q, Description: ptr("Search result")}
		result.Search_Response_OneOf = &Search_Response_OneOf{runtime.NewEitherFromB[User, SearchItem](item)}
	}
	return NewSearchResponseData(&result), nil
}

// GetStatus handles GET /status
func (s *Service) GetStatus(ctx context.Context) (*GetStatusResponseData, error) {
	status := StatusResponse{Status: ptr("healthy"), Uptime: ptr(3600)}
	return NewGetStatusResponseData(&status), nil
}

// UploadImage handles POST /images
func (s *Service) UploadImage(ctx context.Context, opts *UploadImageServiceRequestOptions) (*UploadImageResponseData, error) {
	result := UploadImageResponse{ID: ptr("img-123"), URL: ptr("http://example.com/img-123")}
	resp := NewUploadImageResponseData(&result)
	resp.Status = http.StatusCreated
	return resp, nil
}

// ListProducts handles GET /products
func (s *Service) ListProducts(ctx context.Context, opts *ListProductsServiceRequestOptions) (*ListProductsResponseData, error) {
	products := ListProductsResponse{{ID: "prod-1", Name: "Product 1", Price: 9.99}}
	return NewListProductsResponseData(&products), nil
}

// GetCategory handles GET /categories/{categoryId}
func (s *Service) GetCategory(ctx context.Context, opts *GetCategoryServiceRequestOptions) (*GetCategoryResponseData, error) {
	category := Category{ID: opts.PathParams.CategoryID, Name: "Test Category"}
	return NewGetCategoryResponseData(&category), nil
}

// GetItemsByStatus handles GET /items/{type}/{rating}
func (s *Service) GetItemsByStatus(ctx context.Context, opts *GetItemsByStatusServiceRequestOptions) (*GetItemsByStatusResponseData, error) {
	items := GetItemsByStatusResponse{fmt.Sprintf("type-%s-rating-%v", opts.PathParams.Type, opts.PathParams.Rating)}
	return NewGetItemsByStatusResponseData(&items), nil
}

// GetUserPost handles GET /users/{id}/posts/{postId}
func (s *Service) GetUserPost(ctx context.Context, opts *GetUserPostServiceRequestOptions) (*GetUserPostResponseData, error) {
	post := Post{ID: opts.PathParams.PostID, UserID: opts.PathParams.ID, Title: "Test Post", Content: "Post content"}
	return NewGetUserPostResponseData(&post), nil
}

// CreateOrder handles POST /orders
func (s *Service) CreateOrder(ctx context.Context, opts *CreateOrderServiceRequestOptions) (*CreateOrderResponseData, error) {
	order := Order{ID: "order-1", Status: Pending}
	return NewCreateOrderResponseData(&order), nil
}

// CreateCompany handles POST /companies
func (s *Service) CreateCompany(ctx context.Context, opts *CreateCompanyServiceRequestOptions) (*CreateCompanyResponseData, error) {
	company := Company{ID: "company-1", Name: opts.Body.Name, Address: opts.Body.Address}
	return NewCreateCompanyResponseData(&company), nil
}
