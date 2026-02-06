package main

import (
	"fmt"
	"os"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/codegen"
)

func main() {
	// Read OpenAPI spec
	specContents, err := os.ReadFile("api.yaml")
	if err != nil {
		panic(err)
	}

	// Create configuration with defaults
	cfg := codegen.NewDefaultConfiguration()
	cfg.PackageName = "api"

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
