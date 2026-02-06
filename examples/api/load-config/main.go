package main

import (
	"fmt"
	"os"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/codegen"
	"gopkg.in/yaml.v3"
)

func main() {
	// Read OpenAPI spec
	specContents, err := os.ReadFile("api.yaml")
	if err != nil {
		panic(err)
	}

	// Read configuration from YAML file
	cfg := codegen.Configuration{}
	cfgContents, err := os.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}
	if err = yaml.Unmarshal(cfgContents, &cfg); err != nil {
		panic(err)
	}

	// Apply defaults to fill in any missing values
	cfg = cfg.WithDefaults()

	fmt.Printf("Package name: %s\n", cfg.PackageName)
	fmt.Printf("Output directory: %s\n", cfg.Output.Directory)
	fmt.Printf("Generate client: %v\n", cfg.Generate.Client)

	// Generate code
	generatedCode, err := codegen.Generate(specContents, cfg)
	if err != nil {
		panic(err)
	}

	// Write to file
	if err := os.WriteFile("generated.go", []byte(generatedCode["all"]), 0644); err != nil {
		panic(err)
	}

	fmt.Println("Code generated successfully to generated.go")
}
