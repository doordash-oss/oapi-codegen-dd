// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your business logic.
// To regenerate, delete this file or set output.scaffold-once-overwrite: true in config.
package api

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/chi/other-pkg-mult-files/types"
	"github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/testdata"
	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
)

// Service handles API requests.
type Service struct {
	mu      sync.RWMutex
	avatars map[string][]byte
}

// NewService creates a new Service.
func NewService() *Service {
	return &Service{avatars: make(map[string][]byte)}
}

var _ ServiceInterface = (*Service)(nil)

// HealthCheck handles GET /health
func (s *Service) HealthCheck(ctx context.Context) (*HealthCheckResponseData, error) {
	status := "OK"
	return NewHealthCheckResponseData(&status), nil
}

// ListUsers handles GET /users
func (s *Service) ListUsers(ctx context.Context, opts *ListUsersServiceRequestOptions) (*ListUsersResponseData, error) {
	fixtures := testdata.Users()
	users := make(types.ListUsersResponse, len(fixtures))
	for i, f := range fixtures {
		users[i] = types.User{
			ID:    f.ID,
			Name:  f.Name,
			Email: f.Email,
		}
	}
	headers := make(http.Header)
	headers.Set("X-Total-Count", fmt.Sprintf("%d", len(users)))
	headers.Set("X-Page-Token", "next-page-token")
	return NewListUsersResponseData(&users).WithHeaders(headers), nil
}

// CreateUser handles POST /users
func (s *Service) CreateUser(ctx context.Context, opts *CreateUserServiceRequestOptions) (*CreateUserResponseData, error) {
	user := types.User{ID: "new-1", Name: opts.Body.Name, Email: opts.Body.Email}
	return NewCreateUserResponseData(&user), nil
}

// ImportUsers handles POST /users/import
func (s *Service) ImportUsers(ctx context.Context, opts *ImportUsersServiceRequestOptions) (*ImportUsersResponseData, error) {
	imported, skipped := 5, 0
	return NewImportUsersResponseData(&types.ImportUsersResponse{Imported: &imported, Skipped: &skipped}), nil
}

// GetUser handles GET /users/{id}
func (s *Service) GetUser(ctx context.Context, opts *GetUserServiceRequestOptions) (*GetUserResponseData, error) {
	// Return user with requested ID for testing path param extraction
	user := types.User{
		ID:    opts.PathParams.ID,
		Name:  "Test User",
		Email: "test@example.com",
	}
	return NewGetUserResponseData(&user), nil
}

// DeleteUser handles DELETE /users/{id}
func (s *Service) DeleteUser(ctx context.Context, opts *DeleteUserServiceRequestOptions) (*DeleteUserResponseData, error) {
	return NewDeleteUserResponseData(nil), nil
}

// GetUserAvatar handles GET /users/{id}/avatar
func (s *Service) GetUserAvatar(ctx context.Context, opts *GetUserAvatarServiceRequestOptions) (*GetUserAvatarResponseData, error) {
	s.mu.RLock()
	avatar, ok := s.avatars[opts.PathParams.ID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("avatar not found")
	}
	file := runtime.File{}
	file.InitFromBytes(avatar, "avatar.png")
	return NewGetUserAvatarResponseData(&file), nil
}

// UploadUserAvatar handles PUT /users/{id}/avatar
func (s *Service) UploadUserAvatar(ctx context.Context, opts *UploadUserAvatarServiceRequestOptions) (*UploadUserAvatarResponseData, error) {
	data, err := io.ReadAll(opts.RawRequest.Body)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.avatars[opts.PathParams.ID] = data
	s.mu.Unlock()
	return NewUploadUserAvatarResponseData(nil), nil
}

// SubmitContactForm handles POST /contact
func (s *Service) SubmitContactForm(ctx context.Context, opts *SubmitContactFormServiceRequestOptions) (*SubmitContactFormResponseData, error) {
	resp := types.SubmitContactFormResponse{"ticketId": "ticket-123"}
	return NewSubmitContactFormResponseData(&resp), nil
}

// CreateNote handles POST /notes
func (s *Service) CreateNote(ctx context.Context, opts *CreateNoteServiceRequestOptions) (*CreateNoteResponseData, error) {
	id := 1
	return NewCreateNoteResponseData(&id), nil
}

// ProcessXMLData handles POST /xml-data
func (s *Service) ProcessXMLData(ctx context.Context, opts *ProcessXMLDataServiceRequestOptions) (*ProcessXMLDataResponseData, error) {
	xmlBytes, err := io.ReadAll(opts.RawRequest.Body)
	if err != nil {
		return nil, err
	}
	var payload types.XMLPayload
	if err := xml.Unmarshal(xmlBytes, &payload); err != nil {
		return nil, err
	}
	responseBytes, _ := xml.Marshal(payload)
	resp := NewProcessXMLDataResponseData(nil)
	resp.Body = responseBytes
	return resp, nil
}

// ExportData handles GET /export
func (s *Service) ExportData(ctx context.Context) (*ExportDataResponseData, error) {
	file := runtime.File{}
	file.InitFromBytes([]byte("exported data"), "export.zip")
	return NewExportDataResponseData(&file), nil
}

