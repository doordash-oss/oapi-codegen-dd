package xsensitivedata

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserMarshalJSON_SensitiveData(t *testing.T) {
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

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Failed to marshal user: %v", err)
	}

	jsonStr := string(data)
	t.Logf("Marshaled JSON: %s", jsonStr)

	// Verify that sensitive data is masked
	if strings.Contains(jsonStr, "user@example.com") {
		t.Error("Email should be masked but found in JSON")
	}

	if strings.Contains(jsonStr, "123-45-6789") {
		t.Error("SSN should be masked but found in JSON")
	}

	if strings.Contains(jsonStr, "1234-5678-9012-3456") {
		t.Error("Credit card should be partially masked but found in JSON")
	}

	// Verify credit card shows last 4 digits
	if !strings.Contains(jsonStr, "3456") {
		t.Error("Credit card should show last 4 digits")
	}

	if strings.Contains(jsonStr, "my-secret-api-key") {
		t.Error("API key should be hashed but found in JSON")
	}

	// Verify that non-sensitive data is present
	if !strings.Contains(jsonStr, `"id":1`) {
		t.Error("ID should be present in JSON")
	}

	if !strings.Contains(jsonStr, `"username":"testuser"`) {
		t.Error("Username should be present in JSON")
	}

	// Verify that email is masked (should be fixed-length asterisks)
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Use the same constant as the runtime for consistency
	expectedMask := "********" // runtime.maskReplacement is not exported

	if email, ok := result["email"].(string); ok {
		if email != expectedMask {
			t.Errorf("Email should be masked as '%s', got: %s", expectedMask, email)
		}
	}

	// Verify that API key is hashed (should be a hex string)
	if apiKey, ok := result["apiKey"].(string); ok {
		if len(apiKey) != 64 { // SHA256 produces 64 hex characters
			t.Errorf("API key should be a SHA256 hash (64 chars), got length: %d", len(apiKey))
		}
	}
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
	if err := json.Unmarshal([]byte(jsonStr), &user); err != nil {
		t.Fatalf("Failed to unmarshal user: %v", err)
	}

	if user.ID != 1 {
		t.Errorf("Expected ID 1, got %d", user.ID)
	}

	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", user.Username)
	}

	if user.Email == nil || *user.Email != "user@example.com" {
		t.Errorf("Expected email 'user@example.com', got %v", user.Email)
	}
}
