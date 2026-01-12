package example1_test

import (
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/examples/client/example1/example1"
	"github.com/stretchr/testify/assert"
)

func TestEnumPrefixes(t *testing.T) {
	// Verify that enum constants have the correct prefixes
	// This ensures that the always-prefix-enums default behavior is preserved

	// Component schema enum should have type name prefix
	assert.Equal(t, "company", string(example1.ClientTypeTypeCompany))
	assert.Equal(t, "individual", string(example1.ClientTypeTypeIndividual))

	// Response schema enum should have response type name prefix
	assert.Equal(t, "company", string(example1.GetClientResponseTypeCompany))
	assert.Equal(t, "individual", string(example1.GetClientResponseTypeIndividual))
}

func TestErrorResponseErrorMethod(t *testing.T) {
	// Verify that error responses have Error() method
	// This ensures responses are not aliases but actual structs

	t.Run("GetClientErrorResponse", func(t *testing.T) {
		msg := "test error message"
		errResp := example1.GetClientErrorResponse{
			Message: &msg,
		}

		// Should have Error() method
		assert.Equal(t, "test error message", errResp.Error())
	})

	t.Run("GetClientErrorResponse_NilMessage", func(t *testing.T) {
		errResp := example1.GetClientErrorResponse{}

		// Should return default error when message is nil
		assert.Equal(t, "unknown error", errResp.Error())
	})

	t.Run("UpdateClientErrorResponseJSON", func(t *testing.T) {
		code := example1.ErrorCode("ERR_123")
		errResp := example1.UpdateClientErrorResponseJSON{
			Code: &code,
		}

		// Should have Error() method
		assert.Equal(t, "ERR_123", errResp.Error())
	})

	t.Run("UpdateClientErrorResponseJSON_NilCode", func(t *testing.T) {
		errResp := example1.UpdateClientErrorResponseJSON{}

		// Should return default error when code is nil
		assert.Equal(t, "unknown error", errResp.Error())
	})
}

func TestResponsesAreNotAliases(t *testing.T) {
	// Verify that responses are actual structs, not aliases
	// This is important because aliases don't support methods like Error()

	// GetClientResponse should be a struct with fields
	resp := example1.GetClientResponse{
		Name: "Test Client",
	}
	assert.Equal(t, "Test Client", resp.Name)

	// Error responses should be structs with fields
	msg := "error"
	errResp := example1.GetClientErrorResponse{
		Message: &msg,
	}
	assert.Equal(t, &msg, errResp.Message)
}
