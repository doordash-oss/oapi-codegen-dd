package deeppathref

import (
	"testing"
	"time"
)

// TestDeepPathReferences verifies that deep path references work correctly
func TestDeepPathReferences(t *testing.T) {
	// Test that the timestamp types are properly aliased
	var createdAt Users_Get_Response200_JSON_Properties_CreatedAt
	var updatedAt Users_Get_Response200_JSON_Properties_UpdatedAt

	// These should be time.Time under the hood
	now := time.Now()
	createdAt = now
	updatedAt = now

	// Test that GetPostResponse uses the referenced types
	post := GetPostResponse{
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}

	if post.CreatedAt == nil {
		t.Error("Expected CreatedAt to be set")
	}
	if post.UpdatedAt == nil {
		t.Error("Expected UpdatedAt to be set")
	}

	// Test that ListComments also uses the same referenced types
	comment := ListComments_Response_Item{
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}

	if comment.CreatedAt == nil {
		t.Error("Expected CreatedAt to be set")
	}
	if comment.UpdatedAt == nil {
		t.Error("Expected UpdatedAt to be set")
	}
}

// TestTypeReuse verifies that the same type is reused across different endpoints
func TestTypeReuse(t *testing.T) {
	// The key benefit of deep path references is type reuse
	// Multiple endpoints reference the same inline schema, so they share the same type

	timestamp := time.Now()

	// This timestamp can be used in GetPost
	post := GetPostResponse{
		CreatedAt: &timestamp,
	}

	// And also in ListComments
	comment := ListComments_Response_Item{
		CreatedAt: &timestamp,
	}

	// Both should have the same value
	if post.CreatedAt != comment.CreatedAt {
		t.Error("Expected both to reference the same timestamp")
	}
}
