package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveStaticDirPrefersLocalPublicDirectory(t *testing.T) {
	root := t.TempDir()
	publicDir := filepath.Join(root, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("create local public dir: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change working directory: %v", err)
	}

	if got := resolveStaticDir(""); got != "public" {
		t.Fatalf("resolveStaticDir() = %q, want public", got)
	}
}

func TestResolveStaticDirHonorsConfiguredPath(t *testing.T) {
	if got := resolveStaticDir("/custom/public"); got != "/custom/public" {
		t.Fatalf("resolveStaticDir(configured) = %q, want /custom/public", got)
	}
}
