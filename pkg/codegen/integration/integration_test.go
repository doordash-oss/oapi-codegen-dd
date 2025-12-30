// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

//go:build integration
// +build integration

package integration

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/doordash/oapi-codegen-dd/v3/pkg/codegen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/specs
var specsFS embed.FS

func TestIntegration(t *testing.T) {
	specPath := os.Getenv("SPEC")

	// Collect specs to process
	specs := collectSpecs(t, specPath)
	if len(specs) == 0 {
		log.Println("No specs to process, skipping integration test")
		return
	}

	log.Printf("Found %d spec(s) to process\n", len(specs))

	cfg := codegen.Configuration{
		PackageName: "integration",
		Generate: &codegen.GenerateOptions{
			Client: true,
		},
		Client: &codegen.Client{
			Name: "IntegrationClient",
		},
	}
	cfg = cfg.Merge(codegen.NewDefaultConfiguration())

	for _, name := range specs {
		t.Run(fmt.Sprintf("test-%s", name), func(t *testing.T) {
			t.Parallel()

			contents, err := getFileContents(name)
			if err != nil {
				t.Fatalf("failed to download file: %s", err)
			}

			fmt.Printf("[%s] Generating code\n", name)
			res, err := codegen.Generate(contents, cfg)
			require.NoError(t, err, "failed to generate code")
			require.NotNil(t, res, "result should not be nil")

			assert.NotNil(t, res["package integration"])
			assert.NotNil(t, res["type IntegrationClient struct {"])
			assert.NotNil(t, res["RequestOptions struct {"])
		})
	}
}

func getFileContents(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return contents, nil
}

func collectSpecs(t *testing.T, specPath string) []string {
	var specs []string

	if specPath != "" {
		specs = append(specs, specPath)
		return specs
	}

	// Walk through testdata/specs
	err := fs.WalkDir(specsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		fileName := d.Name()
		if fileName[0] == '-' || strings.Contains(path, "/stash/") {
			return nil
		}

		if strings.HasSuffix(fileName, ".yml") || strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".json") {
			specs = append(specs, path)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk specs directory: %v", err)
	}

	return specs
}
