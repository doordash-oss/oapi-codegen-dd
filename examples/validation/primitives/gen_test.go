package gen

import (
	"errors"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
)

func TestResponseValidation(t *testing.T) {
	tests := []struct {
		name    string
		resp    Response
		wantErr bool
	}{
		{
			name: "valid - all fields valid",
			resp: Response{
				Msn1:                     runtime.Ptr("12345"),
				Msn2:                     runtime.Ptr("anything"),
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				Msn3:                     runtime.Ptr(50),
				MsnFloat:                 3.14,
				MsnBool:                  true,
				UserRequired:             User{},
			},
			wantErr: false,
		},
		{
			name: "invalid - msn1 too short",
			resp: Response{
				Msn1:                     runtime.Ptr("123"),
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 1.0,
				MsnBool:                  false,
				UserRequired:             User{},
			},
			wantErr: true,
		},
		{
			name: "invalid - msn1 too long",
			resp: Response{
				Msn1:                     runtime.Ptr("12345678"),
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 1.0,
				MsnBool:                  false,
				UserRequired:             User{},
			},
			wantErr: true,
		},
		{
			name: "invalid - msn3 too small",
			resp: Response{
				Msn3:                     runtime.Ptr(0),
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 1.0,
				MsnBool:                  false,
				UserRequired:             User{},
			},
			wantErr: true,
		},
		{
			name: "invalid - msn3 too large",
			resp: Response{
				Msn3:                     runtime.Ptr(101),
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 1.0,
				MsnBool:                  false,
				UserRequired:             User{},
			},
			wantErr: true,
		},
		{
			name: "invalid - missing required MsnFloat",
			resp: Response{
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnBool:                  true,
				UserRequired:             User{},
			},
			wantErr: true,
		},
		{
			name: "valid - MsnBool false is allowed (booleans can't be required)",
			resp: Response{
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 1.0,
				MsnBool:                  false,
				UserRequired:             User{},
			},
			wantErr: false,
		},
		{
			name: "valid - MsnFloat with zero value fails required check",
			resp: Response{
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 0.0,
				MsnBool:                  true,
				UserRequired:             User{},
			},
			wantErr: true,
		},
		{
			name: "valid - UserRequired with empty struct",
			resp: Response{
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 1.0,
				MsnBool:                  true,
				UserRequired:             User{},
			},
			wantErr: false,
		},
		{
			name: "valid - UserOptional is nil",
			resp: Response{
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 1.0,
				MsnBool:                  true,
				UserRequired:             User{Name: runtime.Ptr("John"), Age: runtime.Ptr(30)},
				UserOptional:             nil,
			},
			wantErr: false,
		},
		{
			name: "valid - UserOptional with values",
			resp: Response{
				MsnReqWithConstraints:    "valid",
				MsnReqWithoutConstraints: "anything",
				MsnFloat:                 1.0,
				MsnBool:                  true,
				UserRequired:             User{Name: runtime.Ptr("John"), Age: runtime.Ptr(30)},
				UserOptional:             &User{Name: runtime.Ptr("Jane"), Age: runtime.Ptr(25)},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resp.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Response.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPredefinedValue_Validate(t *testing.T) {
	tests := []struct {
		name    string
		pv      PredefinedValue
		wantErr bool
	}{
		{
			name: "valid - valid Predefined value",
			pv: PredefinedValue{
				Value: runtime.Ptr(PredefinedA2),
				Type:  runtime.Ptr("test"),
			},
			wantErr: false,
		},
		{
			name: "valid - nil Value",
			pv: PredefinedValue{
				Value: nil,
				Type:  runtime.Ptr("test"),
			},
			wantErr: false,
		},
		{
			name: "invalid - invalid Predefined value",
			pv: PredefinedValue{
				Value: runtime.Ptr(Predefined("INVALID")),
				Type:  runtime.Ptr("test"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pv.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PredefinedValue.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResponseValidation_MultipleErrors(t *testing.T) {
	// Test that all validation errors are collected, not just the first one
	resp := Response{
		Msn1:                     runtime.Ptr(MsnWithConstraints("123")), // Too short (min=4)
		MsnReqWithConstraints:    "12",                                   // Too short (min=4)
		MsnReqWithoutConstraints: "",                                     // Missing required
		Msn3:                     runtime.Ptr(200),                       // Too large (max=100)
		MsnFloat:                 0,                                      // Missing required (zero value)
		UserRequired:             User{},
	}

	err := resp.Validate()
	if err == nil {
		t.Fatal("Expected validation errors, got nil")
	}

	// Check that it's a ValidationErrors (multiple errors)
	var validationErrors runtime.ValidationErrors
	if !errors.As(err, &validationErrors) {
		t.Fatalf("Expected runtime.ValidationErrors, got %T", err)
	}

	// Should have multiple errors (at least 4: Msn1, MsnReqWithConstraints, MsnReqWithoutConstraints, Msn3, MsnFloat)
	if len(validationErrors) < 4 {
		t.Errorf("Expected at least 4 validation errors, got %d: %v", len(validationErrors), validationErrors)
	}

	// Verify error message contains all field names
	errMsg := err.Error()
	expectedFields := []string{"Msn1", "MsnReqWithConstraints", "MsnReqWithoutConstraints", "Msn3"}
	for _, field := range expectedFields {
		if !contains(errMsg, field) {
			t.Errorf("Expected error message to contain field %q, got: %s", field, errMsg)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
