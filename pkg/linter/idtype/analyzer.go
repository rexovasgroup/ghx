// Package idtype provides a go/analysis analyzer that flags struct fields
// representing GitHub database IDs that use int instead of int64.
//
// GitHub database IDs are internally 64-bit, and the REST API OpenAPI spec
// declares many of them with format: int64. Using Go's int (which is
// platform-dependent) risks overflow on 32-bit architectures.
package idtype

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "idtype",
	Doc:      "checks that struct fields representing GitHub database IDs use int64 instead of int",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.StructType)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		st := n.(*ast.StructType)
		for _, field := range st.Fields.List {
			if !isIntType(field.Type) {
				continue
			}
			for _, name := range field.Names {
				if isIDField(name.Name, field.Tag) {
					pass.Reportf(field.Pos(), "struct field %s looks like a GitHub database ID but uses int; use int64 instead", name.Name)
				}
			}
		}
	})

	return nil, nil
}

// isIntType checks if the type expression is the bare "int" type.
func isIntType(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "int"
}

// isIDField returns true if the field name or JSON tag suggests this is a
// database ID. It checks:
//   - Field name is exactly "ID" or "Id"
//   - Field name ends with "ID" or "Id" (e.g. DatabaseId, RepositoryID, ActorId)
//   - JSON tag is exactly "id" or ends with "_id"
func isIDField(fieldName string, tag *ast.BasicLit) bool {
	if matchesIDName(fieldName) {
		return true
	}
	if tag != nil && tag.Kind == token.STRING {
		jsonTag := extractJSONTag(tag.Value)
		if matchesIDTag(jsonTag) {
			return true
		}
	}
	return false
}

// matchesIDName checks if a Go field name looks like a database ID field.
func matchesIDName(name string) bool {
	if name == "ID" || name == "Id" {
		return true
	}
	if strings.HasSuffix(name, "ID") || strings.HasSuffix(name, "Id") {
		return true
	}
	return false
}

// matchesIDTag checks if a JSON tag value looks like a database ID field.
func matchesIDTag(tag string) bool {
	if tag == "" {
		return false
	}
	if tag == "id" {
		return true
	}
	if strings.HasSuffix(tag, "_id") {
		return true
	}
	return false
}

// extractJSONTag extracts the JSON field name from a struct tag literal.
// For example, given `json:"repository_id,omitempty"`, it returns "repository_id".
func extractJSONTag(rawTag string) string {
	// Remove surrounding backticks or quotes
	tag := strings.Trim(rawTag, "`\"")

	const prefix = `json:"`
	idx := strings.Index(tag, prefix)
	if idx < 0 {
		return ""
	}
	tag = tag[idx+len(prefix):]
	if end := strings.Index(tag, `"`); end >= 0 {
		tag = tag[:end]
	}
	// Strip options like omitempty
	if comma := strings.Index(tag, ","); comma >= 0 {
		tag = tag[:comma]
	}
	return tag
}
