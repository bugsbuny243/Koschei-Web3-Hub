package outbound

import (
	"context"
	"net/http"
	"testing"
)

func TestValidateOperatorURLRejectsPrivateProductionTargets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	for _, raw := range []string{
		"http://127.0.0.1:8080",
		"https://127.0.0.1:8443",
		"https://10.0.0.1/rpc",
		"https://192.168.1.2/rpc",
		"https://169.254.1.1/meta",
		"file:///etc/passwd",
		"https://user:pass@example.com/rpc",
	} {
		if _, err := ValidateOperatorURL(context.Background(), raw); err == nil {
			t.Fatalf("expected rejection for %q", raw)
		}
	}
}

func TestValidateOperatorURLAllowsLoopbackOnlyOutsideProduction(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	if _, err := ValidateOperatorURL(context.Background(), "http://127.0.0.1:8080/rpc"); err != nil {
		t.Fatalf("test loopback should remain available: %v", err)
	}
}

func TestValidateFixedHTTPSHost(t *testing.T) {
	if _, err := ValidateFixedHTTPSHost("https://api.github.com/repos/openai/openai", "api.github.com"); err != nil {
		t.Fatalf("expected allowed fixed host: %v", err)
	}
	for _, raw := range []string{
		"http://api.github.com/repos/openai/openai",
		"https://127.0.0.1/repos/openai/openai",
		"https://evil.example/repos/openai/openai",
	} {
		if _, err := ValidateFixedHTTPSHost(raw, "api.github.com"); err == nil {
			t.Fatalf("expected fixed-host rejection for %q", raw)
		}
	}
}

func TestHardenOperatorClientRejectsRedirectToPrivateProductionTarget(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	client := HardenOperatorClient(context.Background(), &http.Client{})
	req, err := http.NewRequest(http.MethodGet, "https://127.0.0.1/internal", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(req, []*http.Request{{}}); err == nil {
		t.Fatal("expected private redirect rejection")
	}
}
