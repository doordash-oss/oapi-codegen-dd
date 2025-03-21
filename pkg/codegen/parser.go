package codegen

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"io/fs"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

// Embed the templates directory
//
//go:embed templates
var templates embed.FS

// Parser uses the provided ParseContext to generate Go code for the API.
type Parser struct {
	tpl *template.Template
	ctx *ParseContext
	cfg *Configuration
}

// TplTypeContext is the context passed to templates to generate code for type definitions.
type TplTypeContext struct {
	Types        []TypeDefinition
	Imports      []string
	SpecLocation string
	Config       *Configuration
}

// NewParser creates a new Parser with the provided ParseConfig and ParseContext.
func NewParser(cfg *Configuration, ctx *ParseContext) (*Parser, error) {
	tpl, err := loadTemplates()
	if err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}
	return &Parser{
		tpl: tpl,
		ctx: ctx,
		cfg: cfg,
	}, nil
}

// Parse generates Go code for the API using the provided ParseContext.
// It returns a map of generated code for each type of definition.
func (p *Parser) Parse() (map[string]string, error) {
	typesOut := make(map[string]string)

	if len(p.ctx.Enums) > 0 {
		out, err := p.ParseTemplates([]string{"enums.tmpl"}, p.ctx)
		if err != nil {
			return nil, fmt.Errorf("error generating code for type enums: %w", err)
		}
		typesOut["enums"] = FormatCode(out)
	}

	for sl, tds := range p.ctx.TypeDefinitions {
		if len(tds) == 0 {
			continue
		}
		typesCtx := &TplTypeContext{
			Types:        tds,
			SpecLocation: string(sl),
			Imports:      p.ctx.Imports,
			Config:       p.cfg,
		}
		out, err := p.ParseTemplates([]string{"types.tmpl"}, typesCtx)
		if err != nil {
			return nil, fmt.Errorf("error generating code for %s type definitions: %w", sl, err)
		}
		typesOut[getSpecLocationOutName(sl)] = FormatCode(out)
	}

	if len(p.ctx.AdditionalTypes) > 0 {
		out, err := p.ParseTemplates([]string{"additional-properties.tmpl"}, &TplTypeContext{
			Types:   p.ctx.AdditionalTypes,
			Imports: p.ctx.Imports,
		})
		if err != nil {
			return nil, fmt.Errorf("error generating code for additional properties: %w", err)
		}
		typesOut["additional"] = FormatCode(out)
	}

	if len(p.ctx.UnionTypes) > 0 {
		out, err := p.ParseTemplates([]string{"types.tmpl", "union.tmpl"}, &TplTypeContext{
			Types:        p.ctx.UnionTypes,
			SpecLocation: "union",
			Imports:      p.ctx.Imports,
		})
		if err != nil {
			return nil, fmt.Errorf("error generating code for union types: %w", err)
		}
		typesOut["unions"] = FormatCode(out)
	}

	if len(p.ctx.UnionWithAdditionalTypes) > 0 {
		out, err := p.ParseTemplates([]string{"union-and-additional-properties.tmpl"}, &TplTypeContext{
			Types:        p.ctx.UnionWithAdditionalTypes,
			SpecLocation: "union_with_additional",
			Imports:      p.ctx.Imports,
		})
		if err != nil {
			return nil, fmt.Errorf("error generating code for union types with additional properties: %w", err)
		}
		typesOut["unions_with_additional"] = FormatCode(out)
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
func FormatCode(src string) string {
	src = strings.Trim(src, "\n") + "\n"
	if src == "" {
		return src
	}

	res, err := optimizeImports([]byte(src))
	if err != nil {
		return string(res)
	}

	res, err = format.Source(res)
	if err != nil {
		return string(res)
	}

	return sanitizeCode(string(res))
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
		return src, err
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
