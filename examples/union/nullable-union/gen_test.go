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
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvent_RequiredNullable(t *testing.T) {
	t.Run("null decodes to nil and encodes back as null", func(t *testing.T) {
		var event Event
		require.NoError(t, json.Unmarshal([]byte(`{"updatedAt":null,"note":null}`), &event))
		assert.Nil(t, event.UpdatedAt)
		assert.Nil(t, event.Note)

		out, err := json.Marshal(event)
		require.NoError(t, err)
		assert.JSONEq(t, `{"updatedAt":null,"note":null}`, string(out))
	})

	t.Run("a value encodes as usual", func(t *testing.T) {
		updatedAt := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
		location := "Berlin"

		out, err := json.Marshal(Event{UpdatedAt: &updatedAt, Location: &location})
		require.NoError(t, err)
		assert.JSONEq(t, `{"updatedAt":"2026-10-08T12:00:00Z","note":null,"location":"Berlin"}`, string(out))
	})

	t.Run("generated MarshalJSON writes the key too", func(t *testing.T) {
		var event EventWithExtras
		require.NoError(t, json.Unmarshal([]byte(`{"updatedAt":null,"source":"api"}`), &event))
		assert.Nil(t, event.UpdatedAt)

		out, err := json.Marshal(event)
		require.NoError(t, err)
		assert.JSONEq(t, `{"updatedAt":null,"source":"api"}`, string(out))
	})
}

func TestUser_OptionalNullable(t *testing.T) {
	out, err := json.Marshal(User{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(out))
}
