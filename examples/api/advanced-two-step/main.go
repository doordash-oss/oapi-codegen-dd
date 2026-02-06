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

	// Create configuration
	cfg := codegen.NewDefaultConfiguration()

	// Step 1: Create parse context
	parseCtx, errs := codegen.CreateParseContext(specContents, cfg)
	if len(errs) > 0 {
		panic(fmt.Sprintf("parsing spec: %v", errs[0]))
	}

	// Inspect what was parsed
	fmt.Printf("Found %d operations\n", len(parseCtx.Operations))
	for loc, types := range parseCtx.TypeDefinitions {
		fmt.Printf("Found %d types in %s\n", len(types), loc)
	}

	// Step 2: Create parser and generate code
	parser, err := codegen.NewParser(cfg, parseCtx)
	if err != nil {
		panic(err)
	}

	codes, err := parser.Parse()
	if err != nil {
		panic(err)
	}

	// Write generated code
	if err := os.WriteFile("generated.go", []byte(codes["all"]), 0644); err != nil {
		panic(err)
	}

	fmt.Println("Code generated successfully to generated.go")
}
