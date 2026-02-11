// Copyright 2025 DoorDash, Inc.
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
	"strings"
	"unicode"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"go.yaml.in/yaml/v4"
)

func CreateDocument(docContents []byte, cfg Configuration) (libopenapi.Document, error) {
	doc, err := LoadDocumentFromContents(docContents)
	if err != nil {
		return nil, err
	}

	// Apply overlays before filtering and pruning
	if cfg.Overlay != nil && len(cfg.Overlay.Sources) > 0 {
		doc, err = applyOverlays(doc, cfg.Overlay.Sources)
		if err != nil {
			return nil, fmt.Errorf("error applying overlays: %w", err)
		}
	}

	if _, err = doc.BuildV3Model(); err != nil {
		return nil, fmt.Errorf("error building model: %w", err)
	}

	var filtered bool
	model, filtered, err := filterOutDocument(doc, cfg.Filter)
	if err != nil {
		return nil, fmt.Errorf("error filtering document: %w", err)
	}

	// If we filtered anything, we must prune to remove dangling references
	// Otherwise, only prune if SkipPrune is false
	if filtered || !cfg.SkipPrune {
		if err = pruneSchema(model); err != nil {
			return nil, fmt.Errorf("error pruning schema: %w", err)
		}
		return doc, nil
	}

	return doc, nil
}

func LoadDocumentFromContents(contents []byte) (libopenapi.Document, error) {
	docConfig := &datamodel.DocumentConfiguration{
		SkipCircularReferenceCheck: true,
	}
	doc, err := libopenapi.NewDocumentWithConfiguration(contents, docConfig)
	if err != nil {
		return fixDocument(contents, err, docConfig)
	}
	return doc, nil
}

func fixDocument(contents []byte, originalErr error, docConfig *datamodel.DocumentConfiguration) (libopenapi.Document, error) {
	if !strings.Contains(originalErr.Error(), "unable to parse specification") {
		return nil, originalErr
	}

	// Strip control characters if the error indicates they're present
	processedContents := contents

	// Check if it's a fixable error (control character error)
	if strings.Contains(originalErr.Error(), "control characters are not allowed") {
		text := string(contents)
		cleaned := strings.Map(func(r rune) rune {
			// Keep printable characters, tabs, newlines, and carriage returns
			if unicode.IsPrint(r) || r == '\t' || r == '\n' || r == '\r' {
				return r
			}
			// Remove control characters
			return -1
		}, text)
		processedContents = []byte(cleaned)
	}

	// Unmarshal into a generic map
	var doc map[string]any
	if err := yaml.Unmarshal(processedContents, &doc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal document: %w", err)
	}

	// Extract existing title and version from info section if available
	title := "API"
	version := "1.0.0"

	if info, ok := doc["info"].(map[string]any); ok {
		if t, ok := info["title"].(string); ok && t != "" {
			title = t
		}
		if v, ok := info["version"].(string); ok && v != "" {
			version = v
		}
	}

	// Replace info section with minimal required fields using existing values
	doc["info"] = map[string]any{
		"title":   title,
		"version": version,
	}

	// Marshal back to YAML
	fixed, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document: %w", err)
	}

	// Retry with fixed content
	result, err := libopenapi.NewDocumentWithConfiguration(fixed, docConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating document after fix: %w", err)
	}

	return result, nil
}
