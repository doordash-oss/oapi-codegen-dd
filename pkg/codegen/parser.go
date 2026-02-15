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
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"slices"
	"sort"
	"strings"
	"text/template"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/iancoleman/strcase"
	"golang.org/x/tools/imports"
)

// Embed the templates directory
//
//go:embed templates
var templates embed.FS

// GeneratedCode is a map of file names to generated code content.
// Scaffold files (service, middleware, server/main) are prefixed with "scaffold:" in the key.
type GeneratedCode map[string]string

// GetCombined returns the combined single-file output (the "all" key).
func (g GeneratedCode) GetCombined() string {
	return g["all"]
}

// scaffoldPrefix is the prefix used to identify scaffold files in GeneratedCode.
const scaffoldPrefix = "scaffold:"

// IsScaffoldFile returns true if the file name indicates a scaffold file.
func IsScaffoldFile(name string) bool {
	return strings.HasPrefix(name, scaffoldPrefix)
}

// ScaffoldFileName returns the actual file name without the scaffold prefix.
func ScaffoldFileName(name string) string {
	return strings.TrimPrefix(name, scaffoldPrefix)
}

// Parser uses the provided ParseContext to generate Go code for the API.
type Parser struct {
	tpl *template.Template
	ctx *ParseContext
	cfg Configuration
}

type ParseOptions struct {
	OmitDescription        bool
	DefaultIntType         string
	AlwaysPrefixEnumValues bool
	SkipValidation         bool

	// ErrorMapping maps response type names to the field that should be used
	// for the Error() method. When a response type has error mapping configured,
	// it cannot be an alias (aliases don't support methods).
	ErrorMapping map[string]string

	// runtime options
	typeTracker  *TypeTracker
	reference    string
	path         []string
	specLocation SpecLocation

	// Track visited schema paths to prevent infinite recursion
	visited map[string]bool

	// model is the high-level OpenAPI model, used to resolve $ref to mutated schemas
	// instead of following stale low-level references
	model *v3high.Document
}

func (o ParseOptions) WithReference(reference string) ParseOptions {
	o.reference = reference
	return o
}

func (o ParseOptions) WithPath(path []string) ParseOptions {
	o.path = slices.Clone(path)
	return o
}

func (o ParseOptions) WithSpecLocation(specLocation SpecLocation) ParseOptions {
	o.specLocation = specLocation
	return o
}

type EnumContext struct {
	Enums       []EnumDefinition
	Imports     []string
	Config      Configuration
	WithHeader  bool
	TypeTracker *TypeTracker
}

// TplTypeContext is the context passed to templates to generate code for type definitions.
type TplTypeContext struct {
	Types []TypeDefinition

	// Map of type names to schemas for cross-referencing
	TypeSchemaMap  map[string]GoSchema
	Imports        []string
	SpecLocation   string
	Config         Configuration
	WithHeader     bool
	ResponseErrors map[string]bool
	TypeTracker    *TypeTracker
}

// TplOperationsContext is the context passed to templates to generate client code.
type TplOperationsContext struct {
	Operations    []OperationDefinition
	Imports       []string
	Config        Configuration
	WithHeader    bool
	ServerOptions *ServerOptions
	PackageName   string
}

// NewParser creates a new Parser with the provided ParseConfig and ParseContext.
func NewParser(cfg Configuration, ctx *ParseContext) (*Parser, error) {
	cfg = cfg.WithDefaults()

	tpl, err := loadTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	// load user-provided templates. Will Override built-in versions.
	for name, tplContents := range cfg.UserTemplates {
		userTpl := tpl.New(name)

		txt, err := getUserTemplateText(tplContents)
		if err != nil {
			return nil, fmt.Errorf("error loading user-provided template %q: %w", name, err)
		}

		_, err = userTpl.Parse(txt)
		if err != nil {
			return nil, fmt.Errorf("error parsing user-provided template %q: %w", name, err)
		}
	}

	return &Parser{
		tpl: tpl,
		ctx: ctx,
		cfg: cfg,
	}, nil
}