// GetOAuthToken handles POST /oauth/token
func (s *Service) GetOAuthToken(ctx context.Context, opts *GetOAuthTokenServiceRequestOptions) (*GetOAuthTokenResponseData, error) {
	expiresIn := 3600
	return NewGetOAuthTokenResponseData(&types.GetOAuthTokenResponse{
		AccessToken: "test-token", TokenType: "bearer", ExpiresIn: &expiresIn,
	}), nil
}

// GetItemsByType handles GET /items/{type}
func (s *Service) GetItemsByType(ctx context.Context, opts *GetItemsByTypeServiceRequestOptions) (*GetItemsByTypeResponseData, error) {
	items := types.GetItemsByTypeResponse{opts.PathParams.Type + "-item1", opts.PathParams.Type + "-item2"}
	return NewGetItemsByTypeResponseData(&items), nil
}

// Search handles GET /search
func (s *Service) Search(ctx context.Context, opts *SearchServiceRequestOptions) (*SearchResponseData, error) {
	q := opts.Query.Q
	if len(q) > 5 && q[:5] == "user:" {
		user := types.User{ID: "1", Name: q[5:], Email: "search@example.com"}
		union := &types.Search_Response_OneOf{Either: runtime.NewEitherFromA[types.User, types.SearchItem](user)}
		return NewSearchResponseData(&types.SearchResponse{Search_Response_OneOf: union}), nil
	}
	item := types.SearchItem{ID: "item-1", Title: q}
	union := &types.Search_Response_OneOf{Either: runtime.NewEitherFromB[types.User, types.SearchItem](item)}
	return NewSearchResponseData(&types.SearchResponse{Search_Response_OneOf: union}), nil
}

// GetStatus handles GET /status
func (s *Service) GetStatus(ctx context.Context) (*GetStatusResponseData, error) {
	status, uptime := "healthy", 12345
	return NewGetStatusResponseData(&types.GetStatusResponse{Status: &status, Uptime: &uptime}), nil
}

// UploadImage handles POST /images
func (s *Service) UploadImage(ctx context.Context, opts *UploadImageServiceRequestOptions) (*UploadImageResponseData, error) {
	id, url := "img-123", "https://example.com/images/img-123"
	return NewUploadImageResponseData(&types.UploadImageResponse{ID: &id, URL: &url}), nil
}

// ListProducts handles GET /products
func (s *Service) ListProducts(ctx context.Context, opts *ListProductsServiceRequestOptions) (*ListProductsResponseData, error) {
	fixtures := testdata.Products()
	fixtures = testdata.FilterProductsByIDs(fixtures, opts.Query.Ids)
	fixtures = testdata.FilterProductsByTags(fixtures, opts.Query.Tags)
	products := make([]types.Product, len(fixtures))
	for i, f := range fixtures {
		products[i] = types.Product{ID: f.ID, Name: f.Name, Price: f.Price, Tags: f.Tags}
	}
	resp := types.ListProductsResponse(products)
	return NewListProductsResponseData(&resp), nil
}

// GetCategory handles GET /categories/{categoryId}
func (s *Service) GetCategory(ctx context.Context, opts *GetCategoryServiceRequestOptions) (*GetCategoryResponseData, error) {
	category := types.Category{ID: opts.PathParams.CategoryID, Name: "Test Category"}
	return NewGetCategoryResponseData(&category), nil
}

// GetItemsByStatus handles GET /items/{type}/{rating}
func (s *Service) GetItemsByStatus(ctx context.Context, opts *GetItemsByStatusServiceRequestOptions) (*GetItemsByStatusResponseData, error) {
	// Return items based on type and rating
	items := types.GetItemsByStatusResponse{
		fmt.Sprintf("item-type-%s-rating-%.1f", opts.PathParams.Type, opts.PathParams.Rating),
	}
	return NewGetItemsByStatusResponseData(&items), nil
}

// GetUserPost handles GET /users/{id}/posts/{postId}
func (s *Service) GetUserPost(ctx context.Context, opts *GetUserPostServiceRequestOptions) (*GetUserPostResponseData, error) {
	f := testdata.NewPost(opts.PathParams.ID, opts.PathParams.PostID)
	post := types.Post{ID: f.ID, UserID: f.UserID, Title: f.Title, Content: f.Content}
	return NewGetUserPostResponseData(&post), nil
}

// CreateOrder handles POST /orders
func (s *Service) CreateOrder(ctx context.Context, opts *CreateOrderServiceRequestOptions) (*CreateOrderResponseData, error) {
	order := types.Order{
		ID: testdata.NewOrderID(), ProductID: opts.Body.ProductID,
		Quantity: opts.Body.Quantity, Status: "pending",
	}
	if err := order.Validate(); err != nil {
		return nil, &types.CreateOrderErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()}
	}
	return NewCreateOrderResponseData(&order), nil
}

// CreateCompany handles POST /companies
func (s *Service) CreateCompany(ctx context.Context, opts *CreateCompanyServiceRequestOptions) (*CreateCompanyResponseData, error) {
	var contacts *types.Company_Contacts
	if opts.Body.Contacts != nil {
		c := make(types.Company_Contacts, len(*opts.Body.Contacts))
		for i, item := range *opts.Body.Contacts {
			c[i] = types.Company_Contacts_Item(item)
		}
		contacts = &c
	}
	company := types.Company{
		ID:       testdata.NewCompanyID(),
		Name:     opts.Body.Name,
		Address:  opts.Body.Address,
		Contacts: contacts,
	}

	return NewCreateCompanyResponseData(&company), nil
}
