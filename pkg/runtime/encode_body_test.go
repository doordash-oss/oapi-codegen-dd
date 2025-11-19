// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package runtime

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeFormFields(t *testing.T) {
	type Coordinates struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	type Address struct {
		City        string `json:"city"`
		Country     string `json:"country"`
		Coordinates `json:"coordinates"`
	}

	type User struct {
		ID        int      `json:"id"`
		Name      string   `json:"name"`
		Age       int      `json:"age"`
		Address   Address  `json:"address"`
		Nicknames []string `json:"nicknames"`
	}

	payload := User{
		ID:   123456789,
		Name: "Jane Doe",
		Age:  30,
		Address: Address{
			City:    "Berlin",
			Country: "DE",
			Coordinates: Coordinates{
				Latitude:  52.5200,
				Longitude: 13.4050,
			},
		},
		Nicknames: []string{"JD", "Janie"},
	}

	t.Run("default", func(t *testing.T) {
		res, err := EncodeFormFields(payload, map[string]FieldEncoding{
			"address": {},
			"nicknames": {
				Style: "form",
			},
		})

		resDecoded, _ := url.QueryUnescape(res)
		require.NoError(t, err)
		expected := "address.city=Berlin&address.coordinates.latitude=52.52&address.coordinates.longitude=13.405&address.country=DE&age=30&id=123456789&name=Jane Doe&nicknames=JD&nicknames=Janie"
		assert.Equal(t, expected, resDecoded)
	})

	t.Run("deepObject", func(t *testing.T) {
		res, err := EncodeFormFields(payload, map[string]FieldEncoding{
			"address": {
				Style: "deepObject",
			},
			"nicknames": {
				Style: "deepObject",
			},
		})

		resDecoded, _ := url.QueryUnescape(res)
		require.NoError(t, err)
		expected := "address[city]=Berlin&address[coordinates][latitude]=52.52&address[coordinates][longitude]=13.405&address[country]=DE&age=30&id=123456789&name=Jane Doe&nicknames[0]=JD&nicknames[1]=Janie"
		assert.Equal(t, expected, resDecoded)
	})

	t.Run("different types", func(t *testing.T) {
		data := map[string]any{
			"v1": 1,
			"v2": 1.2,
			"v3": true,
			"v4": "test",
			"v5": 123456789,
			"v6": 123.456789,
			"v7": int64(12345678901234),
		}
		res, err := EncodeFormFields(data, map[string]FieldEncoding{})

		resDecoded, _ := url.QueryUnescape(res)
		require.NoError(t, err)
		expected := "v1=1&v2=1.2&v3=true&v4=test&v5=123456789&v6=123.456789&v7=12345678901234"
		assert.Equal(t, expected, resDecoded)
	})
}
