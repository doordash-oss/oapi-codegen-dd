// Copyright 2026 DoorDash, Inc.
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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOverlaySource_File(t *testing.T) {
	data, err := loadOverlaySource("testdata/overlay-add-extensions.yml")
	require.NoError(t, err)
	assert.Contains(t, string(data), "overlay: 1.0.0")
	assert.Contains(t, string(data), "x-go-name: UserModel")
}

func TestLoadOverlaySource_FileNotFound(t *testing.T) {
	_, err := loadOverlaySource("testdata/nonexistent.yml")
	require.Error(t, err)
}

func TestLoadOverlaySource_URL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("overlay: 1.0.0\ninfo:\n  title: Test\n  version: 1.0.0\nactions: []"))
	}))
	defer server.Close()

	data, err := loadOverlaySource(server.URL)
	require.NoError(t, err)
	assert.Contains(t, string(data), "overlay: 1.0.0")
}

func TestLoadOverlayFromURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test content"))
	}))
	defer server.Close()

	data, err := loadOverlayFromURL(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "test content", string(data))
}

func TestLoadOverlayFromURL_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := loadOverlayFromURL(server.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code: 404")
}

func TestLoadOverlayFromURL_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := loadOverlayFromURL(server.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code: 500")
}

func TestLoadOverlayFromURL_InvalidURL(t *testing.T) {
	_, err := loadOverlayFromURL("http://localhost:99999/nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error fetching URL")
}

func TestApplyOverlays_EmptySources(t *testing.T) {
	doc, err := LoadDocumentFromContents([]byte(readTestdata(t, "overlay-base.yml")))
	require.NoError(t, err)

	result, err := applyOverlays(doc, []string{})
	require.NoError(t, err)
	assert.Equal(t, doc, result)
}

func TestApplyOverlays_InvalidOverlayContent(t *testing.T) {
	doc, err := LoadDocumentFromContents([]byte(readTestdata(t, "overlay-base.yml")))
	require.NoError(t, err)

	// Create a temp file with invalid overlay content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid overlay yaml: [[["))
	}))
	defer server.Close()

	_, err = applyOverlays(doc, []string{server.URL})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error applying overlay")
}

func TestApplyOverlays_SingleSource(t *testing.T) {
	doc, err := LoadDocumentFromContents([]byte(readTestdata(t, "overlay-base.yml")))
	require.NoError(t, err)

	result, err := applyOverlays(doc, []string{"testdata/overlay-add-extensions.yml"})
	require.NoError(t, err)

	// Verify the overlay was applied by checking the rendered document
	model, err := result.BuildV3Model()
	require.NoError(t, err)

	userSchema, ok := model.Model.Components.Schemas.Get("User")
	require.True(t, ok)

	// Check x-go-name extension was added
	goName, ok := userSchema.Schema().Extensions.Get("x-go-name")
	require.True(t, ok)
	assert.Equal(t, "UserModel", goName.Value)
}
