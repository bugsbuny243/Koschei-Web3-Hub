package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveStaticDirPrefersRepoPublicDirectory(t *testing.T) {
	root := t.TempDir()
	publicDir := filepath.Join(root, "koschei", "api", "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("create repo public dir: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	if got := resolveStaticDir(""); got != filepath.Join("koschei", "api", "public") {
		t.Fatalf("resolveStaticDir() = %q, want koschei/api/public", got)
	}
}

func TestResolveStaticDirHonorsConfiguredPath(t *testing.T) {
	if got := resolveStaticDir("/custom/public"); got != "/custom/public" {
		t.Fatalf("resolveStaticDir(configured) = %q, want /custom/public", got)
	}
}
