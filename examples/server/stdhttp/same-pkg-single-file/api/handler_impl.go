// Package api This file is generated ONCE as a starting point and will NOT be overwritten.
// Modify it freely to add your business logic.
// To regenerate, delete this file or set output.scaffold-once-overwrite: true in config.
package api

import (
	"context"
	"fmt"

	"github.com/doordash-oss/oapi-codegen-dd/v3/examples/server/testdata"
	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

var _ HandlerInterface = (*Handler)(nil)

func (h *Handler) HealthCheck(ctx context.Context) (*HealthCheckResponseData, error) {
	status := "OK"
	return NewHealthCheckResponseData(&status), nil
}

func (h *Handler) ListUsers(ctx context.Context, opts *ListUsersRequestOptions) (*ListUsersResponseData, error) {
	fixtures := testdata.Users()
	users := make(ListUsersResponse, len(fixtures))
	for i, f := range fixtures {
		users[i] = User{ID: f.ID, Name: f.Name, Email: f.Email}
	}
	return NewListUsersResponseData(&users), nil
}

func (h *Handler) CreateUser(ctx context.Context, opts *CreateUserRequestOptions) (*CreateUserResponseData, error) {
	user := User{ID: "new-user", Name: opts.Body.Name, Email: opts.Body.Email}
	return NewCreateUserResponseData(&user), nil
}

func (h *Handler) ImportUsers(ctx context.Context, opts *ImportUsersRequestOptions) (*ImportUsersResponseData, error) {
	imported := 10
	resp := ImportUsersResponse{Imported: &imported}
	return NewImportUsersResponseData(&resp), nil
}

func (h *Handler) GetUser(ctx context.Context, opts *GetUserRequestOptions) (*GetUserResponseData, error) {
	user := User{ID: opts.PathParams.ID, Name: "Test User", Email: "test@example.com"}
	return NewGetUserResponseData(&user), nil
}

func (h *Handler) DeleteUser(ctx context.Context, opts *DeleteUserRequestOptions) (*DeleteUserResponseData, error) {
	return NewDeleteUserResponseData(nil), nil
}

func (h *Handler) GetUserAvatar(ctx context.Context, opts *GetUserAvatarRequestOptions) (*GetUserAvatarResponseData, error) {
	return NewGetUserAvatarResponseData(nil), nil
}

func (h *Handler) UploadUserAvatar(ctx context.Context, opts *UploadUserAvatarRequestOptions) (*UploadUserAvatarResponseData, error) {
	return NewUploadUserAvatarResponseData(nil), nil
}

func (h *Handler) SubmitContactForm(ctx context.Context, opts *SubmitContactFormRequestOptions) (*SubmitContactFormResponseData, error) {
	resp := SubmitContactFormResponse{"ticketId": "ticket-123"}
	return NewSubmitContactFormResponseData(&resp), nil
}

func (h *Handler) CreateNote(ctx context.Context, opts *CreateNoteRequestOptions) (*CreateNoteResponseData, error) {
	id := 1
	return NewCreateNoteResponseData(&id), nil
}

func (h *Handler) ProcessXMLData(ctx context.Context, opts *ProcessXMLDataRequestOptions) (*ProcessXMLDataResponseData, error) {
	return NewProcessXMLDataResponseData([]byte("<response/>")), nil
}

func (h *Handler) ExportData(ctx context.Context) (*ExportDataResponseData, error) {
	file := runtime.File{}
	file.InitFromBytes([]byte("exported data"), "export.zip")
	return NewExportDataResponseData(&file), nil
}

func (h *Handler) GetOAuthToken(ctx context.Context, opts *GetOAuthTokenRequestOptions) (*GetOAuthTokenResponseData, error) {
	expiresIn := 3600
	return NewGetOAuthTokenResponseData(&GetOAuthTokenResponse{
		AccessToken: "test-token", TokenType: "bearer", ExpiresIn: &expiresIn,
	}), nil
}

func (h *Handler) GetItemsByType(ctx context.Context, opts *GetItemsByTypeRequestOptions) (*GetItemsByTypeResponseData, error) {
	items := GetItemsByTypeResponse{opts.PathParams.Type + "-item1", opts.PathParams.Type + "-item2"}
	return NewGetItemsByTypeResponseData(&items), nil
}

func (h *Handler) Search(ctx context.Context, opts *SearchRequestOptions) (*SearchResponseData, error) {
	return NewSearchResponseData(new(SearchResponse)), nil
}

func (h *Handler) GetStatus(ctx context.Context) (*GetStatusResponseData, error) {
	status, uptime := "healthy", 12345
	return NewGetStatusResponseData(&GetStatusResponse{Status: &status, Uptime: &uptime}), nil
}

func (h *Handler) UploadImage(ctx context.Context, opts *UploadImageRequestOptions) (*UploadImageResponseData, error) {
	id, url := "img-123", "https://example.com/images/img-123"
	return NewUploadImageResponseData(&UploadImageResponse{ID: &id, URL: &url}), nil
}

func (h *Handler) ListProducts(ctx context.Context, opts *ListProductsRequestOptions) (*ListProductsResponseData, error) {
	fixtures := testdata.Products()
	products := make([]Product, len(fixtures))
	for i, f := range fixtures {
		products[i] = Product{ID: f.ID, Name: f.Name, Price: f.Price, Tags: f.Tags}
	}
	resp := ListProductsResponse(products)
	return NewListProductsResponseData(&resp), nil
}

func (h *Handler) GetCategory(ctx context.Context, opts *GetCategoryRequestOptions) (*GetCategoryResponseData, error) {
	category := Category{ID: opts.PathParams.CategoryID, Name: "Test Category"}
	return NewGetCategoryResponseData(&category), nil
}

func (h *Handler) GetItemsByStatus(ctx context.Context, opts *GetItemsByStatusRequestOptions) (*GetItemsByStatusResponseData, error) {
	items := GetItemsByStatusResponse{
		fmt.Sprintf("item-active-%v-rating-%.1f", opts.PathParams.Active, opts.PathParams.Rating),
	}
	return NewGetItemsByStatusResponseData(&items), nil
}

func (h *Handler) GetUserPost(ctx context.Context, opts *GetUserPostRequestOptions) (*GetUserPostResponseData, error) {
	f := testdata.NewPost(opts.PathParams.UserID, opts.PathParams.PostID)
	post := Post{ID: f.ID, UserID: f.UserID, Title: f.Title, Content: f.Content}
	return NewGetUserPostResponseData(&post), nil
}

func (h *Handler) CreateOrder(ctx context.Context, opts *CreateOrderRequestOptions) (*CreateOrderResponseData, error) {
	order := Order{ID: testdata.NewOrderID(), ProductID: opts.Body.ProductID, Quantity: opts.Body.Quantity, Status: "pending"}
	return NewCreateOrderResponseData(&order), nil
}

func (h *Handler) CreateCompany(ctx context.Context, opts *CreateCompanyRequestOptions) (*CreateCompanyResponseData, error) {
	company := Company{ID: testdata.NewCompanyID(), Name: opts.Body.Name, Address: opts.Body.Address}
	return NewCreateCompanyResponseData(&company), nil
}
