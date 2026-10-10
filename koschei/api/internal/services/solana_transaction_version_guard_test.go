package services

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSolanaTransactionVersionLiterals prevents version-0 requests from
// silently disabling creator history when Solana v1 transactions appear.
func TestSolanaTransactionVersionLiterals(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil { return walkErr }
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == ".git" { return filepath.SkipDir }
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") { return nil }
		source, err := os.ReadFile(path)
		if err != nil { return err }
		if !strings.Contains(string(source), "maxSupportedTransactionVersion") { return nil }
		file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
		if err != nil { return err }
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.KeyValueExpr)
			if !ok { return true }
			key, ok := lit.Key.(*ast.BasicLit)
			if !ok || key.Kind != token.STRING { return true }
			name, err := strconv.Unquote(key.Value)
			if err != nil || name != "maxSupportedTransactionVersion" { return true }
			value, ok := lit.Value.(*ast.BasicLit)
			if !ok || value.Kind != token.INT { return true }
			n, err := strconv.ParseInt(value.Value, 0, 64)
			if err == nil && n < SolanaMaxSupportedTransactionVersion {
				t.Errorf("%s: hardcoded maxSupportedTransactionVersion=%d is below %d", path, n, SolanaMaxSupportedTransactionVersion)
			}
			return true
		})
		return nil
	})
	if err != nil { t.Fatal(err) }
}
