package gen

import (
	"encoding/json"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFiles_Response_UnmarshalJSON(t *testing.T) {
	data := []byte(`[{"a":"val-a"},{"b":"val-b"}]`)

	var resp GetFilesResponse
	err := json.Unmarshal(data, &resp)
	require.NoError(t, err)

	require.Len(t, resp, 2)

	fst := resp[0].GetFiles_Response_OneOf.A.A
	snd := resp[1].GetFiles_Response_OneOf.A.B
	assert.Equal(t, "val-a", *fst)
	assert.Equal(t, "val-b", *snd)
}

func TestGetFiles_Response_MarshalJSON(t *testing.T) {
	resp := GetFilesResponse{
		GetFiles_Response_Item{
			GetFiles_Response_OneOf: &GetFiles_Response_OneOf{
				Either: runtime.NewEitherFromA[GetFiles_Response_OneOf_0, VariantC](
					GetFiles_Response_OneOf_0{
						A: func() *string {
							s := "val-a"
							return &s
						}(),
					},
				),
			},
		},
		GetFiles_Response_Item{
			GetFiles_Response_OneOf: &GetFiles_Response_OneOf{
				Either: runtime.NewEitherFromB[GetFiles_Response_OneOf_0, VariantC](
					VariantC{
						C: func() *string {
							s := "val-c"
							return &s
						}(),
					},
				),
			},
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	expected := `[{"a":"val-a"},{"c":"val-c"}]`
	assert.JSONEq(t, expected, string(data))
}

func TestGetFiles_Response_Item_Validate(t *testing.T) {
	t.Run("valid response with variant A", func(t *testing.T) {
		resp := GetFiles_Response_Item{
			GetFiles_Response_OneOf: &GetFiles_Response_OneOf{
				Either: runtime.NewEitherFromA[GetFiles_Response_OneOf_0, VariantC](
					GetFiles_Response_OneOf_0{
						A: func() *string {
							s := "val-a"
							return &s
						}(),
					},
				),
			},
		}

		err := resp.Validate()
		assert.NoError(t, err)
	})

	t.Run("valid response with variant C", func(t *testing.T) {
		resp := GetFiles_Response_Item{
			GetFiles_Response_OneOf: &GetFiles_Response_OneOf{
				Either: runtime.NewEitherFromB[GetFiles_Response_OneOf_0, VariantC](
					VariantC{
						C: func() *string {
							s := "val-c"
							return &s
						}(),
					},
				),
			},
		}

		err := resp.Validate()
		assert.NoError(t, err)
	})
}
