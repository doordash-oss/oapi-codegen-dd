package union

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollaboration_AllOfWithMetadata(t *testing.T) {
	// Test that the metadata-only allOf element doesn't generate a separate type
	// The item field should only have Collaboration_Item_AllOf0, not AllOf1

	t.Run("marshal collaboration with file item", func(t *testing.T) {
		file := File{
			Type: FileTypeFile,
			ID:   "123",
			Name: ptr("document.pdf"),
		}

		itemOneOf := &Collaboration_Item_AllOf0_OneOf{}
		err := itemOneOf.FromFile(file)
		require.NoError(t, err)

		item := &Collaboration_Item1{
			Collaboration_Item_AllOf0: &Collaboration_Item_AllOf0{
				Collaboration_Item_AllOf0_OneOf: itemOneOf,
			},
		}

		collab := Collaboration{
			ID:   "collab-456",
			Item: item,
			Role: ptr(CollaborationRoleEditor),
		}

		data, err := json.Marshal(collab)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "collab-456", result["id"])
		assert.Equal(t, "editor", result["role"])

		itemData := result["item"].(map[string]interface{})
		assert.Equal(t, "file", itemData["type"])
		assert.Equal(t, "123", itemData["id"])
		assert.Equal(t, "document.pdf", itemData["name"])
	})

	t.Run("unmarshal collaboration with folder item", func(t *testing.T) {
		jsonData := `{
			"id": "collab-789",
			"item": {
				"type": "folder",
				"id": "456",
				"name": "My Folder"
			},
			"role": "viewer"
		}`

		var collab Collaboration
		err := json.Unmarshal([]byte(jsonData), &collab)
		require.NoError(t, err)

		assert.Equal(t, "collab-789", collab.ID)
		assert.NotNil(t, collab.Item)
		assert.NotNil(t, collab.Item.Collaboration_Item_AllOf0)
		assert.NotNil(t, collab.Item.Collaboration_Item_AllOf0.Collaboration_Item_AllOf0_OneOf)

		folder, err := collab.Item.Collaboration_Item_AllOf0.Collaboration_Item_AllOf0_OneOf.AsFolder()
		require.NoError(t, err)
		assert.Equal(t, FolderTypeFolder, folder.Type)
		assert.Equal(t, "456", folder.ID)
		assert.Equal(t, "My Folder", *folder.Name)

		assert.Equal(t, CollaborationRoleViewer, *collab.Role)
	})

	t.Run("unmarshal collaboration with web_link item", func(t *testing.T) {
		jsonData := `{
			"id": "collab-999",
			"item": {
				"type": "web_link",
				"id": "789",
				"url": "https://example.com"
			},
			"role": "owner"
		}`

		var collab Collaboration
		err := json.Unmarshal([]byte(jsonData), &collab)
		require.NoError(t, err)

		assert.Equal(t, "collab-999", collab.ID)

		webLink, err := collab.Item.Collaboration_Item_AllOf0.Collaboration_Item_AllOf0_OneOf.AsWebLink()
		require.NoError(t, err)
		assert.Equal(t, WebLinkTypeWebLink, webLink.Type)
		assert.Equal(t, "789", webLink.ID)
		assert.Equal(t, "https://example.com", *webLink.URL)
	})

	t.Run("collaboration with null item", func(t *testing.T) {
		jsonData := `{
			"id": "collab-pending",
			"item": null,
			"role": "editor"
		}`

		var collab Collaboration
		err := json.Unmarshal([]byte(jsonData), &collab)
		require.NoError(t, err)

		assert.Equal(t, "collab-pending", collab.ID)
		assert.Nil(t, collab.Item)
	})
}

func ptr[T any](v T) *T {
	return &v
}
