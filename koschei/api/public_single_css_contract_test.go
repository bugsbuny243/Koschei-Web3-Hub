package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var internalPublicStylesheetRE = regexp.MustCompile(`(?i)<link\b[^>]*\brel=["']stylesheet["'][^>]*\bhref=["'](/css/[^"']+\.css(?:\?[^"']*)?)["'][^>]*>`)

func TestPublicSiteUsesApprovedSurfaceScopedCSSFiles(t *testing.T) {
	files, err := filepath.Glob("public/css/*.css")
	if err != nil {
		t.Fatalf("glob public css: %v", err)
	}
	sort.Strings(files)
	want := []string{
		filepath.FromSlash("public/css/koschei-dashboard.css"),
		filepath.FromSlash("public/css/koschei-home.css"),
		filepath.FromSlash("public/css/koschei.css"),
	}
	if len(files) != len(want) {
		t.Fatalf("public CSS contract drifted: got %v, want %v", files, want)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Fatalf("public CSS contract drifted: got %v, want %v", files, want)
		}
	}

	bundle, err := os.ReadFile(filepath.FromSlash("public/css/koschei.css"))
	if err != nil {
		t.Fatalf("read canonical CSS: %v", err)
	}
	if !strings.Contains(string(bundle), "KOSCHEI WEB3 — canonical public stylesheet") {
		t.Fatal("canonical CSS is missing the consolidation provenance header")
	}

	err = filepath.WalkDir("public", func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".html") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		refs := internalPublicStylesheetRE.FindAllStringSubmatch(string(body), -1)
		if len(refs) > 1 {
			t.Errorf("%s loads %d internal CSS files; want at most one", path, len(refs))
			return nil
		}
		if len(refs) == 0 {
			return nil
		}

		expected := "/css/koschei.css?v=1"
		switch filepath.ToSlash(path) {
		case "public/index.html":
			expected = "/css/koschei-home.css?v=1"
		case "public/dashboard.html":
			expected = "/css/koschei-dashboard.css?v=1"
		}
		if refs[0][1] != expected {
			t.Errorf("%s loads %q; want %q", path, refs[0][1], expected)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk public HTML: %v", err)
	}
}
