// Copyright 2026 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package gen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type arrayService struct {
	query *SearchQuery
	body  *SetTagsBody
}

func (s *arrayService) Search(_ context.Context, opts *SearchServiceRequestOptions) (*SearchResponseData, error) {
	s.query = opts.Query
	return NewSearchResponseData(nil), nil
}

func (s *arrayService) SetTags(_ context.Context, opts *SetTagsServiceRequestOptions) (*SetTagsResponseData, error) {
	s.body = opts.Body
	return NewSetTagsResponseData(nil), nil
}

func TestSearch_InlineArrayItemCounts(t *testing.T) {
	search := func(t *testing.T, rawQuery string) (int, *SearchQuery) {
		t.Helper()
		svc := &arrayService{}
		rec := httptest.NewRecorder()
		NewRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/search?"+rawQuery, nil))
		return rec.Code, svc.query
	}

	t.Run("maxItems", func(t *testing.T) {
		code, query := search(t, "term=a&term=b&term=c")
		assert.Equal(t, http.StatusNoContent, code)
		assert.Equal(t, []string{"a", "b", "c"}, query.Term)

		code, query = search(t, "term=a&term=b&term=c&term=d")
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Nil(t, query, "the service is not called")
	})

	t.Run("items are still validated", func(t *testing.T) {
		code, _ := search(t, "term=a&term=toolong")
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("minItems", func(t *testing.T) {
		code, _ := search(t, "id=1")
		assert.Equal(t, http.StatusBadRequest, code)

		code, query := search(t, "id=1&id=2")
		assert.Equal(t, http.StatusNoContent, code)
		assert.Equal(t, []int{1, 2}, query.ID)
	})

	t.Run("an absent optional array has no item count", func(t *testing.T) {
		code, _ := search(t, "")
		assert.Equal(t, http.StatusNoContent, code)
	})
}

func TestSetTags_InlineArrayItemCounts(t *testing.T) {
	setTags := func(t *testing.T, body string) (int, *SetTagsBody) {
		t.Helper()
		svc := &arrayService{}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/tags", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		NewRouter(svc).ServeHTTP(rec, req)
		return rec.Code, svc.body
	}

	code, body := setTags(t, `{"tags": ["a", "b"]}`)
	assert.Equal(t, http.StatusNoContent, code)
	assert.Equal(t, []string{"a", "b"}, body.Tags)

	for name, payload := range map[string]string{
		"minItems": `{"tags": []}`,
		"maxItems": `{"tags": ["a", "b", "c"]}`,
		"required": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			code, body := setTags(t, payload)
			assert.Equal(t, http.StatusBadRequest, code)
			assert.Nil(t, body, "the service is not called")
		})
	}
}
