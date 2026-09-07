package http

import (
	"os"
	"strings"
	"testing"
)

func TestInvestigationShareUsesUserReviewedXIntent(t *testing.T) {
	share := mustReadShareFixture(t, "../../public/js/investigation-share.js")
	for _, required := range []string{
		"https://x.com/intent/tweet",
		"window.open",
		"publicResultURL",
		"evidence_pending",
		"Eksik kanıt güvenli sayılmaz",
		"installRadarShare",
	} {
		if !strings.Contains(share, required) {
			t.Fatalf("investigation share module is missing %q", required)
		}
	}
	for _, forbidden := range []string{"/2/tweets", "Authorization", "Bearer ", "access_token", "api.x.com"} {
		if strings.Contains(share, forbidden) {
			t.Fatalf("investigation share module must not auto-post or hold X credentials: %q", forbidden)
		}
	}
}

func mustReadShareFixture(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
