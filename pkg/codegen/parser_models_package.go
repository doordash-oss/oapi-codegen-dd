// Copyright 2026 DoorDash, Inc.
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
	"go/ast"
	"go/parser"
	"sort"
	"strings"
	"text/template"
)

// modelsQualifier prefixes model type identifiers in a Go type expression
// with the alias of the package the models were generated into, for handler
// code that is generated into a separate package (generate.handler.models-package).
//
// A zero-value modelsQualifier (no alias) is a documented no-op: every method
// returns its input unchanged, which is what keeps output byte-identical when
// models-package isn't set.
type modelsQualifier struct {
	alias string
	names map[string]bool
}

// newModelsQualifier builds a modelsQualifier for the given alias. names is
// the set of type names the models run would emit for this spec - typically
// the keys of the typeSchemaMap already built from ParseContext.
func newModelsQualifier(alias string, names map[string]GoSchema) *modelsQualifier {
	if alias == "" {
		return &modelsQualifier{}
	}
	nameSet := make(map[string]bool, len(names))
	for name := range names {
		nameSet[name] = true
	}
	return &modelsQualifier{alias: alias, names: nameSet}
}

// funcMap returns the template functions this qualifier backs: "modelType"
// qualifies a Go type expression, "modelsPrefix" returns "<alias>." (or "")
// for qualifying function calls such as error constructors.
func (q *modelsQualifier) funcMap() template.FuncMap {
	return template.FuncMap{
		"modelType":    q.qualify,
		"modelsPrefix": q.prefix,
	}
}

// qualify prefixes every identifier in expr that names a model type with the
// models package alias, e.g. "*User" -> "*models.User". Composite type
// expressions ("[]*User", "map[string]User", "struct{}") and plain
// identifiers are both supported; expressions that aren't valid Go type
// syntax, or reference no model type, are returned unchanged.
func (q *modelsQualifier) qualify(expr string) string {
	if q == nil || q.alias == "" || expr == "" {
		return expr
	}

	node, err := parser.ParseExpr(expr)
	if err != nil {
		return expr
	}

	type replacement struct {
		start, end int
		text       string
	}
	var replacements []replacement

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			// Already qualified (e.g. time.Time, runtime.Date) - leave it alone.
			return false
		case *ast.Ident:
			if q.names[x.Name] {
				replacements = append(replacements, replacement{
					start: int(x.Pos()) - 1,
					end:   int(x.End()) - 1,
					text:  q.alias + "." + x.Name,
				})
			}
		}
		return true
	})

	if len(replacements) == 0 {
		return expr
	}

	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start < replacements[j].start })

	var b strings.Builder
	last := 0
	for _, r := range replacements {
		b.WriteString(expr[last:r.start])
		b.WriteString(r.text)
		last = r.end
	}
	b.WriteString(expr[last:])
	return b.String()
}

// prefix returns "<alias>." or "" if no alias is configured. Used to qualify
// calls into the models package, such as generated error constructors
// (New<ErrorType>), that aren't themselves type expressions.
func (q *modelsQualifier) prefix() string {
	if q == nil || q.alias == "" {
		return ""
	}
	return q.alias + "."
}
