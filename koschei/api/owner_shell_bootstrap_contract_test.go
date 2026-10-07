package main

import (
	"os"
	"strings"
	"testing"
)

func TestAuthenticatedOwnerShellDoesNotWaitForOperationsBeforeRendering(t *testing.T) {
	production, err := os.ReadFile("public/owner-production.html")
	if err != nil {
		t.Fatalf("read owner production shell: %v", err)
	}
	body := string(production)
	if strings.Contains(body, `id="appView" class="owner-app hidden"`) {
		t.Fatal("authenticated owner application must not stay hidden while /api/owner/operations is loading")
	}
	if !strings.Contains(body, `id="appView" class="owner-app"`) {
		t.Fatal("authenticated owner application shell is missing")
	}
	if !strings.Contains(body, "Loading owner production data…") {
		t.Fatal("owner shell must expose a visible loading state while production data loads")
	}
	if !strings.Contains(body, `/css/owner-access-gate.css?v=3`) {
		t.Fatal("owner production shell did not bust the access-gate stylesheet cache")
	}
}

func TestOwnerLoginUsesReadableAccessGateStylesheetRevision(t *testing.T) {
	login, err := os.ReadFile("public/owner-login.html")
	if err != nil {
		t.Fatalf("read owner login shell: %v", err)
	}
	if !strings.Contains(string(login), `/css/owner-access-gate.css?v=2`) {
		t.Fatal("owner login shell did not bust the access-gate stylesheet cache")
	}
	styles, err := os.ReadFile("public/css/owner-access-gate.css")
	if err != nil {
		t.Fatalf("read owner access-gate stylesheet: %v", err)
	}
	css := string(styles)
	for _, required := range []string{
		`.owner-login-shell h1`,
		`color:#f5fbfd!important`,
		`.owner-login-shell .muted`,
		`color:#a8bbc2!important`,
	} {
		if !strings.Contains(css, required) {
			t.Fatalf("owner login readability contract missing %q", required)
		}
	}
}
