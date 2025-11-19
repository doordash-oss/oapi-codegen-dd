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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDate_MarshalJSON(t *testing.T) {
	testDate := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)
	b := struct {
		DateField Date `json:"date"`
	}{
		DateField: Date{testDate},
	}
	jsonBytes, err := json.Marshal(b)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"date":"2025-04-01"}`, string(jsonBytes))
}

func TestDate_UnmarshalJSON(t *testing.T) {
	testDate := time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC)
	jsonStr := `{"date":"2019-04-01"}`
	b := struct {
		DateField Date `json:"date"`
	}{}
	err := json.Unmarshal([]byte(jsonStr), &b)
	assert.NoError(t, err)
	assert.Equal(t, testDate, b.DateField.Time)
}

func TestDate_Stringer(t *testing.T) {
	t.Run("nil date", func(t *testing.T) {
		var d *Date
		assert.Equal(t, "<nil>", fmt.Sprintf("%v", d))
	})

	t.Run("ptr date", func(t *testing.T) {
		d := &Date{
			Time: time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC),
		}
		assert.Equal(t, "2019-04-01", fmt.Sprintf("%v", d))
	})

	t.Run("value date", func(t *testing.T) {
		d := Date{
			Time: time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC),
		}
		assert.Equal(t, "2019-04-01", fmt.Sprintf("%v", d))
	})
}

func TestDate_UnmarshalText(t *testing.T) {
	testDate := time.Date(2022, 6, 14, 0, 0, 0, 0, time.UTC)
	value := []byte("2022-06-14")

	date := Date{}
	err := date.UnmarshalText(value)

	assert.NoError(t, err)
	assert.Equal(t, testDate, date.Time)
}
