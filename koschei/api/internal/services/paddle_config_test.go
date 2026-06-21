package services

import (
	"os"
	"reflect"
	"testing"
)

func clearPaddleConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PADDLE_ENV", "PADDLE_ENVIRONMENT", "PADDLE_MODE",
		"PADDLE_API_KEY", "PADDLE_SECRET_KEY", "PADDLE_API_TOKEN", "PADDLE_TOKEN",
		"PADDLE_CLIENT_TOKEN", "PADDLE_CLIENT_SIDE_TOKEN", "PADDLE_CLIENTSIDE_TOKEN", "NEXT_PUBLIC_PADDLE_CLIENT_TOKEN", "PUBLIC_PADDLE_CLIENT_TOKEN", "VITE_PADDLE_CLIENT_TOKEN",
		"PADDLE_WEBHOOK_SECRET", "PADDLE_WEBHOOK_KEY", "PADDLE_WEBHOOK_SECRET_KEY", "PADDLE_NOTIFICATION_SECRET", "PADDLE_ENDPOINT_SECRET",
		"PADDLE_STARTER_PRICE_ID", "PADDLE_STARTER_PRICE_USD_ID", "PADDLE_PRICE_STARTER_ID", "PADDLE_PRICE_ID_STARTER",
		"PADDLE_PROFESSIONAL_PRICE_ID", "PADDLE_PROFESSIONAL_PRICE_USD_ID", "PADDLE_BUILDER_PRICE_ID", "PADDLE_PRO_PRICE_ID", "PADDLE_PRICE_PROFESSIONAL_ID", "PADDLE_PRICE_PRO_ID", "PADDLE_PRICE_ID_PROFESSIONAL",
		"PADDLE_ENTERPRISE_PRICE_ID", "PADDLE_ENTERPRISE_PRICE_USD_ID", "PADDLE_STUDIO_PRICE_ID", "PADDLE_PRICE_ENTERPRISE_ID", "PADDLE_PRICE_ID_ENTERPRISE",
		"PADDLE_STARTER_PRODUCT_ID", "PADDLE_PRODUCT_STARTER_ID", "PADDLE_PRODUCT_ID_STARTER",
		"PADDLE_PROFESSIONAL_PRODUCT_ID", "PADDLE_BUILDER_PRODUCT_ID", "PADDLE_PRO_PRODUCT_ID", "PADDLE_PRODUCT_PROFESSIONAL_ID", "PADDLE_PRODUCT_ID_PROFESSIONAL",
		"PADDLE_ENTERPRISE_PRODUCT_ID", "PADDLE_STUDIO_PRODUCT_ID", "PADDLE_PRODUCT_ENTERPRISE_ID", "PADDLE_PRODUCT_ID_ENTERPRISE",
		"PUBLIC_APP_URL", "NEXT_PUBLIC_APP_URL", "APP_URL", "BASE_URL", "PUBLIC_URL", "RAILWAY_STATIC_URL", "RAILWAY_PUBLIC_DOMAIN",
		"PADDLE_CHECKOUT_URL", "PADDLE_DEFAULT_PAYMENT_LINK", "PADDLE_PAYMENT_LINK",
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
	if !cfg.Enabled || !cfg.CheckoutReady || !cfg.AutomationReady || !cfg.AllPlansReady {
		t.Fatalf("expected fully configured Paddle, got %#v", cfg.PublicStatus())
	}
	if cfg.ConfiguredPlanCount != 3 {
		t.Fatalf("configured plan count=%d want=3", cfg.ConfiguredPlanCount)
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
	if paddleConfigStatus(cfg) != "configured" {
		t.Fatalf("unexpected status: %s", paddleConfigStatus(cfg))
	}
}

