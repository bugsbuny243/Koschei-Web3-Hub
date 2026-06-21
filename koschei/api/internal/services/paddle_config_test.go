package services

import (
	"os"
	"reflect"
	"testing"
)

func clearPaddleConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PADDLE_ENV", "PADDLE_ENVIRONMENT", "PADDLE_API_KEY", "PADDLE_SECRET_KEY",
		"PADDLE_CLIENT_TOKEN", "PADDLE_CLIENT_SIDE_TOKEN", "NEXT_PUBLIC_PADDLE_CLIENT_TOKEN", "PUBLIC_PADDLE_CLIENT_TOKEN",
		"PADDLE_WEBHOOK_SECRET", "PADDLE_WEBHOOK_KEY",
		"PADDLE_STARTER_PRICE_ID", "PADDLE_STARTER_PRICE_USD_ID", "PADDLE_PRICE_STARTER_ID",
		"PADDLE_PROFESSIONAL_PRICE_ID", "PADDLE_PROFESSIONAL_PRICE_USD_ID", "PADDLE_BUILDER_PRICE_ID", "PADDLE_PRICE_PROFESSIONAL_ID",
		"PADDLE_ENTERPRISE_PRICE_ID", "PADDLE_ENTERPRISE_PRICE_USD_ID", "PADDLE_STUDIO_PRICE_ID", "PADDLE_PRICE_ENTERPRISE_ID",
		"PUBLIC_APP_URL", "NEXT_PUBLIC_APP_URL", "RAILWAY_STATIC_URL", "RAILWAY_PUBLIC_DOMAIN",
		"PADDLE_CHECKOUT_URL", "PADDLE_DEFAULT_PAYMENT_LINK",
	} {
		t.Setenv(key, "")
	}
}

func TestPaddleConfigCanonicalProductionReady(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_API_KEY", "pdl_live_test")
	t.Setenv("PADDLE_WEBHOOK_SECRET", "pdl_ntfset_test")
	t.Setenv("PADDLE_STARTER_PRICE_ID", "pri_starter")
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_ID", "pri_professional")
	t.Setenv("PADDLE_ENTERPRISE_PRICE_ID", "pri_enterprise")
	t.Setenv("PUBLIC_APP_URL", "https://tradepigloball.co/")

	cfg := LoadPaddleConfigFromEnv()
	if !cfg.Enabled || !cfg.CheckoutReady || !cfg.AutomationReady {
		t.Fatalf("expected fully configured Paddle, got %#v", cfg.PublicStatus())
	}
	if cfg.ClientTokenConfigured {
		t.Fatal("client token must remain optional for server-side checkout")
	}
	if cfg.PublicAppURL != "https://tradepigloball.co" {
		t.Fatalf("unexpected public app URL: %q", cfg.PublicAppURL)
	}
	if len(cfg.MissingFields) != 0 {
		t.Fatalf("unexpected missing fields: %#v", cfg.MissingFields)
	}
}

func TestPaddleConfigUsesRailwayDomainAndAliases(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_API_KEY", "pdl_live_test")
	t.Setenv("PADDLE_WEBHOOK_KEY", "pdl_ntfset_test")
	t.Setenv("PADDLE_STARTER_PRICE_USD_ID", "pri_starter")
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_USD_ID", "pri_professional")
	t.Setenv("PADDLE_ENTERPRISE_PRICE_USD_ID", "pri_enterprise")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "tradepigloball.co")

	cfg := LoadPaddleConfigFromEnv()
	if !cfg.Enabled {
		t.Fatalf("expected Railway/alias configuration to be ready, got %#v", cfg.PublicStatus())
	}
	if cfg.PublicAppURL != "https://tradepigloball.co" {
		t.Fatalf("unexpected Railway public URL: %q", cfg.PublicAppURL)
	}
}

func TestPaddleConfigReportsExactMissingFields(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_API_KEY", "pdl_live_test")
	t.Setenv("PADDLE_WEBHOOK_SECRET", "pdl_ntfset_test")
	t.Setenv("PADDLE_STARTER_PRICE_ID", "pri_starter")
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_ID", "pri_professional")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "tradepigloball.co")

	cfg := LoadPaddleConfigFromEnv()
	if cfg.Enabled || paddleConfigStatus(cfg) != "partial" {
		t.Fatalf("expected partial configuration, got %#v", cfg.PublicStatus())
	}
	want := []string{"PADDLE_ENTERPRISE_PRICE_ID"}
	if !reflect.DeepEqual(cfg.MissingFields, want) {
		t.Fatalf("missing fields=%#v want=%#v", cfg.MissingFields, want)
	}
}

func TestPaddleCheckoutReadyWhileWebhookIncomplete(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_API_KEY", "pdl_live_test")
	t.Setenv("PADDLE_STARTER_PRICE_ID", "pri_starter")
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_ID", "pri_professional")
	t.Setenv("PADDLE_ENTERPRISE_PRICE_ID", "pri_enterprise")
	t.Setenv("PUBLIC_APP_URL", "https://tradepigloball.co")

	cfg := LoadPaddleConfigFromEnv()
	if !cfg.CheckoutReady {
		t.Fatalf("checkout should be ready without webhook: %#v", cfg.PublicStatus())
	}
	if cfg.AutomationReady || cfg.Enabled {
		t.Fatalf("automation must remain disabled without webhook: %#v", cfg.PublicStatus())
	}
	if paddleConfigStatus(cfg) != "checkout_ready_webhook_incomplete" {
		t.Fatalf("unexpected status: %s", paddleConfigStatus(cfg))
	}
	want := []string{"PADDLE_WEBHOOK_SECRET"}
	if !reflect.DeepEqual(cfg.MissingFields, want) {
		t.Fatalf("missing fields=%#v want=%#v", cfg.MissingFields, want)
	}
}

func TestCanonicalizePaddleEnvFeedsLegacyHandlers(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_WEBHOOK_KEY", "pdl_ntfset_test")
	t.Setenv("PADDLE_STARTER_PRICE_USD_ID", "pri_starter")
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_USD_ID", "pri_professional")
	t.Setenv("PADDLE_ENTERPRISE_PRICE_USD_ID", "pri_enterprise")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "tradepigloball.co")

	canonicalizePaddleEnv()
	checks := map[string]string{
		"PADDLE_WEBHOOK_SECRET":        "pdl_ntfset_test",
		"PADDLE_STARTER_PRICE_ID":      "pri_starter",
		"PADDLE_PROFESSIONAL_PRICE_ID": "pri_professional",
		"PADDLE_ENTERPRISE_PRICE_ID":   "pri_enterprise",
		"PUBLIC_APP_URL":               "https://tradepigloball.co",
	}
	for key, want := range checks {
		if got := os.Getenv(key); got != want {
			t.Fatalf("%s=%q want=%q", key, got, want)
		}
	}
}
