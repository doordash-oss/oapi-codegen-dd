package gen

import (
	"testing"
)

func TestOrderDirection_TypeMismatch(t *testing.T) {
	// Test that OrderDirection (declared as integer but with string values)
	// is correctly generated as a string type
	tests := []struct {
		name    string
		value   OrderDirection
		wantErr bool
	}{
		{
			name:    "valid asc",
			value:   OrderDirectionAsc,
			wantErr: false,
		},
		{
			name:    "valid desc",
			value:   OrderDirectionDesc,
			wantErr: false,
		},
		{
			name:    "invalid value",
			value:   OrderDirection("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.value.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("OrderDirection.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPriority_TypeMismatch(t *testing.T) {
	// Test that Priority (declared as number but with string values)
	// is correctly generated as a string type
	tests := []struct {
		name    string
		value   Priority
		wantErr bool
	}{
		{
			name:    "valid low",
			value:   PriorityLow,
			wantErr: false,
		},
		{
			name:    "valid medium",
			value:   PriorityMedium,
			wantErr: false,
		},
		{
			name:    "valid high",
			value:   PriorityHigh,
			wantErr: false,
		},
		{
			name:    "invalid value",
			value:   Priority("critical"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.value.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Priority.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStatusCode_CorrectType(t *testing.T) {
	// Test that StatusCode (correctly declared as integer with numeric values)
	// is generated as an int type
	tests := []struct {
		name    string
		value   StatusCode
		wantErr bool
	}{
		{
			name:    "valid 200",
			value:   StatusCodeN200,
			wantErr: false,
		},
		{
			name:    "valid 404",
			value:   StatusCodeN404,
			wantErr: false,
		},
		{
			name:    "valid 500",
			value:   StatusCodeN500,
			wantErr: false,
		},
		{
			name:    "invalid 201",
			value:   StatusCode(201),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.value.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("StatusCode.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