// Parse generates Go code for the API using the provided ParseContext.
// It returns a map of generated code for each type of definition.
func (p *Parser) Parse() (GeneratedCode, error) {
	typesOut := make(map[string]string)
	scaffoldOut := make(map[string]string)

	useSingleFile := p.cfg.Output != nil && p.cfg.Output.UseSingleFile
	withHeader := !useSingleFile

	// Only generate models if Models is not explicitly false
	shouldGenerateModels := p.cfg.Generate == nil || p.cfg.Generate.Models == nil || *p.cfg.Generate.Models
	if useSingleFile {
		out, err := p.ParseTemplates([]string{"header-inc.tmpl"}, EnumContext{
			Imports:    p.ctx.Imports,
			Config:     p.cfg,
			WithHeader: true,
		})
		if err != nil {
			return nil, fmt.Errorf("error generating code for header: %w", err)
		}
		typesOut["header"] = out

		// Generate validator declaration for single file mode (only if generating models)
		if shouldGenerateModels && !p.cfg.Generate.Validation.Skip {
			out, err := p.ParseTemplates([]string{"common.tmpl"}, EnumContext{
				Imports:    p.ctx.Imports,
				Config:     p.cfg,
				WithHeader: false,
			})
			if err != nil {
				return nil, fmt.Errorf("error generating code for validator: %w", err)
			}
			typesOut["validator"] = out
		}
	}

	if len(p.ctx.Operations) > 0 && p.cfg.Generate.Client {
		opsCtx := &TplOperationsContext{
			Operations: p.ctx.Operations,
			Imports:    p.ctx.Imports,
			Config:     p.cfg,
			WithHeader: withHeader,
		}
		for _, tmpl := range []string{"client", "client-options"} {
			out, err := p.ParseTemplates([]string{tmpl + ".tmpl"}, opsCtx)
			if err != nil {
				return nil, fmt.Errorf("error generating code for client: %w", err)
			}
			formatted := out
			if !useSingleFile {
				formatted, err = FormatCode(out)
				if err != nil {
					return nil, err
				}
			}
			typesOut[strcase.ToSnake(tmpl)] = formatted
		}
	}

	// Generate handler code if handler generation is enabled
	if len(p.ctx.Operations) > 0 && p.cfg.Generate.Handler != nil {
		opsCtx := &TplOperationsContext{
			Operations: p.ctx.Operations,
			Imports:    p.ctx.Imports,
			Config:     p.cfg,
			WithHeader: withHeader,
		}
		// Determine which templates to use based on handler kind
		handlerKind := p.cfg.Generate.Handler.Kind
		templatePrefix := "handler/" + string(handlerKind) + "/"
		sharedPrefix := "handler/"

		// Generate handler files
		if useSingleFile {
			// In single-file mode, use the router-specific template which includes all shared templates
			out, err := p.ParseTemplates([]string{templatePrefix + "handler.tmpl"}, opsCtx)
			if err != nil {
				return nil, fmt.Errorf("error generating code for handler: %w", err)
			}
			typesOut["handler"] = out
		} else {
			// In multi-file mode, generate separate files from shared templates
			for _, tmpl := range []string{"errors", "adapter", "router"} {
				out, err := p.ParseTemplates([]string{sharedPrefix + tmpl + ".tmpl"}, opsCtx)
				if err != nil {
					return nil, fmt.Errorf("error generating code for %s: %w", tmpl, err)
				}
				formatted, err := FormatCode(out)
				if err != nil {
					return nil, err
				}
				typesOut[strcase.ToSnake(tmpl)] = formatted
			}
		}

		// Generate shared templates (router-agnostic) - these are regenerated files
		for _, tmpl := range []string{"response-data", "service-options"} {
			out, err := p.ParseTemplates([]string{sharedPrefix + tmpl + ".tmpl"}, opsCtx)
			if err != nil {
				return nil, fmt.Errorf("error generating code for %s: %w", tmpl, err)
			}
			formatted := out
			if !useSingleFile {
				formatted, err = FormatCode(out)
				if err != nil {
					return nil, fmt.Errorf("error formatting %s: %w", tmpl, err)
				}
			}
			typesOut[strcase.ToSnake(tmpl)] = formatted
		}

		// Resolve scaffold output once for service and middleware
		scaffoldOutput := p.cfg.Generate.Handler.ResolveScaffoldOutput(p.cfg.Output)
		scaffoldPackage := scaffoldOutput.Package
		if scaffoldPackage == "" {
			scaffoldPackage = p.cfg.PackageName
		}

		// Generate middleware if enabled - scaffolded file
		if p.cfg.Generate.Handler.Middleware != nil {
			middlewareCtx := &TplOperationsContext{
				Operations:  p.ctx.Operations,
				Imports:     p.ctx.Imports,
				Config:      p.cfg,
				WithHeader:  withHeader,
				PackageName: scaffoldPackage,
			}
			middlewareTmpl := templatePrefix + "middleware.tmpl"
			middlewareOut, err := p.ParseTemplates([]string{middlewareTmpl}, middlewareCtx)
			if err != nil {
				// Fall back to shared middleware template
				middlewareOut, err = p.ParseTemplates([]string{sharedPrefix + "middleware.tmpl"}, middlewareCtx)
				if err != nil {
					return nil, fmt.Errorf("error generating code for middleware: %w", err)
				}
			}
			// Scaffold files are always separate files, so always format them
			formattedMiddleware, err := FormatCode(middlewareOut)
			if err != nil {
				return nil, fmt.Errorf("error formatting middleware: %w", err)
			}

			// Use directory path as key if scaffold has different output directory
			middlewareKey := "middleware"
			if scaffoldOutput.Directory != "" && scaffoldOutput.Directory != p.cfg.Output.Directory {
				middlewareKey = scaffoldOutput.Directory + "/middleware"
			}
			scaffoldOut[middlewareKey] = formattedMiddleware
		}

		// Generate service implementation stub - scaffolded file
		serviceCtx := &TplOperationsContext{
			Operations:  p.ctx.Operations,
			Imports:     p.ctx.Imports,
			Config:      p.cfg,
			WithHeader:  withHeader,
			PackageName: scaffoldPackage,
		}
		out, err := p.ParseTemplates([]string{sharedPrefix + "service.tmpl"}, serviceCtx)
		if err != nil {
			return GeneratedCode{}, fmt.Errorf("error generating code for handler implementation: %w", err)
		}

		// Scaffold files are always separate files, so always format them
		formatted, err := FormatCode(out)
		if err != nil {
			return nil, fmt.Errorf("error formatting service: %w", err)
		}

		// Use directory path as key if scaffold has different output directory
		serviceKey := "service"
		if scaffoldOutput.Directory != "" && scaffoldOutput.Directory != p.cfg.Output.Directory {
			serviceKey = scaffoldOutput.Directory + "/service"
		}
		scaffoldOut[serviceKey] = formatted

		// Generate server main.go if server generation is enabled - scaffolded file
		if p.cfg.Generate.Handler.Server != nil {
			serverOpts := p.cfg.Generate.Handler.Server.WithDefaults()
			if err := serverOpts.Validate(); err != nil {
				return nil, fmt.Errorf("invalid server options: %w", err)
			}
			serverOutput := p.cfg.Generate.Handler.ResolveServerOutput()
			serverCtx := &TplOperationsContext{
				Operations:    p.ctx.Operations,
				Imports:       p.ctx.Imports,
				Config:        p.cfg,
				WithHeader:    withHeader,
				ServerOptions: &serverOpts,
				PackageName:   serverOutput.Package,
			}

			// Try framework-specific server template first, fall back to shared
			serverTmpl := templatePrefix + "server.tmpl"
			out, err := p.ParseTemplates([]string{serverTmpl}, serverCtx)
			if err != nil {
				// Fall back to shared server template
				out, err = p.ParseTemplates([]string{sharedPrefix + "server.tmpl"}, serverCtx)
				if err != nil {
					return nil, fmt.Errorf("error generating code for server: %w", err)
				}
			}

			formatted, err := FormatCode(out)
			if err != nil {
				return nil, fmt.Errorf("error formatting server: %w", err)
			}
			scaffoldOut[serverOutput.Directory+"/main"] = formatted
		}
	}

	// Generate MCP server tools if MCP server generation is enabled
	if len(p.ctx.Operations) > 0 && p.cfg.Generate.MCPServer != nil {
		if !p.cfg.Generate.Client {
			return nil, fmt.Errorf("MCP server generation requires client generation to be enabled (set generate.client: true)")
		}
		opsCtx := &TplOperationsContext{
			Operations: p.ctx.Operations,
			Imports:    p.ctx.Imports,
			Config:     p.cfg,
			WithHeader: withHeader,
		}
		out, err := p.ParseTemplates([]string{"mcp/tools.tmpl"}, opsCtx)
		if err != nil {
			return nil, fmt.Errorf("error generating code for MCP tools: %w", err)
		}
		formatted := out
		if !useSingleFile {
			formatted, err = FormatCode(out)
			if err != nil {
				return nil, fmt.Errorf("error formatting MCP tools: %w", err)
			}
		}
		typesOut["mcp_tools"] = formatted
	}

	// Generate validator file if validation is not skipped, not using single file, and generating models
	if shouldGenerateModels && !useSingleFile && !p.cfg.Generate.Validation.Skip {
		out, err := p.ParseTemplates([]string{"common.tmpl"}, EnumContext{
			Imports:    p.ctx.Imports,
			Config:     p.cfg,
			WithHeader: withHeader,
		})
		if err != nil {
			return nil, fmt.Errorf("error generating code for validator: %w", err)
		}
		formatted, err := FormatCode(out)
		if err != nil {
			return nil, err
		}
		typesOut["common"] = formatted
	}
	if shouldGenerateModels && len(p.ctx.Enums) > 0 {
		out, err := p.ParseTemplates([]string{"enums.tmpl"}, EnumContext{
			Enums:       p.ctx.Enums,
			Imports:     p.ctx.Imports,
			Config:      p.cfg,
			WithHeader:  withHeader,
			TypeTracker: p.ctx.TypeTracker,
		})
		if err != nil {
			return nil, fmt.Errorf("error generating code for type enums: %w", err)
		}
		formatted := out
		if !useSingleFile {
			formatted, err = FormatCode(out)
			if err != nil {
				return nil, err
			}
		}
		typesOut["enums"] = formatted
	}

	responseErrs := make(map[string]bool)
	for _, respErr := range p.ctx.ResponseErrors {
		responseErrs[respErr] = true
	}

	// Build a map of type names to schemas for cross-referencing
	typeSchemaMap := make(map[string]GoSchema)
	for _, tds := range p.ctx.TypeDefinitions {
		for _, td := range tds {
			typeSchemaMap[td.Name] = td.Schema
		}
	}
	for _, td := range p.ctx.UnionTypes {
		typeSchemaMap[td.Name] = td.Schema
	}

	// Only generate model types if Models is not explicitly false
	if shouldGenerateModels {
		for sl, tds := range p.ctx.TypeDefinitions {
			if len(tds) == 0 {
				continue
			}
			typesCtx := &TplTypeContext{
				Types:          tds,
				TypeSchemaMap:  typeSchemaMap,
				SpecLocation:   string(sl),
				Imports:        p.ctx.Imports,
				Config:         p.cfg,
				WithHeader:     withHeader,
				ResponseErrors: responseErrs,
				TypeTracker:    p.ctx.TypeTracker,
			}
			out, err := p.ParseTemplates([]string{"types.tmpl"}, typesCtx)
			if err != nil {
				return nil, fmt.Errorf("error generating code for %s type definitions: %w", sl, err)
			}
			formatted := out
			if !useSingleFile {
				formatted, err = FormatCode(out)
				if err != nil {
					return nil, err
				}
			}
			typesOut[getSpecLocationOutName(sl)] = formatted
		}

		if len(p.ctx.UnionTypes) > 0 {
			out, err := p.ParseTemplates([]string{"types.tmpl", "union.tmpl"}, &TplTypeContext{
				Types:          p.ctx.UnionTypes,
				TypeSchemaMap:  typeSchemaMap,
				SpecLocation:   "union",
				Imports:        p.ctx.Imports,
				Config:         p.cfg,
				WithHeader:     withHeader,
				ResponseErrors: responseErrs,
				TypeTracker:    p.ctx.TypeTracker,
			})
			if err != nil {
				return nil, fmt.Errorf("error generating code for union types: %w", err)
			}
			formatted := out
			if !useSingleFile {
				formatted, err = FormatCode(out)
				if err != nil {
					return nil, err
				}
			}
			typesOut["unions"] = formatted
		}
	}

	if useSingleFile {
		res := ""
		if header, ok := typesOut["header"]; ok {
			res += header + "\n"
			delete(typesOut, "header")
		}

		// sort the types out by name
		typeNames := make([]string, 0, len(typesOut))
		for name := range typesOut {
			typeNames = append(typeNames, name)
		}

		sort.Strings(typeNames)

		for _, name := range typeNames {
			code, ok := typesOut[name]
			if !ok {
				continue
			}
			res += code + "\n"
		}

		formatted, err := FormatCode(res)
		if err != nil {
			println(res)
			return nil, err
		}
		typesOut = map[string]string{"all": formatted}
	}

	// Merge scaffold files into the main map with prefix
	for name, content := range scaffoldOut {
		typesOut[scaffoldPrefix+name] = content
	}

	return typesOut, nil
}

