package main

import (
	"os"
	"strings"
	"testing"
)

func TestGlobalShellProducesEnglishNavigationAndMessages(t *testing.T) {
	body, err := os.ReadFile("public/js/koschei-global-shell.js")
	if err != nil {
		t.Fatalf("read global shell: %v", err)
	}
	text := string(body)
	for _, required := range []string{
		"['/','Home']",
		"['/dashboard','Customer Panel']",
		"['/live','Live SOC']",
		"['/cases','Cases']",
		"nav.setAttribute('aria-label','Main navigation')",
		"document.documentElement.lang='en'",
		"The evidence service did not respond within",
		"DEGRADED DEPENDENCY — The evidence service did not respond within",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("global shell missing English contract %q", required)
		}
	}
	for _, forbidden := range []string{
		"document.documentElement.lang='tr'",
		"['/scan','Token Scan']",
		"['/transaction-shield','Transaction Shield']",
		"['/safe-check','Safe Check']",
		"['/scan?mode=deep','Deep Scan']",
		"nav.setAttribute('aria-label','Ana menü')",
		"run.textContent='Kontrol ediliyor…'",
		"Koschei ARVIS · Solana güvenlik merkezi</span>",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("global shell contains retired or Turkish UI contract %q", forbidden)
		}
	}
}
