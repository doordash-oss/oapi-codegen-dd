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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_extTypeName(t *testing.T) {
	type args struct {
		extPropValue json.RawMessage
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "success",
			args:    args{json.RawMessage(`"uint64"`)},
			want:    "uint64",
			wantErr: false,
		},
		{
			name:    "nil conversion error",
			args:    args{nil},
			want:    "",
			wantErr: true,
		},
		{
			name:    "type conversion error",
			args:    args{json.RawMessage(`12`)},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// kin-openapi no longer returns these as RawMessage
			var extPropValue any
			if tt.args.extPropValue != nil {
				err := json.Unmarshal(tt.args.extPropValue, &extPropValue)
				assert.NoError(t, err)
			}
			got, err := parseString(extPropValue)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_extParsePropGoTypeSkipOptionalPointer(t *testing.T) {
	type args struct {
		extPropValue json.RawMessage
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name:    "success when set to true",
			args:    args{json.RawMessage(`true`)},
			want:    true,
			wantErr: false,
		},
		{
			name:    "success when set to false",
			args:    args{json.RawMessage(`false`)},
			want:    false,
			wantErr: false,
		},
		{
			name:    "nil conversion error",
			args:    args{nil},
			want:    false,
			wantErr: true,
		},
		{
			name:    "type conversion error",
			args:    args{json.RawMessage(`"true"`)},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// kin-openapi no longer returns these as RawMessage
			var extPropValue any
			if tt.args.extPropValue != nil {
				err := json.Unmarshal(tt.args.extPropValue, &extPropValue)
				assert.NoError(t, err)
			}
			got, err := parseBooleanValue(extPropValue)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
