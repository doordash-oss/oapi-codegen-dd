package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/doordash/oapi-codegen/v3/pkg/codegen"
	"gopkg.in/yaml.v3"
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
	specContents, err := os.ReadFile(filePath)
	if err != nil {
		errExit("Error reading file: %v", err)
	}

	// Read the config file
	cfg := codegen.Configuration{}
	if flagConfigFile != "" {
		cfgContents, err := os.ReadFile(flagConfigFile)
		if err != nil {
			errExit("Error reading config file: %v", err)
		}

		err = yaml.Unmarshal(cfgContents, &cfg)
		if err != nil {
			errExit("Error parsing config file: %v", err)
		}
	}

	cfg = cfg.Merge(codegen.NewDefaultConfiguration())

	code, err := codegen.Generate(specContents, cfg)
	if err != nil {
		errExit("Error generating code: %v", err)
	}

	destDir := ""
	destFile := ""
	if cfg.Output != nil {
		destDir = filepath.Join(cfg.Output.Directory)
		err = os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			errExit("Error creating directory: %v", err)
		}
		if cfg.Output.UseSingleFile {
			destFile = filepath.Join(destDir, cfg.Output.Filename)
		} else {
			destDir = filepath.Join(destDir, cfg.PackageName)
			err = os.MkdirAll(destDir, os.ModePerm)
			if err != nil {
				errExit("Error creating directory: %v", err)
			}
		}
	}

	if destFile != "" {
		err = os.WriteFile(destFile, []byte(code.GetCombined()), 0644)
		if err != nil {
			errExit("Error writing file: %v", err)
		}
	} else if destDir != "" {
		for name, contents := range code {
			err = os.WriteFile(filepath.Join(destDir, name+".go"), []byte(contents), 0644)
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
