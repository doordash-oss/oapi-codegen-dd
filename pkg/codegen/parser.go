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
	"io/fs"
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

// isScaffoldOnce checks if a file name is a scaffold-once file.
// For keys with "/" it matches by suffix, otherwise exact match.
func isScaffoldOnce(name string) bool {
	if ScaffoldOnceFiles[name] {
		return true
	}
	for key := range ScaffoldOnceFiles {
		if strings.Contains(key, "/") && strings.HasSuffix(name, key) {
			return true
		}
	}
	return false
}

type GeneratedCode map[string]string

func (g GeneratedCode) GetCombined() string {
	return g["all"]
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
}

// NewParser creates a new Parser with the provided ParseConfig and ParseContext.
func NewParser(cfg Configuration, ctx *ParseContext) (*Parser, error) {
	cfg = cfg.WithDefaults()
	tpl, err := loadTemplates()
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

	useSingleFile := p.cfg.Output != nil && p.cfg.Output.UseSingleFile
	withHeader := !useSingleFile
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

		// Generate validator declaration for single file mode
		if !p.cfg.Generate.Validation.Skip {
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
		sharedPrefix := "handler/shared/"

		// Generate handler files
		if useSingleFile {
			// In single-file mode, use the router-specific template which includes all shared templates
			out, err := p.ParseTemplates([]string{templatePrefix + "handler.tmpl"}, opsCtx)
			if err != nil {
				return nil, fmt.Errorf("error generating code for handler: %w", err)
			}
			typesOut["handler"] = out
		} else {
			// In multi-file mode, generate three separate files from shared templates
			for _, tmpl := range []string{"handler", "adapter", "router"} {
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

		// Generate shared templates (router-agnostic)
		for _, tmpl := range []string{"middleware", "response-data", "handler-options"} {
			out, err := p.ParseTemplates([]string{sharedPrefix + tmpl + ".tmpl"}, opsCtx)
			if err != nil {
				return nil, fmt.Errorf("error generating code for %s: %w", tmpl, err)
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

		// Generate handler implementation stub (scaffold once - only written if file doesn't exist)
		out, err := p.ParseTemplates([]string{sharedPrefix + "handler-impl.tmpl"}, opsCtx)
		if err != nil {
			return nil, fmt.Errorf("error generating code for handler implementation: %w", err)
		}
		formatted := out
		if !useSingleFile {
			formatted, err = FormatCode(out)
			if err != nil {
				return nil, err
			}
		}
		typesOut["handler_impl"] = formatted

		// Generate server main.go if server generation is enabled
		if p.cfg.Generate.Handler.Server != nil {
			serverOpts := p.cfg.Generate.Handler.Server.WithDefaults()
			if err := serverOpts.Validate(); err != nil {
				return nil, fmt.Errorf("invalid server options: %w", err)
			}
			serverCtx := opsCtx
			serverCtx.ServerOptions = &serverOpts
			out, err := p.ParseTemplates([]string{sharedPrefix + "server.tmpl"}, serverCtx)
			if err != nil {
				return nil, fmt.Errorf("error generating code for server: %w", err)
			}
			formatted, err := FormatCode(out)
			if err != nil {
				return nil, err
			}
			typesOut[serverOpts.Directory+"/main"] = formatted
		}
	}

	// Generate validator file if validation is not skipped and not using single file
	if !useSingleFile && !p.cfg.Generate.Validation.Skip {
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

	if len(p.ctx.Enums) > 0 {
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
	shouldGenerateModels := p.cfg.Generate == nil || p.cfg.Generate.Models == nil || *p.cfg.Generate.Models
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

		// Store scaffold-once files to keep separate
		scaffoldFiles := make(map[string]string)

		// sort the types out by name
		typeNames := make([]string, 0, len(typesOut))
		for name := range typesOut {
			typeNames = append(typeNames, name)
		}

		sort.Strings(typeNames)

		for _, name := range typeNames {
			// Skip scaffold-once files - they should remain separate
			if isScaffoldOnce(name) {
				scaffoldFiles[name] = typesOut[name]
				delete(typesOut, name)
				continue
			}
			code, ok := typesOut[name]
			if !ok {
				continue
			}
			res += code + "\n"
			delete(typesOut, name)
		}

		formatted, err := FormatCode(res)
		if err != nil {
			println(res)
			return nil, err
		}
		typesOut = map[string]string{"all": formatted}

		// Add back scaffold-once files
		for name, code := range scaffoldFiles {
			typesOut[name] = code
		}
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

func loadTemplates() (*template.Template, error) {
	tpl := template.New("templates").Funcs(TemplateFunctions)

	err := fs.WalkDir(templates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking directory %s: %w", path, err)
		}
		if d.IsDir() {
			return nil
		}

		buf, err := templates.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file '%s': %w", path, err)
		}

		templateName := strings.TrimPrefix(path, "templates/")
		tmpl := tpl.New(templateName)
		_, err = tmpl.Parse(string(buf))
		if err != nil {
			return fmt.Errorf("parsing template '%s': %w", path, err)
		}
		return nil
	})

	return tpl, err
}

// FormatCode formats the provided Go code.
// It optimizes imports and formats the code using gofmt.
func FormatCode(src string) (string, error) {
	src = strings.Trim(src, "\n") + "\n"
	if src == "" {
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
