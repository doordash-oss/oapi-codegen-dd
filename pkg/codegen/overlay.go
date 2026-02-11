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
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/pb33f/libopenapi"
)

// applyOverlays applies a list of overlay files to the document in order.
// Each source can be a file path or URL.
func applyOverlays(doc libopenapi.Document, sources []string) (libopenapi.Document, error) {
	for _, source := range sources {
		overlayBytes, err := loadOverlaySource(source)
		if err != nil {
			return nil, fmt.Errorf("error loading overlay %q: %w", source, err)
		}

		result, err := libopenapi.ApplyOverlayFromBytes(doc, overlayBytes)
		if err != nil {
			return nil, fmt.Errorf("error applying overlay %q: %w", source, err)
		}

		doc = result.OverlayDocument
	}

	return doc, nil
}

// loadOverlaySource loads overlay content from a file path or URL.
func loadOverlaySource(source string) ([]byte, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return loadOverlayFromURL(source)
	}
	// #nosec G304 -- Overlay paths are user-specified in config, same as OpenAPI spec paths
	return os.ReadFile(source)
}

// loadOverlayFromURL fetches overlay content from a URL.
func loadOverlayFromURL(url string) ([]byte, error) {
	// #nosec G107 -- Overlay URLs are user-specified in config, same as OpenAPI spec URLs
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error fetching URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
