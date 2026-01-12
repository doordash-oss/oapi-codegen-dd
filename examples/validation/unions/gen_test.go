package gen

import (
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
)

func TestUnionValidation_StringWithMinLength(t *testing.T) {
	tests := []struct {
		name    string
		union   Response_User_OneOf
		wantErr bool
	}{
		{
			name: "valid - User object",
			union: Response_User_OneOf{
				Either: runtime.Either[User, string]{
					A: User{Name: ptrString("John"), Age: ptrInt(30)},
					N: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "valid - string with minLength 3",
			union: Response_User_OneOf{
				Either: runtime.Either[User, string]{
					B: "abc",
					N: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - string too short (minLength 3)",
			union: Response_User_OneOf{
				Either: runtime.Either[User, string]{
					B: "ab", // Only 2 characters, but minLength is 3
					N: 2,
				},
			},
			wantErr: true, // Should fail validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.union.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Response_User_OneOf.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnionValidation_IntegerWithMinimum(t *testing.T) {
	tests := []struct {
		name        string
		union       Response_Friend_AnyOf
		tryAsUser   bool
		tryAsString bool
		tryAsInt    bool
		wantErr     bool
	}{
		{
			name: "valid - User object",
			union: Response_Friend_AnyOf{
				union: []byte(`{"name":"John","age":30}`),
			},
			tryAsUser: true,
			wantErr:   false,
		},
		{
			name: "valid - string with minLength 3",
			union: Response_Friend_AnyOf{
				union: []byte(`"abc"`),
			},
			tryAsString: true,
			wantErr:     false,
		},
		{
			name: "invalid - string too short (minLength 3)",
			union: Response_Friend_AnyOf{
				union: []byte(`"ab"`), // Only 2 characters, but minLength is 3
			},
			tryAsString: true,
			wantErr:     true, // Should fail validation
		},
		{
			name: "valid - integer >= 1",
			union: Response_Friend_AnyOf{
				union: []byte(`5`),
			},
			tryAsInt: true,
			wantErr:  false,
		},
		{
			name: "invalid - integer < 1 (minimum is 1)",
			union: Response_Friend_AnyOf{
				union: []byte(`0`), // Less than minimum of 1
			},
			tryAsInt: true,
			wantErr:  true, // Should fail validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For multi-element unions, we need to use AsValidated<Type>() methods
			var err error
			if tt.tryAsUser {
				_, err = tt.union.AsValidatedUser()
			} else if tt.tryAsString {
				_, err = tt.union.AsValidatedString()
			} else if tt.tryAsInt {
				_, err = tt.union.AsValidatedInt()
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("Response_Friend_AnyOf.AsValidated*() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func ptrString(s string) *string {
	return &s
}

func ptrInt(i int) *int {
	return &i
}