// ParseTemplates parses provided templates with the given data and returns the generated code.
func (p *Parser) ParseTemplates(templates []string, data any) (string, error) {
	var generatedTemplates []string
	for _, tmpl := range templates {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)

		if err := p.tpl.ExecuteTemplate(w, tmpl, data); err != nil {
			return "", fmt.Errorf("error generating %s: %s", tmpl, err)
		}
		if err := w.Flush(); err != nil {
			return "", fmt.Errorf("error flushing output buffer for %s: %s", tmpl, err)
		}
		generatedTemplates = append(generatedTemplates, buf.String())
	}

	return strings.Join(generatedTemplates, "\n"), nil
}

func loadTemplates(cfg Configuration) (*template.Template, error) {
	tpl := template.New("templates").Funcs(TemplateFunctions)

	// Load templates from specific directories in order:
	// 1. Root templates (templates/*.tmpl)
	// 2. Handler shared templates (templates/handler/*.tmpl)
	// 3. Selected framework templates (templates/handler/{kind}/*.tmpl)
	dirs := []string{
		"templates",
		"templates/handler",
	}

	// Add framework-specific directory if handler is configured
	if cfg.Generate != nil && cfg.Generate.Handler != nil && cfg.Generate.Handler.Kind != "" {
		dirs = append(dirs, "templates/handler/"+string(cfg.Generate.Handler.Kind))
	}

	// Add MCP templates directory if MCP server is configured
	if cfg.Generate != nil && cfg.Generate.MCPServer != nil {
		dirs = append(dirs, "templates/mcp")
	}

	for _, dir := range dirs {
		entries, err := templates.ReadDir(dir)
		if err != nil {
			// Skip if directory doesn't exist
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			path := dir + "/" + entry.Name()
			buf, err := templates.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("error reading file '%s': %w", path, err)
			}

			templateName := strings.TrimPrefix(path, "templates/")
			tmpl := tpl.New(templateName)
			_, err = tmpl.Parse(string(buf))
			if err != nil {
				return nil, fmt.Errorf("parsing template '%s': %w", path, err)
			}
		}
	}

	return tpl, nil
}

