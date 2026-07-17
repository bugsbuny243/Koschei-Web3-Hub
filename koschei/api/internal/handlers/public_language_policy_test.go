package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserFacingPagesAvoidLegalFraming(t *testing.T) {
	root := filepath.Join("..", "..", "public")
	files := []string{
		"index.html",
		"dashboard.html",
		"scan.html",
		"owner-production.html",
		filepath.Join("js", "owner-court-ui.js"),
		filepath.Join("js", "owner-command-center-v2.js"),
		filepath.Join("js", "owner-unified-radar.js"),
	}
	forbidden := []string{
		"mahkeme",
		"savcı",
		"savcılık",
		"dava dosyası",
		"yeni dava",
		"heyet",
		"tribunal",
		"prosecutor review",
		"court layer",
		"court file",
		"read-only court",
	}
	for _, name := range files {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		content := strings.ToLower(string(raw))
		for _, phrase := range forbidden {
			if strings.Contains(content, strings.ToLower(phrase)) {
				t.Errorf("user-facing file %s contains forbidden legal framing %q", name, phrase)
			}
		}
	}
}
