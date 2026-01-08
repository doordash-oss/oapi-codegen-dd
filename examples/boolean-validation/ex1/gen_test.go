package gen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUser_Validate_BooleanFields(t *testing.T) {
	t.Run("OK cases - should pass validation", func(t *testing.T) {
		okCases := []struct {
			name string
			user User
		}{
			{
				name: "active=true with required name",
				user: User{
					Name:   "John Doe",
					Active: true,
				},
			},
			{
				name: "active=false with required name (KEY TEST: false is valid)",
				user: User{
					Name:   "Jane Doe",
					Active: false,
				},
			},
			{
				name: "optional verified=nil",
				user: User{
					Name:     "Bob Smith",
					Active:   true,
					Verified: nil,
				},
			},
			{
				name: "optional verified=false",
				user: User{
					Name:     "Alice Johnson",
					Active:   true,
					Verified: boolPtr(false),
				},
			},
			{
				name: "optional verified=true",
				user: User{
					Name:     "Charlie Brown",
					Active:   false,
					Verified: boolPtr(true),
				},
			},
			{
				name: "all booleans false",
				user: User{
					Name:     "Test User",
					Active:   false,
					Verified: boolPtr(false),
				},
			},
			{
				name: "all booleans true",
				user: User{
					Name:     "Test User",
					Active:   true,
					Verified: boolPtr(true),
				},
			},
		}

		for _, tc := range okCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.user.Validate()
				require.NoError(t, err, "validation should PASS")
			})
		}
	})

	t.Run("FAIL cases - should fail validation", func(t *testing.T) {
		failCases := []struct {
			name          string
			user          User
			expectedError string
		}{
			{
				name: "missing required name (empty string)",
				user: User{
					Name:   "",
					Active: true,
				},
				expectedError: "Name",
			},
			{
				name: "missing required name with active=false",
				user: User{
					Name:   "",
					Active: false,
				},
				expectedError: "Name",
			},
		}

		for _, tc := range failCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.user.Validate()
				require.Error(t, err, "validation should FAIL")
				assert.Contains(t, err.Error(), tc.expectedError)
			})
		}
	})
}

func boolPtr(b bool) *bool {
	return &b
}

func TestPostUsersResponse_Validate_WithResponseValidators(t *testing.T) {
	t.Run("OK cases - response validation passes", func(t *testing.T) {
		okCases := []struct {
			name     string
			response PostUsersResponse
		}{
			{
				name: "valid response with active=true",
				response: PostUsersResponse{
					Name:   "John Doe",
					Active: true,
				},
			},
			{
				name: "valid response with active=false (KEY TEST)",
				response: PostUsersResponse{
					Name:   "Jane Doe",
					Active: false,
				},
			},
			{
				name: "valid response with all fields",
				response: PostUsersResponse{
					Name:     "Bob Smith",
					Active:   true,
					Verified: boolPtr(true),
				},
			},
		}

		for _, tc := range okCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.response.Validate()
				require.NoError(t, err, "response validation should PASS")
			})
		}
	})

	t.Run("FAIL cases - response validation fails", func(t *testing.T) {
		failCases := []struct {
			name          string
			response      PostUsersResponse
			expectedError string
		}{
			{
				name: "missing required name",
				response: PostUsersResponse{
					Name:   "",
					Active: true,
				},
				expectedError: "Name",
			},
		}

		for _, tc := range failCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.response.Validate()
				require.Error(t, err, "response validation should FAIL")
				assert.Contains(t, err.Error(), tc.expectedError)
			})
		}
	})
}
