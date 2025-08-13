package codegen

import (
	_ "embed"
	"fmt"
	"os"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadTestDocuments(t *testing.T, partialPath string) (libopenapi.Document, libopenapi.Document) {
	t.Helper()
	partialContent, err := os.ReadFile("testdata/" + partialPath)
	require.NoError(t, err)

	srcDoc, err := LoadDocumentFromContents([]byte(testDocument))
	require.NoError(t, err)

	partialDoc, err := LoadDocumentFromContents(partialContent)
	require.NoError(t, err)
	return srcDoc, partialDoc
}

func loadUserDocuments(t *testing.T, partialPath string) (libopenapi.Document, libopenapi.Document) {
	t.Helper()
	partialContent, err := os.ReadFile("testdata/" + partialPath)
	require.NoError(t, err)

	srcDoc, err := LoadDocumentFromContents([]byte(userDocument))
	require.NoError(t, err)

	partialDoc, err := LoadDocumentFromContents(partialContent)
	require.NoError(t, err)
	return srcDoc, partialDoc
}

func loadPaymentIntentDocuments(t *testing.T, partialPath string) (libopenapi.Document, libopenapi.Document) {
	srcContent, err := os.ReadFile("testdata/intent.yml")
	require.NoError(t, err)

	srcDoc, err := LoadDocumentFromContents(srcContent)
	require.NoError(t, err)

	partialContent, err := os.ReadFile(fmt.Sprintf("testdata/%s", partialPath))
	require.NoError(t, err)

	partialDoc, err := LoadDocumentFromContents(partialContent)
	require.NoError(t, err)

	return srcDoc, partialDoc
}

func getSchemaProperties(t *testing.T, res libopenapi.Document, name string) ([]string, *base.SchemaProxy) {
	t.Helper()

	v3Model, errs := res.BuildV3Model()
	if errs != nil {
		t.Fatalf("error building document: %v", errs)
	}
	model := v3Model.Model
	schemaObj, exists := model.Components.Schemas.Get(name)
	require.True(t, exists)

	props := getPropertyKeys(schemaObj)
	return props, schemaObj
}

func getPropertyKeys(schemaPr *base.SchemaProxy) []string {
	if schemaPr == nil {
		return nil
	}

	var keys []string
	for key := range schemaPr.Schema().Properties.KeysFromOldest() {
		keys = append(keys, key)
	}
	return keys
}

func getPropertyNamesWithExtensions(schemaPr *base.SchemaProxy) []keyValue[string, string] {
	if schemaPr == nil {
		return nil
	}

	var res []keyValue[string, string]
	for key, value := range schemaPr.Schema().Properties.FromOldest() {
		vSchema := value.Schema()
		exts := extractExtensions(vSchema.Extensions)
		for name, extRaw := range exts {
			ext, _ := parseString(extRaw)
			res = append(res, keyValue[string, string]{key: key, value: fmt.Sprintf("%s: %s", name, ext)})
		}
	}
	return res
}

func TestMergeDocuments(t *testing.T) {
	t.Run("empty document", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-paths-empty.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		srcRendered, _ := srcDoc.Render()
		resRendered, _ := res.Render()

		assert.Equal(t, srcRendered, resRendered)
	})

	t.Run("new path appended", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-paths-new.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		assert.Equal(t, 6, model.Paths.PathItems.Len())

		fooPath, exists := model.Paths.PathItems.Get("/foo")
		require.True(t, exists)
		assert.NotNil(t, fooPath.Get)

		barPath, exists := model.Paths.PathItems.Get("/bar")
		require.True(t, exists)
		assert.NotNil(t, barPath.Get)
		assert.NotNil(t, barPath.Post)
	})

	t.Run("new method appended", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-paths-user-patch.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		assert.Equal(t, 4, model.Paths.PathItems.Len())

		userPath, exists := model.Paths.PathItems.Get("/user")
		require.True(t, exists)
		assert.NotNil(t, userPath.Patch)
	})

	t.Run("new params appended", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-paths-existing-params.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		assert.Equal(t, 4, model.Paths.PathItems.Len())

		userPath, exists := model.Paths.PathItems.Get("/test/{name}")
		require.True(t, exists)
		assert.NotNil(t, userPath.Get)

		expectedParams := []string{"name", "$top", "from", "to"}
		var params []string
		for _, v := range userPath.Get.Parameters {
			params = append(params, v.Name)
		}
		assert.Equal(t, expectedParams, params)
	})

	t.Run("new request body appended", func(t *testing.T) {
		srcDoc, partialDoc := loadUserDocuments(t, "partial-paths-req-body-new.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		userPath, exists := model.Paths.PathItems.Get("/user")
		require.True(t, exists)
		assert.NotNil(t, userPath.Post)

		reqBody := userPath.Post.RequestBody
		require.NotNil(t, reqBody)
	})

	t.Run("request bodies merged", func(t *testing.T) {
		srcDoc, partialDoc := loadUserDocuments(t, "partial-paths-req-body-existing.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		userPath, exists := model.Paths.PathItems.Get("/user")
		require.True(t, exists)
		assert.NotNil(t, userPath.Patch)

		reqBody := userPath.Patch.RequestBody
		require.NotNil(t, reqBody)
		expectedKeys := []string{"name", "city", "zip"}
		props := getPropertyKeys(reqBody.Content.Value("application/json").Schema)
		assert.Equal(t, expectedKeys, props)
	})

	t.Run("request bodies with nested properties merged", func(t *testing.T) {
		srcDoc, partialDoc := loadPaymentIntentDocuments(t, "partial-intent.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		pathItem, exists := model.Paths.PathItems.Get("/v1/payment_intents")
		require.True(t, exists)
		assert.NotNil(t, pathItem.Post)

		reqBody := pathItem.Post.RequestBody
		require.NotNil(t, reqBody)
		expectedKeys := []string{"user_data", "payment_method_data"}
		schemaPr := reqBody.Content.Value("application/x-www-form-urlencoded").Schema

		rootProps := getPropertyKeys(schemaPr)
		assert.Equal(t, expectedKeys, rootProps)

		paymentMethodData := schemaPr.Schema().Properties.Value("payment_method_data")
		require.NotNil(t, paymentMethodData)
		expectedPaymentMethodDataProps := []string{"payment_id", "card"}
		paymentMethodDataProps := getPropertyKeys(paymentMethodData)
		assert.Equal(t, expectedPaymentMethodDataProps, paymentMethodDataProps)

		card := paymentMethodData.Schema().Properties.Value("card")
		require.NotNil(t, card)
		expectedCardProps := []string{"exp_month", "exp_year", "last4", "network_token"}
		cardProps := getPropertyKeys(card)
		assert.Equal(t, expectedCardProps, cardProps)
	})

	t.Run("new enums can be added to existing inside request body", func(t *testing.T) {
		srcDoc, partialDoc := loadPaymentIntentDocuments(t, "partial-intent-enums.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		pathItem, exists := model.Paths.PathItems.Get("/v1/payment_intents")
		require.True(t, exists)
		assert.NotNil(t, pathItem.Post)

		reqBody := pathItem.Post.RequestBody
		schemaPr := reqBody.Content.Value("application/x-www-form-urlencoded").Schema

		userData := schemaPr.Schema().Properties.Value("user_data")
		userID := userData.Schema().Properties.Value("user_id")

		expectedUserEnums := []string{"user-1", "user-2", "user-3", "user-4"}
		var actualUserEnums []string
		for _, node := range userID.Schema().Enum {
			actualUserEnums = append(actualUserEnums, node.Value)
		}
		assert.Equal(t, expectedUserEnums, actualUserEnums)

		params := pathItem.Post.Parameters
		expectedApiKeyEnums := []string{"foo", "bar", "car"}
		var actualApiKeyEnums []string

		for _, param := range params {
			if param.Name != "api-key" {
				continue
			}
			for _, node := range param.Schema.Schema().Enum {
				actualApiKeyEnums = append(actualApiKeyEnums, node.Value)
			}
		}
		assert.Equal(t, expectedApiKeyEnums, actualApiKeyEnums)
	})

	t.Run("extensions merged", func(t *testing.T) {
		srcDoc, partialDoc := loadUserDocuments(t, "partial-paths-user-extensions.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		userPath, exists := model.Paths.PathItems.Get("/user")
		require.True(t, exists)
		assert.NotNil(t, userPath.Put)

		reqBody := userPath.Put.RequestBody
		require.NotNil(t, reqBody)
		expectedProps := []keyValue[string, string]{
			{"name", "x-go-name: new-name"},
			{"city", "x-go-name: new-city"},
		}
		props := getPropertyNamesWithExtensions(reqBody.Content.Value("application/json").Schema)
		assert.Equal(t, expectedProps, props)

		user := model.Components.Schemas.Value("User")
		userProps := getPropertyNamesWithExtensions(user)
		expectedUserProps := []keyValue[string, string]{
			{"name", "x-go-name: NewComponentUserName"},
		}
		assert.Equal(t, expectedUserProps, userProps)
	})

	t.Run("request body previously inlined can use overwriting ref", func(t *testing.T) {
		srcDoc, partialDoc := loadPaymentIntentDocuments(t, "partial-intent-req-body-ref.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		epPath, exists := model.Paths.PathItems.Get("/v1/payment_intents")
		require.True(t, exists)
		assert.NotNil(t, epPath.Post)

		reqBody := epPath.Post.RequestBody
		require.NotNil(t, reqBody)
		expectedKeys := []string{"user_data", "payment_method_data", "options"}
		props := getPropertyKeys(reqBody.Content.Value("application/x-www-form-urlencoded").Schema)
		assert.Equal(t, expectedKeys, props)

		paymentMethodData := reqBody.Content.Value("application/x-www-form-urlencoded").Schema.Schema().Properties.Value("payment_method_data")
		require.NotNil(t, paymentMethodData)
		expectedPaymentMethodDataProps := []string{"custom_payment_id", "custom_card"}
		paymentMethodDataProps := getPropertyKeys(paymentMethodData)
		assert.Equal(t, expectedPaymentMethodDataProps, paymentMethodDataProps)

		paymentMethodDataComponent := model.Components.Schemas.Value("custom_payment_method_data")
		require.NotNil(t, paymentMethodDataComponent)
	})

	t.Run("schema previously inlined can use overwriting ref", func(t *testing.T) {
		srcDoc, partialDoc := loadPaymentIntentDocuments(t, "partial-user-schema-existing-ref.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		user := model.Components.Schemas.Value("User")
		require.NotNil(t, user)
		expectedKeys := []string{"id", "name", "errors"}
		props := getPropertyKeys(user)
		assert.Equal(t, expectedKeys, props)

		errors := user.Schema().Properties.Value("errors")
		require.NotNil(t, errors)
		expectedErrorsProps := []string{"length", "error_data"}
		errorsProps := getPropertyKeys(errors)
		assert.Equal(t, expectedErrorsProps, errorsProps)

		errorData := errors.Schema().Properties.Value("error_data").Schema().Items.A
		require.NotNil(t, errorData)
		ref := errorData.GetReference()
		assert.Equal(t, "#/components/schemas/Error", ref)
		expectedErrorDataProps := []string{"code", "message"}
		errorDataProps := getPropertyKeys(errorData)
		assert.Equal(t, expectedErrorDataProps, errorDataProps)
	})

	t.Run("foreign ref can be used in other doc", func(t *testing.T) {
		srcDoc, partialDoc := loadPaymentIntentDocuments(t, "partial-intent-req-body-existing-ref.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		epPath, exists := model.Paths.PathItems.Get("/v1/payment_intents")
		require.True(t, exists)
		assert.NotNil(t, epPath.Post)

		reqBody := epPath.Post.RequestBody
		require.NotNil(t, reqBody)
		expectedKeys := []string{"user_data", "payment_method_data"}
		props := getPropertyKeys(reqBody.Content.Value("application/x-www-form-urlencoded").Schema)
		assert.Equal(t, expectedKeys, props)

		// instead of original user_id we have user object with id and name
		userData := reqBody.Content.Value("application/x-www-form-urlencoded").Schema.Schema().Properties.Value("user_data")
		require.NotNil(t, userData)
		userDataRef := userData.GoLow().GetReference()
		assert.Equal(t, "#/components/schemas/User", userDataRef)
		expectedUserDataProps := []string{"id", "name"}
		userDataProps := getPropertyKeys(userData)
		assert.Equal(t, expectedUserDataProps, userDataProps)

		paymentMethodData := reqBody.Content.Value("application/x-www-form-urlencoded").Schema.Schema().Properties.Value("payment_method_data")
		require.NotNil(t, paymentMethodData)
		expectedPaymentMethodDataProps := []string{"payment_id"}
		paymentMethodDataProps := getPropertyKeys(paymentMethodData)
		assert.Equal(t, expectedPaymentMethodDataProps, paymentMethodDataProps)
	})

	t.Run("new response code appended", func(t *testing.T) {
		srcDoc, partialDoc := loadUserDocuments(t, "partial-paths-user-new-response.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		userPath, exists := model.Paths.PathItems.Get("/user")
		require.True(t, exists)

		responses := userPath.Get.Responses.Codes
		assert.NotNil(t, userPath.Get.Responses.Default)

		assert.Equal(t, 2, responses.Len())
		assert.NotNil(t, responses.Value("200"))
		assert.NotNil(t, responses.Value("201"))
	})

	t.Run("same response code properties merged", func(t *testing.T) {
		srcDoc, partialDoc := loadUserDocuments(t, "partial-paths-user-existing-response.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		userPath, exists := model.Paths.PathItems.Get("/user")
		require.True(t, exists)

		responses := userPath.Put.Responses.Codes
		assert.NotNil(t, userPath.Get.Responses.Default)

		assert.Equal(t, 1, responses.Len())
		resp := responses.Value("200")
		require.NotNil(t, resp)

		expectedProps := []string{"name", "age", "city", "zip"}
		props := getPropertyKeys(resp.Content.Value("application/json").Schema)
		assert.Equal(t, expectedProps, props)
	})

	t.Run("new schema is injected", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-schema-person.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		props, _ := getSchemaProperties(t, res, "Person")
		expected := []string{"name", "address"}
		assert.Equal(t, expected, props)
	})

	t.Run("new properties can be added to existing", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-schema-cat-alive.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		props, _ := getSchemaProperties(t, res, "CatAlive")
		expected := []string{"name", "alive_since", "nickname", "age"}
		assert.Equal(t, expected, props)
	})

	t.Run("existing properties can be overwritten with corresponding attributes", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-schema-error-overwrites.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		props, errSchemaPr := getSchemaProperties(t, res, "Error")
		expected := []string{"code", "message"}
		assert.Equal(t, expected, props)

		required := errSchemaPr.Schema().Required
		assert.Equal(t, []string{"code", "message"}, required)

		codeSchema := errSchemaPr.Schema().Properties.Value("code").Schema()
		assert.Equal(t, 1000, int(*codeSchema.Minimum))
		assert.Equal(t, 2000, int(*codeSchema.Maximum))
	})

	t.Run("new nested properties can be added to existing", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-schema-location.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		props, schemaPr := getSchemaProperties(t, res, "Location")
		expected := []string{"coordinates", "address"}
		assert.Equal(t, expected, props)

		addressSch, exists := schemaPr.Schema().Properties.Get("address")
		require.True(t, exists)
		props = getPropertyKeys(addressSch)
		expected = []string{"street", "city", "owner", "zip"}
		assert.Equal(t, expected, props)

		ownerSch, exists := addressSch.Schema().Properties.Get("owner")
		require.True(t, exists)
		props = getPropertyKeys(ownerSch)
		expected = []string{"name", "age"}
		assert.Equal(t, expected, props)
	})

	t.Run("all-of, any-of appended", func(t *testing.T) {
		srcDoc, partialDoc := loadTestDocuments(t, "partial-schema-user.yml")

		res, err := MergeDocuments(srcDoc, partialDoc)
		require.NoError(t, err)

		v3Model, errs := res.BuildV3Model()
		if errs != nil {
			t.Fatalf("error building document: %v", errs)
		}
		model := v3Model.Model

		schemaObj, exists := model.Components.Schemas.Get("User")
		require.True(t, exists)

		allOfSchemas := schemaObj.Schema().AllOf
		require.Len(t, allOfSchemas, 2)

		anyOfSchemas := schemaObj.Schema().AnyOf
		require.Len(t, anyOfSchemas, 2)

		notSchemas := schemaObj.Schema().Not
		assert.NotNil(t, notSchemas)
	})
}
