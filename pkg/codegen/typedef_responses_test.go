// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package codegen

import (
	"testing"

	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOperationResponses(t *testing.T) {
	loadOp := func(t *testing.T, contents []byte, path, method string) (*v3.Operation, ParseOptions) {
		t.Helper()
		doc, err := libopenapi.NewDocument(contents)
		require.NoError(t, err)
		model, errs := doc.BuildV3Model()
		require.Empty(t, errs)

		item := model.Model.Paths.PathItems.GetOrZero(path)
		require.NotNil(t, item, "path %s not found", path)

		var op *v3.Operation
		switch method {
		case "get":
			op = item.Get
		case "post":
			op = item.Post
		case "put":
			op = item.Put
		case "delete":
			op = item.Delete
		}
		require.NotNil(t, op, "operation %s %s not found", method, path)

		opts := ParseOptions{
			typeTracker: newTypeTracker(),
			visited:     map[string]bool{},
			model:       &model.Model,
		}
		return op, opts
	}

	t.Run("default response with content becomes 200 success when no explicit success documented", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        default:
          description: any
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
`)
		op, opts := loadOp(t, contents, "/ping", "get")
		res, _, err := getOperationResponses("ping", op.Responses, opts)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, 200, res.SuccessStatusCode,
			"default with content must be installed as 200 success, not a fabricated 204")
		require.NotNil(t, res.Success)
		assert.True(t, res.Success.IsSuccess)
		assert.Equal(t, "application/json", res.Success.ContentType)
		assert.False(t, res.Success.Schema.IsZero(),
			"the default response's schema must drive the success type")

		// Default must NOT also be installed as a 500 error in this case.
		_, has500 := res.All[500]
		assert.False(t, has500, "default was promoted to success; it must not also appear as 500")

		// No fabricated 204.
		_, has204 := res.All[204]
		assert.False(t, has204)
	})

	t.Run("default response stays as 500 error when explicit success is documented", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  ok:
                    type: boolean
        default:
          description: error
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
`)
		op, opts := loadOp(t, contents, "/ping", "get")
		res, _, err := getOperationResponses("ping", op.Responses, opts)
		require.NoError(t, err)

		assert.Equal(t, 200, res.SuccessStatusCode)
		require.NotNil(t, res.Success)
		assert.True(t, res.Success.IsSuccess)

		require.NotNil(t, res.Error, "default must be installed as the fallback error")
		assert.Equal(t, 500, res.Error.StatusCode)
		assert.False(t, res.Error.IsSuccess)
	})

	t.Run("default response without content falls back to fabricated 204 success", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        default:
          description: any
`)
		op, opts := loadOp(t, contents, "/ping", "get")
		res, _, err := getOperationResponses("ping", op.Responses, opts)
		require.NoError(t, err)

		// No content on default means there's nothing to promote to success;
		// the 204 fallback kicks in.
		assert.Equal(t, 204, res.SuccessStatusCode)
		require.NotNil(t, res.Success)
		assert.Equal(t, "struct{}", res.Success.ResponseName)
	})

	t.Run("empty success body preserves declared ContentType", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /page:
    get:
      operationId: getPage
      responses:
        '200':
          description: html
          content:
            text/html: {}
`)
		op, opts := loadOp(t, contents, "/page", "get")
		res, _, err := getOperationResponses("getPage", op.Responses, opts)
		require.NoError(t, err)

		require.NotNil(t, res.Success)
		assert.Equal(t, 200, res.Success.StatusCode)
		assert.Equal(t, "struct{}", res.Success.ResponseName,
			"no schema = no decoded body, so the response is struct{}")
		assert.Equal(t, "text/html", res.Success.ContentType,
			"the declared media type must survive even when the body schema is empty")
	})

	t.Run("default with content promoted to success generates a Response type, not an ErrorResponse", func(t *testing.T) {
		contents := []byte(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        default:
          description: any
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
`)
		op, opts := loadOp(t, contents, "/ping", "get")
		res, typeDefs, err := getOperationResponses("ping", op.Responses, opts)
		require.NoError(t, err)
		require.NotNil(t, res.Success)

		// The success-side name suffix is "Response", not "ErrorResponse".
		assert.Equal(t, "pingResponse", res.Success.ResponseName)

		var names []string
		for _, td := range typeDefs {
			names = append(names, td.Name)
		}
		assert.Contains(t, names, "pingResponse")
		assert.NotContains(t, names, "pingErrorResponse",
			"when default is promoted to success, no parallel ErrorResponse type should be emitted")
	})
}
