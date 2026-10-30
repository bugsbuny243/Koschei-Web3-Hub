package http

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The quarantined synthetic analysis routes must not silently return through
// public HTML/JS after an unrelated UI refresh. Registered replacement routes
// are tested separately by the HTTP route inventory contract.
func TestPublicFrontendDoesNotCallQuarantinedLegacyAPIs(t *testing.T) {
	retired := []string{
		"/api/mev/analyze",
		"/api/liquidity/analyze",
		"/api/dao/proposal-risk",
		"/api/owner/kosch-access",
	}
	root := filepath.Join("..", "..", "public")
	checked := 0
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "__tests__" || entry.Name() == "__fixtures__" {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".html") && !strings.HasSuffix(name, ".js") {
			return nil
		}
		if strings.HasSuffix(name, ".test.js") {
			return nil
		}
		contents, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		checked++
		for _, route := range retired {
			if strings.Contains(string(contents), route) {
				t.Errorf("%s references quarantined API %s", filename, route)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no public HTML/JS inspected")
	}
}
