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
	"testing"

	"github.com/stretchr/testify/assert"
)

type searchService struct {
	query *SearchQuery
}

func (s *searchService) Search(_ context.Context, opts *SearchServiceRequestOptions) (*SearchResponseData, error) {
	s.query = opts.Query
	return NewSearchResponseData(nil), nil
}

func TestSearch_ReferencedArrayParams(t *testing.T) {
	search := func(t *testing.T, rawQuery string) (int, *SearchQuery) {
		t.Helper()
		svc := &searchService{}
		rec := httptest.NewRecorder()
		NewRouter(svc).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/search?"+rawQuery, nil))
		return rec.Code, svc.query
	}

	t.Run("a component parameter reads every value", func(t *testing.T) {
		code, query := search(t, "term=one&term=two")
		assert.Equal(t, http.StatusNoContent, code)
		assert.Equal(t, Terms{"one", "two"}, query.Term)
	})

	t.Run("its items are validated", func(t *testing.T) {
		code, query := search(t, "term=one&term=toolong")
		assert.Equal(t, http.StatusBadRequest, code)
		assert.Nil(t, query, "the service is not called")
	})

	t.Run("a referenced schema validates itself", func(t *testing.T) {
		code, query := search(t, "tag=a&tag=b")
		assert.Equal(t, http.StatusNoContent, code)
		assert.Equal(t, TagList{"a", "b"}, query.Tag)

		code, _ = search(t, "tag=a&tag=b&tag=c")
		assert.Equal(t, http.StatusBadRequest, code, "TagList allows two items")
	})

	t.Run("items are parsed by their type", func(t *testing.T) {
		code, query := search(t, "id=1&id=2")
		assert.Equal(t, http.StatusNoContent, code)
		assert.Equal(t, IDList{1, 2}, query.ID)

		code, _ = search(t, "id=one")
		assert.Equal(t, http.StatusBadRequest, code)
	})
}
