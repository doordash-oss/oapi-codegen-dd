package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/codegen"
	"gopkg.in/yaml.v3"
)

func main() {
	// Read OpenAPI spec
	specContents, err := os.ReadFile("api.yaml")
	if err != nil {
		panic(fmt.Sprintf("reading spec: %v", err))
	}

	// Read configuration from YAML
	cfg := codegen.Configuration{}
	cfgContents, err := os.ReadFile("config.yaml")
	if err != nil {
		panic(fmt.Sprintf("reading config: %v", err))
	}
	if err = yaml.Unmarshal(cfgContents, &cfg); err != nil {
		panic(fmt.Sprintf("parsing config: %v", err))
	}
	cfg = cfg.WithDefaults()

	// Create parse context
	parseCtx, errs := codegen.CreateParseContext(specContents, cfg)
	if len(errs) > 0 {
		panic(fmt.Sprintf("parsing spec: %v", errs[0]))
	}

	// Log what we found
	fmt.Printf("Parsed OpenAPI spec:\n")
	fmt.Printf("  Operations: %d\n", len(parseCtx.Operations))
	for loc, types := range parseCtx.TypeDefinitions {
		fmt.Printf("  Types in %s: %d\n", loc, len(types))
	}

	// Create parser
	parser, err := codegen.NewParser(cfg, parseCtx)
	if err != nil {
		panic(fmt.Sprintf("creating parser: %v", err))
	}

	// Generate code
	codes, err := parser.Parse()
	if err != nil {
		panic(fmt.Sprintf("generating code: %v", err))
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(cfg.Output.Directory, 0755); err != nil {
		panic(fmt.Sprintf("creating output directory: %v", err))
	}

	// Write generated files
	if cfg.Output.UseSingleFile {
		// Single file output
		outPath := filepath.Join(cfg.Output.Directory, cfg.Output.Filename)
		formatted, err := codegen.FormatCode(codes["all"])
		if err != nil {
			panic(fmt.Sprintf("formatting code: %v", err))
		}
		if err := os.WriteFile(outPath, []byte(formatted), 0644); err != nil {
			panic(fmt.Sprintf("writing file: %v", err))
		}
		fmt.Printf("\nGenerated code written to: %s\n", outPath)
	} else {
		// Multiple file output
		for name, code := range codes {
			if name == "all" {
				continue // Skip the combined output
			}
			outPath := filepath.Join(cfg.Output.Directory, name+".go")
			formatted, err := codegen.FormatCode(code)
			if err != nil {
				panic(fmt.Sprintf("formatting %s: %v", name, err))
			}
			if err := os.WriteFile(outPath, []byte(formatted), 0644); err != nil {
				panic(fmt.Sprintf("writing %s: %v", name, err))
			}
			fmt.Printf("Generated: %s\n", outPath)
		}
	}

	fmt.Println("\nCode generation complete!")
}
