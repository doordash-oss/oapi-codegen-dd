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

	// Create parse context to access type definitions
	parseCtx, errs := codegen.CreateParseContext(specContents, cfg)
	if len(errs) > 0 {
		panic(fmt.Sprintf("parsing spec: %v", errs[0]))
	}

	// Access operations
	fmt.Printf("=== Operations (%d) ===\n", len(parseCtx.Operations))
	for _, op := range parseCtx.Operations {
		fmt.Printf("- %s %s (OperationID: %s)\n", op.Method, op.Path, op.ID)
		if op.Body != nil {
			fmt.Printf("  Request: %s\n", op.Body.Name)
		}
		for code, resp := range op.Response.All {
			fmt.Printf("  Response %d: %s\n", code, resp.ResponseName)
		}
	}

	// Access type definitions
	fmt.Printf("\n=== Type Definitions ===\n")
	for loc, types := range parseCtx.TypeDefinitions {
		fmt.Printf("\n%s (%d types):\n", loc, len(types))
		for _, td := range types {
			fmt.Printf("- %s (GoType: %s)\n", td.Name, td.Schema.GoType)
			if td.Schema.Description != "" {
				fmt.Printf("  Description: %s\n", td.Schema.Description)
			}
			if td.IsAlias() {
				fmt.Printf("  Is alias: true\n")
			}
			if td.IsOptional() {
				fmt.Printf("  Is optional: true\n")
			}
		}
	}

	// Use TypeTracker to look up types
	fmt.Printf("\n=== TypeTracker Lookups ===\n")
	userRef := "#/components/schemas/User"
	if typeName, ok := parseCtx.TypeTracker.LookupByRef(userRef); ok {
		fmt.Printf("Found type for ref %s: %s\n", userRef, typeName)
	}

	if typeDef, ok := parseCtx.TypeTracker.LookupByName("User"); ok {
		fmt.Printf("Found type by name 'User': %s (GoType: %s)\n",
			typeDef.Name, typeDef.Schema.GoType)
	}
}
