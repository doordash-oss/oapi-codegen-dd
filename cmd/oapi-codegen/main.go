// Copyright 2025 DoorDash, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/codegen"
	"go.yaml.in/yaml/v4"
)

const (
	// File and directory permissions for generated code
	generatedDirPerm  = 0755
	generatedFilePerm = 0644
)

var (
	flagConfigFile string
	flagPrintUsage bool
)

func main() {
	flag.StringVar(&flagConfigFile, "config", "", "A YAML config file that controls oapi-codegen behavior.")
	flag.BoolVar(&flagPrintUsage, "help", false, "Show this help and exit.")

	flag.Parse()

	if flagPrintUsage {
		flag.Usage()
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		errExit("Please specify a path to a OpenAPI spec file")
	} else if flag.NArg() > 1 {
		errExit("Only one OpenAPI spec file is accepted and it must be the last CLI argument")
	}

	// Read the spec file (supports both local files and URLs)
	specPath := flag.Arg(0)
	specContents, err := readSpec(specPath)
	if err != nil {
		errExit("Error reading spec: %v", err)
	}

	// Read the config file
	cfg := codegen.Configuration{}
	hasConfigFile := flagConfigFile != ""
	if hasConfigFile {
		// #nosec G304 -- CLI tool intentionally reads user-specified config files
		cfgContents, err := os.ReadFile(flagConfigFile)
		if err != nil {
			errExit("Error reading config file: %v", err)
		}

		err = yaml.Unmarshal(cfgContents, &cfg)
		if err != nil {
			errExit("Error parsing config file: %v", err)
		}
	}

	cfg = cfg.WithDefaults()

	// If no config file was provided and input is a URL, output to stdout
	// For local files without config, keep default behavior (write to gen.go)
	if !hasConfigFile && (strings.HasPrefix(specPath, "http://") || strings.HasPrefix(specPath, "https://")) {
		cfg.Output = nil
	}

	code, err := codegen.Generate(specContents, cfg)
	if err != nil {
		errExit("Error generating code: %v", err)
	}

	destDir := ""
	destFile := ""
	if cfg.Output != nil {
		destDir = cfg.Output.Directory
		if destDir != "" {
			err = os.MkdirAll(destDir, generatedDirPerm)
			if err != nil {
				errExit("Error creating directory: %v", err)
			}
		}
		if cfg.Output.UseSingleFile {
			destFile = filepath.Join(destDir, cfg.Output.Filename)
		} else {
			destDir = filepath.Join(destDir, cfg.PackageName)
			err = os.MkdirAll(destDir, generatedDirPerm)
			if err != nil {
				errExit("Error creating directory: %v", err)
			}
		}
	}

	if destFile == "" && destDir == "" {
		fmt.Print(code.GetCombined())
		return
	}

	if destFile != "" {
		if err = os.WriteFile(destFile, []byte(code.GetCombined()), generatedFilePerm); err != nil {
			errExit("Error writing file: %v", err)
		}
	}

	for name, contents := range code {
		isScaffold := codegen.IsScaffoldFile(name)
		actualName := name
		if isScaffold {
			actualName = codegen.ScaffoldFileName(name)
		}

		// Skip "all" key (combined output for single-file mode)
		if name == "all" {
			continue
		}

		// In single-file mode, only write scaffold files
		if destFile != "" && !isScaffold {
			continue
		}

		// Determine file path
		var filePath string
		if strings.Contains(actualName, "/") {
			// Files with "/" have their full path already (e.g., "server/main")
			filePath = actualName + ".go"
		} else if destDir != "" {
			filePath = filepath.Join(destDir, actualName+".go")
		} else {
			filePath = filepath.Join(filepath.Dir(destFile), actualName+".go")
		}

		// Skip scaffold files if they exist and overwrite is not set
		scaffoldOverwrite := cfg.Generate != nil && cfg.Generate.Handler != nil &&
			cfg.Generate.Handler.Output != nil && cfg.Generate.Handler.Output.Overwrite
		if isScaffold && !scaffoldOverwrite {
			if _, err := os.Stat(filePath); err == nil {
				continue
			}
		}

		if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
			errExit("Error creating directory: %v", err)
		}
		if err = os.WriteFile(filePath, []byte(contents), generatedFilePerm); err != nil {
			errExit("Error writing file: %v", err)
		}
	}
}

func errExit(msg string, args ...any) {
	msg = msg + "\n"
	_, _ = fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}

// readSpec reads an OpenAPI spec from a file path or URL
func readSpec(path string) ([]byte, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return fetchURL(path)
	}
	// #nosec G304 -- CLI tool intentionally reads user-specified OpenAPI spec files
	return os.ReadFile(path)
}

// fetchURL fetches content from a URL
func fetchURL(url string) ([]byte, error) {
	// #nosec G107 -- CLI tool intentionally fetches user-specified URLs
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}