func TestPaddleConfigUsesRailwayDomainAndAliases(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_API_TOKEN", "pdl_live_test")
	t.Setenv("PADDLE_ENDPOINT_SECRET", "pdl_ntfset_test")
	t.Setenv("PADDLE_PRICE_ID_STARTER", "pri_starter")
	t.Setenv("PADDLE_PRICE_ID_PROFESSIONAL", "pri_professional")
	t.Setenv("PADDLE_PRICE_ID_ENTERPRISE", "pri_enterprise")
	t.Setenv("PADDLE_MODE", "production")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "tradepigloball.co")

	cfg := LoadPaddleConfigFromEnv()
	if !cfg.Enabled || !cfg.AllPlansReady {
		t.Fatalf("expected Railway/alias configuration to be ready, got %#v", cfg.PublicStatus())
	}
	if cfg.PublicAppURL != "https://tradepigloball.co" {
		t.Fatalf("unexpected Railway public URL: %q", cfg.PublicAppURL)
	}
	if got := os.Getenv("PADDLE_API_KEY"); got != "pdl_live_test" {
		t.Fatalf("canonical API key was not populated: %q", got)
	}
	if got := os.Getenv("PADDLE_WEBHOOK_SECRET"); got != "pdl_ntfset_test" {
		t.Fatalf("canonical webhook secret was not populated: %q", got)
	}
}

func TestPaddleConfigKeepsAutomationReadyWithPartialCatalog(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_API_KEY", "pdl_live_test")
	t.Setenv("PADDLE_WEBHOOK_SECRET", "pdl_ntfset_test")
	t.Setenv("PADDLE_STARTER_PRICE_ID", "pri_starter")
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_ID", "pri_professional")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "tradepigloball.co")

	cfg := LoadPaddleConfigFromEnv()
	if !cfg.Enabled || !cfg.AutomationReady || cfg.AllPlansReady {
		t.Fatalf("expected automated Paddle with partial catalog, got %#v", cfg.PublicStatus())
	}
	if paddleConfigStatus(cfg) != "configured_partial_catalog" {
		t.Fatalf("unexpected status: %s", paddleConfigStatus(cfg))
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

func TestPaddleConfigDoesNotRequireClientTokenOrProductIDs(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_API_KEY", "api")
	t.Setenv("PADDLE_WEBHOOK_SECRET", "webhook")
	t.Setenv("PADDLE_STARTER_PRICE_ID", "pri_starter")
	t.Setenv("PUBLIC_APP_URL", "https://tradepigloball.co")

	cfg := LoadPaddleConfigFromEnv()
	if !cfg.Enabled || !cfg.PlanReady("starter") {
		t.Fatalf("server-side Paddle checkout should be ready: %#v", cfg.PublicStatus())
	}
	if cfg.ClientTokenConfigured || cfg.StarterProductID != "" {
		t.Fatalf("client token and product IDs must remain optional: %#v", cfg.PublicStatus())
	}
}

func TestPaddleConfigStripsWrappedQuotes(t *testing.T) {
	clearPaddleConfigEnv(t)
	t.Setenv("PADDLE_API_KEY", `"api"`)
	t.Setenv("PADDLE_WEBHOOK_SECRET", `'webhook'`)
	t.Setenv("PADDLE_STARTER_PRICE_ID", `"pri_starter"`)
	t.Setenv("PADDLE_PROFESSIONAL_PRICE_ID", `"pri_professional"`)
	t.Setenv("PADDLE_ENTERPRISE_PRICE_ID", `"pri_enterprise"`)
	t.Setenv("PUBLIC_APP_URL", `"https://tradepigloball.co/"`)

	cfg := LoadPaddleConfigFromEnv()
	if cfg.APIKey != "api" || cfg.WebhookSecret != "webhook" {
		t.Fatalf("quoted credentials were not normalized")
	}
	if cfg.PublicAppURL != "https://tradepigloball.co" {
		t.Fatalf("quoted public app URL was not normalized: %q", cfg.PublicAppURL)
	}
	if !cfg.Enabled {
		t.Fatalf("quoted Railway values should still configure Paddle: %#v", cfg.PublicStatus())
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
