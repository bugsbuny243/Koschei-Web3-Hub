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
		filepath.FromSlash("public/css/customer-universal-address-scan-v1.css"),
		filepath.FromSlash("public/css/evm-authority-desk.css"),
		filepath.FromSlash("public/css/koschei-dashboard-premium.css"),
		filepath.FromSlash("public/css/koschei-dashboard.css"),
		filepath.FromSlash("public/css/koschei-home.css"),
		filepath.FromSlash("public/css/koschei-scan-premium.css"),
		filepath.FromSlash("public/css/koschei.css"),
		filepath.FromSlash("public/css/premium-evidence-matrix.css"),
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

	approved := map[string]struct{}{}
	for _, path := range want {
		approved["/css/"+filepath.Base(path)] = struct{}{}
	}
	requiredByPage := map[string][]string{
		"public/index.html": {
			"/css/koschei-home.css?v=2",
		},
		"public/dashboard.html": {
			"/css/koschei-dashboard.css?v=2",
			"/css/koschei-dashboard-premium.css?v=2",
		},
		"public/scan.html": {
			"/css/koschei.css?v=1",
			"/css/koschei-scan-premium.css?v=2",
			"/css/premium-evidence-matrix.css?v=1",
			"/css/evm-authority-desk.css?v=1",
			"/css/customer-universal-address-scan-v1.css?v=2",
		},
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
		seen := map[string]struct{}{}
		for _, ref := range refs {
			href := ref[1]
			cssPath := strings.SplitN(href, "?", 2)[0]
			if _, ok := approved[cssPath]; !ok {
				t.Errorf("%s loads unapproved internal CSS %q", path, href)
			}
			if _, duplicate := seen[href]; duplicate {
				t.Errorf("%s loads duplicate stylesheet %q", path, href)
			}
			seen[href] = struct{}{}
		}
		for _, required := range requiredByPage[filepath.ToSlash(path)] {
			if _, ok := seen[required]; !ok {
				t.Errorf("%s missing required scoped stylesheet %q", path, required)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk public HTML: %v", err)
	}
}
