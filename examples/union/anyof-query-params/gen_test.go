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
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
)

// A multi-variant union query parameter must reach the query string
// instead of marshaling to an empty object.
func TestUnionQueryParamIsEncoded(t *testing.T) {
	id := WidgetID("w1")

	t.Run("flattened union", func(t *testing.T) {
		opts := &ListGadgetsRequestOptions{Query: &ListGadgetsQuery{
			ParentID: &ListGadgetsQuery_ParentID{
				ListGadgets_Query_ParentID_AnyOf: &ListGadgets_Query_ParentID_AnyOf{
					Either: runtime.NewEitherFromA[WidgetID, string](id),
				},
			},
		}}
		m, err := opts.GetQuery()
		if err != nil {
			t.Fatal(err)
		}
		if got := m["parentID"]; got != "w1" {
			t.Fatalf("parentID = %#v, want \"w1\"", got)
		}
	})

	t.Run("nested union", func(t *testing.T) {
		opts := &ListWidgetsRequestOptions{Query: &ListWidgetsQuery{
			ParentID: &ListWidgetsQuery_ParentID{
				ListWidgets_Query_ParentID_AnyOf_AnyOf: &ListWidgets_Query_ParentID_AnyOf_AnyOf{
					Either: runtime.NewEitherFromB[WidgetID, string]("null"),
				},
			},
		}}
		m, err := opts.GetQuery()
		if err != nil {
			t.Fatal(err)
		}
		if got := m["parentID"]; got != "null" {
			t.Fatalf("parentID = %#v, want \"null\"", got)
		}
	})
}
