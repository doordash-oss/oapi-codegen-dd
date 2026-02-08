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

// Handler handles API requests.
type Handler struct {
	mu      sync.RWMutex
	avatars map[string][]byte
}

// NewHandler creates a new Handler.
func NewHandler() *Handler {
	return &Handler{avatars: make(map[string][]byte)}
}

var _ HandlerInterface = (*Handler)(nil)

func (h *Handler) HealthCheck(ctx context.Context) (*HealthCheckResponseData, error) {
	status := "OK"
	return NewHealthCheckResponseData(&status), nil
}

func (h *Handler) ListUsers(ctx context.Context, opts *ListUsersRequestOptions) (*ListUsersResponseData, error) {
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

func (h *Handler) CreateUser(ctx context.Context, opts *CreateUserRequestOptions) (*CreateUserResponseData, error) {
	user := types.User{ID: "new-1", Name: opts.Body.Name, Email: opts.Body.Email}
	return NewCreateUserResponseData(&user), nil
}

func (h *Handler) ImportUsers(ctx context.Context, opts *ImportUsersRequestOptions) (*ImportUsersResponseData, error) {
	imported, skipped := 5, 0
	return NewImportUsersResponseData(&types.ImportUsersResponse{Imported: &imported, Skipped: &skipped}), nil
}

func (h *Handler) GetUser(ctx context.Context, opts *GetUserRequestOptions) (*GetUserResponseData, error) {
	// Return user with requested ID for testing path param extraction
	user := types.User{
		ID:    opts.PathParams.ID,
		Name:  "Test User",
		Email: "test@example.com",
	}
	return NewGetUserResponseData(&user), nil
}

func (h *Handler) DeleteUser(ctx context.Context, opts *DeleteUserRequestOptions) (*DeleteUserResponseData, error) {
	return NewDeleteUserResponseData(nil), nil
}

func (h *Handler) GetUserAvatar(ctx context.Context, opts *GetUserAvatarRequestOptions) (*GetUserAvatarResponseData, error) {
	h.mu.RLock()
	avatar, ok := h.avatars[opts.PathParams.ID]
	h.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("avatar not found")
	}
	file := runtime.File{}
	file.InitFromBytes(avatar, "avatar.png")
	return NewGetUserAvatarResponseData(&file), nil
}

func (h *Handler) UploadUserAvatar(ctx context.Context, opts *UploadUserAvatarRequestOptions) (*UploadUserAvatarResponseData, error) {
	data, err := io.ReadAll(opts.RawRequest.Body)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	h.avatars[opts.PathParams.ID] = data
	h.mu.Unlock()
	return NewUploadUserAvatarResponseData(nil), nil
}

func (h *Handler) SubmitContactForm(ctx context.Context, opts *SubmitContactFormRequestOptions) (*SubmitContactFormResponseData, error) {
	resp := types.SubmitContactFormResponse{"ticketId": "ticket-123"}
	return NewSubmitContactFormResponseData(&resp), nil
}

func (h *Handler) CreateNote(ctx context.Context, opts *CreateNoteRequestOptions) (*CreateNoteResponseData, error) {
	id := 1
	return NewCreateNoteResponseData(&id), nil
}

func (h *Handler) ProcessXMLData(ctx context.Context, opts *ProcessXMLDataRequestOptions) (*ProcessXMLDataResponseData, error) {
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

func (h *Handler) ExportData(ctx context.Context) (*ExportDataResponseData, error) {
	file := runtime.File{}
	file.InitFromBytes([]byte("exported data"), "export.zip")
	return NewExportDataResponseData(&file), nil
}

func (h *Handler) GetOAuthToken(ctx context.Context, opts *GetOAuthTokenRequestOptions) (*GetOAuthTokenResponseData, error) {
	expiresIn := 3600
	return NewGetOAuthTokenResponseData(&types.GetOAuthTokenResponse{
		AccessToken: "test-token", TokenType: "bearer", ExpiresIn: &expiresIn,
	}), nil
}

func (h *Handler) GetItemsByType(ctx context.Context, opts *GetItemsByTypeRequestOptions) (*GetItemsByTypeResponseData, error) {
	items := types.GetItemsByTypeResponse{opts.PathParams.Type + "-item1", opts.PathParams.Type + "-item2"}
	return NewGetItemsByTypeResponseData(&items), nil
}

func (h *Handler) Search(ctx context.Context, opts *SearchRequestOptions) (*SearchResponseData, error) {
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

func (h *Handler) GetStatus(ctx context.Context) (*GetStatusResponseData, error) {
	status, uptime := "healthy", 12345
	return NewGetStatusResponseData(&types.GetStatusResponse{Status: &status, Uptime: &uptime}), nil
}

func (h *Handler) UploadImage(ctx context.Context, opts *UploadImageRequestOptions) (*UploadImageResponseData, error) {
	id, url := "img-123", "https://example.com/images/img-123"
	return NewUploadImageResponseData(&types.UploadImageResponse{ID: &id, URL: &url}), nil
}

func (h *Handler) ListProducts(ctx context.Context, opts *ListProductsRequestOptions) (*ListProductsResponseData, error) {
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

func (h *Handler) GetCategory(ctx context.Context, opts *GetCategoryRequestOptions) (*GetCategoryResponseData, error) {
	category := types.Category{ID: opts.PathParams.CategoryID, Name: "Test Category"}
	return NewGetCategoryResponseData(&category), nil
}

func (h *Handler) GetItemsByStatus(ctx context.Context, opts *GetItemsByStatusRequestOptions) (*GetItemsByStatusResponseData, error) {
	// Return items based on active status and rating
	items := types.GetItemsByStatusResponse{
		fmt.Sprintf("item-active-%v-rating-%.1f", opts.PathParams.Active, opts.PathParams.Rating),
	}
	return NewGetItemsByStatusResponseData(&items), nil
}

func (h *Handler) GetUserPost(ctx context.Context, opts *GetUserPostRequestOptions) (*GetUserPostResponseData, error) {
	f := testdata.NewPost(opts.PathParams.UserID, opts.PathParams.PostID)
	post := types.Post{ID: f.ID, UserID: f.UserID, Title: f.Title, Content: f.Content}
	return NewGetUserPostResponseData(&post), nil
}

func (h *Handler) CreateOrder(ctx context.Context, opts *CreateOrderRequestOptions) (*CreateOrderResponseData, error) {
	order := types.Order{
		ID: testdata.NewOrderID(), ProductID: opts.Body.ProductID,
		Quantity: opts.Body.Quantity, Status: "pending",
	}
	if err := order.Validate(); err != nil {
		return nil, &types.CreateOrderErrorResponse{Code: "VALIDATION_ERROR", Message: err.Error()}
	}
	return NewCreateOrderResponseData(&order), nil
}

func (h *Handler) CreateCompany(ctx context.Context, opts *CreateCompanyRequestOptions) (*CreateCompanyResponseData, error) {
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
