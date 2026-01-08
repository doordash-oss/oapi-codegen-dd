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

	"github.com/iancoleman/strcase"
	"golang.org/x/tools/imports"
)

// Embed the templates directory
//
//go:embed templates
var templates embed.FS

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

	// runtime options
	currentTypes map[string]TypeDefinition
	reference    string
	path         []string
	specLocation SpecLocation

	// Track visited schema paths to prevent infinite recursion
	visited map[string]bool

	// naming parameters
	baseName     string
	nameSuffixes []string
}

func (o ParseOptions) WithReference(reference string) ParseOptions {
	o.reference = reference
	return o
}

func (o ParseOptions) WithPath(path []string) ParseOptions {
	o.path = slices.Clone(path)
	return o
}

func (o ParseOptions) WithCurrentTypes(currentTypes map[string]TypeDefinition) ParseOptions {
	o.currentTypes = currentTypes
	return o
}

func (o ParseOptions) AddType(td TypeDefinition) ParseOptions {
	o.currentTypes[td.Name] = td
	return o
}

func (o ParseOptions) WithBaseName(baseName string) ParseOptions {
	o.baseName = baseName
	return o
}

func (o ParseOptions) WithNameSuffixes(suffixes []string) ParseOptions {
	o.nameSuffixes = slices.Clone(suffixes)
	return o
}

func (o ParseOptions) WithSpecLocation(specLocation SpecLocation) ParseOptions {
	o.specLocation = specLocation
	return o
}

type EnumContext struct {
	Enums        []EnumDefinition
	Imports      []string
	Config       Configuration
	WithHeader   bool
	TypeRegistry TypeRegistry
}

// TplTypeContext is the context passed to templates to generate code for type definitions.
type TplTypeContext struct {
	Types          []TypeDefinition
	Imports        []string
	SpecLocation   string
	Config         Configuration
	WithHeader     bool
	ResponseErrors map[string]bool
}

// TplOperationsContext is the context passed to templates to generate client code.
type TplOperationsContext struct {
	Operations []OperationDefinition
	Imports    []string
	Config     Configuration
	WithHeader bool
}

// NewParser creates a new Parser with the provided ParseConfig and ParseContext.
func NewParser(cfg Configuration, ctx *ParseContext) (*Parser, error) {
	cfg = cfg.Merge(NewDefaultConfiguration())
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

	if len(p.ctx.Enums) > 0 {
		out, err := p.ParseTemplates([]string{"enums.tmpl"}, EnumContext{
			Enums:        p.ctx.Enums,
			Imports:      p.ctx.Imports,
			Config:       p.cfg,
			WithHeader:   withHeader,
			TypeRegistry: p.ctx.TypeRegistry,
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

	for sl, tds := range p.ctx.TypeDefinitions {
		if len(tds) == 0 {
			continue
		}
		typesCtx := &TplTypeContext{
			Types:          tds,
			SpecLocation:   string(sl),
			Imports:        p.ctx.Imports,
			Config:         p.cfg,
			WithHeader:     withHeader,
			ResponseErrors: responseErrs,
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
			SpecLocation:   "union",
			Imports:        p.ctx.Imports,
			Config:         p.cfg,
			WithHeader:     withHeader,
			ResponseErrors: responseErrs,
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
			delete(typesOut, name)
		}

		formatted, err := FormatCode(res)
		if err != nil {
			println(res)
			return nil, err
		}
		typesOut = map[string]string{"all": formatted}
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
