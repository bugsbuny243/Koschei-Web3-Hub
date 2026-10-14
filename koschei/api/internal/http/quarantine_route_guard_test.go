package http

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Inspect selectors rather than the literal receiver name: handlers may be
// registered through a local alias or a wrapper with a different variable name.
func TestQuarantinedHandlersAreNotRouted(t *testing.T) {
	methods := map[string]string{}
	handlerFiles, err := filepath.Glob("../handlers/*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range handlerFiles {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if file.Doc == nil || !strings.Contains(file.Doc.Text(), "QUARANTINED") {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			receiver := fn.Recv.List[0].Type
			if star, ok := receiver.(*ast.StarExpr); ok {
				receiver = star.X
			}
			if name, ok := receiver.(*ast.Ident); ok && name.Name == "Handler" {
				methods[fn.Name.Name] = path
			}
		}
	}
	if len(methods) == 0 {
		t.Fatal("no quarantined handler methods found")
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok {
				if origin, found := methods[selector.Sel.Name]; found {
					t.Errorf("%s references quarantined %s from %s", path, selector.Sel.Name, origin)
				}
			}
			return true
		})
	}
}

func TestRemovedSyntheticRoutesStayRemoved(t *testing.T) {
	removed := map[string]bool{"/api/mev/analyze": true, "/api/liquidity/analyze": true, "/api/dao/proposal-risk": true}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				value, err := strconv.Unquote(literal.Value)
				if err == nil {
					for _, part := range strings.Fields(value) {
						if removed[part] {
							t.Errorf("%s references removed route %s", path, part)
						}
					}
				}
			}
			return true
		})
	}
}
