package daemon

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Every Method constant in package proto must have a handler, or clients
// calling it get "unknown method" from a daemon that should know it.
func TestEveryMethodHasHandler(t *testing.T) {
	files, err := filepath.Glob("../proto/*.go")
	if err != nil {
		t.Fatal(err)
	}
	methods := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, isValue := n.(*ast.ValueSpec)
			if !isValue {
				return true
			}
			for i, name := range spec.Names {
				if !strings.HasPrefix(name.Name, "Method") || i >= len(spec.Values) {
					continue
				}
				lit, isLit := spec.Values[i].(*ast.BasicLit)
				if !isLit {
					continue
				}
				method, _ := strconv.Unquote(lit.Value)
				methods++
				if _, found := handlers[method]; !found {
					t.Errorf("proto.%s (%q) has no handler", name.Name, method)
				}
			}
			return true
		})
	}
	if methods != len(handlers) {
		t.Errorf("proto declares %d methods, daemon handles %d", methods, len(handlers))
	}
}
