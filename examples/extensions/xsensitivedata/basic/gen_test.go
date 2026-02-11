package xsensitivedata

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserMarshalJSON_RawData(t *testing.T) {
	email := "user@example.com"
	ssn := "123-45-6789"
	creditCard := "1234-5678-9012-3456"
	apiKey := "my-secret-api-key"

	user := User{
		ID:         1,
		Username:   "testuser",
		Email:      &email,
		Ssn:        &ssn,
		CreditCard: &creditCard,
		APIKey:     &apiKey,
	}

	// json.Marshal returns raw data (for API calls)
	data, err := json.Marshal(user)
	require.NoError(t, err)

	jsonStr := string(data)

	// Verify that sensitive data is NOT masked in raw JSON
	assert.Contains(t, jsonStr, "user@example.com", "Email should be present in raw JSON")
	assert.Contains(t, jsonStr, "123-45-6789", "SSN should be present in raw JSON")
	assert.Contains(t, jsonStr, "1234-5678-9012-3456", "Credit card should be present in raw JSON")
	assert.Contains(t, jsonStr, "my-secret-api-key", "API key should be present in raw JSON")
}

func TestUserMasked_SensitiveData(t *testing.T) {
	email := "user@example.com"
	ssn := "123-45-6789"
	creditCard := "1234-5678-9012-3456"
	apiKey := "my-secret-api-key"

	user := User{
		ID:         1,
		Username:   "testuser",
		Email:      &email,
		Ssn:        &ssn,
		CreditCard: &creditCard,
		APIKey:     &apiKey,
	}

	// json.Marshal(user.Masked()) returns masked data
	data, err := json.Marshal(user.Masked())
	require.NoError(t, err)

	jsonStr := string(data)

	// Verify that sensitive data is masked
	assert.NotContains(t, jsonStr, "user@example.com", "Email should be masked")
	assert.NotContains(t, jsonStr, "123-45-6789", "SSN should be masked")
	assert.NotContains(t, jsonStr, "1234-5678-9012-3456", "Credit card should be partially masked")
	assert.NotContains(t, jsonStr, "my-secret-api-key", "API key should be hashed")

	// Verify credit card shows last 4 digits
	assert.Contains(t, jsonStr, "3456", "Credit card should show last 4 digits")

	// Verify that non-sensitive data is present
	assert.Contains(t, jsonStr, `"id":1`, "ID should be present")
	assert.Contains(t, jsonStr, `"username":"testuser"`, "Username should be present")

	// Verify that email is masked (should be fixed-length asterisks)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &result))

	expectedMask := "********"
	assert.Equal(t, expectedMask, result["email"], "Email should be masked as asterisks")

	// Verify that API key is hashed (should be a hex string)
	apiKeyVal, ok := result["apiKey"].(string)
	require.True(t, ok, "apiKey should be a string")
	assert.Len(t, apiKeyVal, 64, "API key should be a SHA256 hash (64 chars)")
}

func TestUserUnmarshalJSON(t *testing.T) {
	jsonStr := `{
		"id": 1,
		"username": "testuser",
		"email": "user@example.com",
		"ssn": "123-45-6789",
		"creditCard": "1234-5678-9012-3456",
		"apiKey": "my-secret-api-key"
	}`

	var user User
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &user))

	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "testuser", user.Username)
	require.NotNil(t, user.Email)
	assert.Equal(t, "user@example.com", *user.Email)
}
