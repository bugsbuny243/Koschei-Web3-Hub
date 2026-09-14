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

	approved := map[string]bool{}
	for _, file := range want {
		approved[filepath.Base(file)] = true
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
		for _, ref := range refs {
			name := filepath.Base(strings.SplitN(ref[1], "?", 2)[0])
			if !approved[name] {
				t.Errorf("%s loads unapproved public CSS %q", path, ref[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk public HTML: %v", err)
	}

	indexBody, err := os.ReadFile("public/index.html")
	if err != nil {
		t.Fatalf("read homepage: %v", err)
	}
	if !strings.Contains(string(indexBody), `/css/koschei-home.css?v=2`) {
		t.Fatal("homepage is not pinned to the current home stylesheet")
	}
	dashboardBody, err := os.ReadFile("public/dashboard.html")
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	for _, href := range []string{`/css/koschei-dashboard.css?v=2`, `/css/koschei-dashboard-premium.css?v=2`} {
		if !strings.Contains(string(dashboardBody), href) {
			t.Fatalf("dashboard missing current stylesheet %q", href)
		}
	}
}
