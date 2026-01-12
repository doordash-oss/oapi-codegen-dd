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
	"os"
	"path/filepath"

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

	// Read the spec file
	filePath := flag.Arg(0)
	// #nosec G304 -- CLI tool intentionally reads user-specified OpenAPI spec files
	specContents, err := os.ReadFile(filePath)
	if err != nil {
		errExit("Error reading file: %v", err)
	}

	// Read the config file
	cfg := codegen.Configuration{}
	if flagConfigFile != "" {
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

	if destFile != "" {
		err = os.WriteFile(destFile, []byte(code.GetCombined()), generatedFilePerm)
		if err != nil {
			errExit("Error writing file: %v", err)
		}
	} else if destDir != "" {
		for name, contents := range code {
			err = os.WriteFile(filepath.Join(destDir, name+".go"), []byte(contents), generatedFilePerm)
			if err != nil {
				errExit("Error writing file: %v", err)
			}
		}
	} else {
		println(code.GetCombined())
	}
}

func errExit(msg string, args ...any) {
	msg = msg + "\n"
	_, _ = fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}