// FormatCode formats the provided Go code.
// It optimizes imports and formats the code using gofmt.
func FormatCode(src string) (string, error) {
	src = strings.Trim(src, "\n") + "\n"
	if src == "\n" || src == "" {
		return src, nil
	}

	res, err := optimizeImports([]byte(src))
	if err != nil {
		return "", fmt.Errorf("error optimizing imports: %w", err)
	}

	res, err = format.Source(res)
	if err != nil {
		return "", fmt.Errorf("error formatting code: %w", err)
	}

	return sanitizeCode(string(res)), nil
}

// sanitizeCode runs sanitizers across the generated Go code to ensure the
// generated code will be able to compile.
func sanitizeCode(src string) string {
	// remove any byte-order-marks which break Go-Code
	// See: https://groups.google.com/forum/#!topic/golang-nuts/OToNIPdfkks
	return strings.ReplaceAll(src, "\uFEFF", "")
}

func optimizeImports(src []byte) ([]byte, error) {
	outBytes, err := imports.Process("gen.go", src, nil)
	if err != nil {
		return nil, err
	}
	return outBytes, nil
}

func getSpecLocationOutName(specLocation SpecLocation) string {
	switch specLocation {
	case SpecLocationPath:
		return "paths"
	case SpecLocationQuery:
		return "queries"
	case SpecLocationHeader:
		return "headers"
	case SpecLocationBody:
		return "payloads"
	case SpecLocationResponse:
		return "responses"
	case SpecLocationSchema:
		return "types"
	case SpecLocationUnion:
		return "unions"
	default:
		return string(specLocation)
	}
}

// getUserTemplateText attempts to retrieve the template text from a passed string or file..
func getUserTemplateText(inputData string) (template string, err error) {
	// if the input data is more than one line, assume its a template and return that data.
	if strings.Contains(inputData, "\n") {
		return inputData, nil
	}

	// load data from file
	// #nosec G304 -- CLI tool intentionally reads user-specified template files
	data, err := os.ReadFile(inputData)
	// return data if found and loaded
	if err == nil {
		return string(data), nil
	}

	// check for non "not found" errors
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to open file %s: %w", inputData, err)
	}

	return string(data), nil
}
