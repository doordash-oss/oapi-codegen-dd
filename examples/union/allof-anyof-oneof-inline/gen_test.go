package gen

import (
	"encoding/json"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createUserBodyPagesItem(t *testing.T) CreateUserBody_Pages_Item {
	t.Helper()
	var (
		tag1 string
		tag2 string
	)

	tag1 = "tag1"
	tag2 = "tag2"

	anyOfValue := CreateUserBody_Pages_AnyOf_1{
		Query: "q",
	}
	anyOf := runtime.NewEitherFromB[CreateUserBody_Pages_AnyOf_0, CreateUserBody_Pages_AnyOf_1](anyOfValue)

	oneOfValue := CreateUserBody_Pages_OneOf_0{
		First:  1,
		Second: 21,
	}
	oneOf := runtime.NewEitherFromA[CreateUserBody_Pages_OneOf_0, CreateUserBody_Pages_OneOf_1](oneOfValue)

	return CreateUserBody_Pages_Item{
		Limit: 10,
		Tag1:  &tag1,
		Tag2:  &tag2,
		CreateUserBody_Pages_AnyOf: &CreateUserBody_Pages_AnyOf{
			Either: anyOf,
		},
		CreateUserBody_Pages_OneOf: &CreateUserBody_Pages_OneOf{
			Either: oneOf,
		},
	}
}

func TestCreateUserBody_Pages_Item_MarshalJSON(t *testing.T) {
	expectedJSON := `{
        "limit": 10,
        "tag1": "tag1",
        "tag2": "tag2",
        "query": "q",
        "first": 1,
        "second": 21
    }`

	res, err := json.Marshal(createUserBodyPagesItem(t))
	require.NoError(t, err)

	assert.JSONEq(t, expectedJSON, string(res))
}

func TestCreateUserBody_Pages_Item_UnmarshalJSON(t *testing.T) {
	data := []byte(`{
        "limit": 10,
        "tag1": "tag1",
        "tag2": "tag2",
        "query": "q",
        "first": 1,
        "second": 21
    }`)
	var obj CreateUserBody_Pages_Item
	err := json.Unmarshal(data, &obj)
	require.NoError(t, err)

	expected := createUserBodyPagesItem(t)

	assert.Equal(t, expected, obj)
}
