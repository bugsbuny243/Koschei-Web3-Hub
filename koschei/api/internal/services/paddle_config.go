package services

import (
	"os"
	"strings"
)

func init() {
	canonicalizePaddleEnv()
}

func canonicalizePaddleEnv() {
	setPaddleCanonicalEnv("PADDLE_API_KEY", firstPaddleEnv(
		"PADDLE_API_KEY",
		"PADDLE_SECRET_KEY",
		"PADDLE_API_TOKEN",
		"PADDLE_TOKEN",
	))
	setPaddleCanonicalEnv("PADDLE_CLIENT_TOKEN", firstPaddleEnv(
		"PADDLE_CLIENT_TOKEN",
		"PADDLE_CLIENT_SIDE_TOKEN",
		"PADDLE_CLIENTSIDE_TOKEN",
		"NEXT_PUBLIC_PADDLE_CLIENT_TOKEN",
		"PUBLIC_PADDLE_CLIENT_TOKEN",
		"VITE_PADDLE_CLIENT_TOKEN",
	))
	setPaddleCanonicalEnv("PADDLE_WEBHOOK_SECRET", firstPaddleEnv(
		"PADDLE_WEBHOOK_SECRET",
		"PADDLE_WEBHOOK_KEY",
		"PADDLE_WEBHOOK_SECRET_KEY",
		"PADDLE_NOTIFICATION_SECRET",
		"PADDLE_ENDPOINT_SECRET",
	))
	setPaddleCanonicalEnv("PADDLE_ENV", firstPaddleEnv(
		"PADDLE_ENV",
		"PADDLE_ENVIRONMENT",
		"PADDLE_MODE",
	))

	setPaddleCanonicalEnv("PADDLE_STARTER_PRICE_ID", firstPaddleEnv(
		"PADDLE_STARTER_PRICE_ID",
		"PADDLE_STARTER_PRICE_USD_ID",
		"PADDLE_PRICE_STARTER_ID",
		"PADDLE_PRICE_ID_STARTER",
	))
	setPaddleCanonicalEnv("PADDLE_PROFESSIONAL_PRICE_ID", firstPaddleEnv(
		"PADDLE_PROFESSIONAL_PRICE_ID",
		"PADDLE_PROFESSIONAL_PRICE_USD_ID",
		"PADDLE_BUILDER_PRICE_ID",
		"PADDLE_PRO_PRICE_ID",
		"PADDLE_PRICE_PROFESSIONAL_ID",
		"PADDLE_PRICE_PRO_ID",
		"PADDLE_PRICE_ID_PROFESSIONAL",
	))
	setPaddleCanonicalEnv("PADDLE_ENTERPRISE_PRICE_ID", firstPaddleEnv(
		"PADDLE_ENTERPRISE_PRICE_ID",
		"PADDLE_ENTERPRISE_PRICE_USD_ID",
		"PADDLE_STUDIO_PRICE_ID",
		"PADDLE_PRICE_ENTERPRISE_ID",
		"PADDLE_PRICE_ID_ENTERPRISE",
	))

	setPaddleCanonicalEnv("PADDLE_STARTER_PRODUCT_ID", firstPaddleEnv(
		"PADDLE_STARTER_PRODUCT_ID",
		"PADDLE_PRODUCT_STARTER_ID",
		"PADDLE_PRODUCT_ID_STARTER",
	))
	setPaddleCanonicalEnv("PADDLE_PROFESSIONAL_PRODUCT_ID", firstPaddleEnv(
		"PADDLE_PROFESSIONAL_PRODUCT_ID",
		"PADDLE_BUILDER_PRODUCT_ID",
		"PADDLE_PRO_PRODUCT_ID",
		"PADDLE_PRODUCT_PROFESSIONAL_ID",
		"PADDLE_PRODUCT_ID_PROFESSIONAL",
	))
	setPaddleCanonicalEnv("PADDLE_ENTERPRISE_PRODUCT_ID", firstPaddleEnv(
		"PADDLE_ENTERPRISE_PRODUCT_ID",
		"PADDLE_STUDIO_PRODUCT_ID",
		"PADDLE_PRODUCT_ENTERPRISE_ID",
		"PADDLE_PRODUCT_ID_ENTERPRISE",
	))

	setPaddleCanonicalEnv("PUBLIC_APP_URL", resolvePaddlePublicAppURL())
	setPaddleCanonicalEnv("PADDLE_CHECKOUT_URL", resolvePaddleCheckoutURL())
}

func setPaddleCanonicalEnv(key, value string) {
	if paddleEnvValue(os.Getenv(key)) != "" || strings.TrimSpace(value) == "" {
		return
	}
	_ = os.Setenv(key, strings.TrimSpace(value))
}

type PaddleConfig struct {
	Enabled                       bool     `json:"enabled"`
	CheckoutReady                 bool     `json:"checkout_ready"`
	AutomationReady               bool     `json:"automation_ready"`
	AllPlansReady                 bool     `json:"all_plans_ready"`
	ConfiguredPlanCount           int      `json:"configured_plan_count"`
	Environment                   string   `json:"environment"`
	APIKeyConfigured              bool     `json:"api_key_configured"`
	ClientTokenConfigured         bool     `json:"client_token_configured"`
	WebhookConfigured             bool     `json:"webhook_configured"`
	APIKey                        string   `json:"-"`
	ClientToken                   string   `json:"-"`
	WebhookSecret                 string   `json:"-"`
	StarterPriceID                string   `json:"-"`
	ProfessionalPriceID           string   `json:"-"`
	EnterprisePriceID             string   `json:"-"`
	StarterProductID              string   `json:"-"`
	ProfessionalProductID         string   `json:"-"`
	EnterpriseProductID           string   `json:"-"`
	PublicAppURL                  string   `json:"-"`
	CheckoutURL                   string   `json:"-"`
	MissingFields                 []string `json:"-"`
}

func LoadPaddleConfigFromEnv() PaddleConfig {
	canonicalizePaddleEnv()
	env := strings.ToLower(strings.TrimSpace(firstPaddleEnv("PADDLE_ENV", "PADDLE_ENVIRONMENT", "PADDLE_MODE")))
	if env != "sandbox" {
		env = "production"
	}
	cfg := PaddleConfig{
		Environment: env,
		APIKey: firstPaddleEnv(
			"PADDLE_API_KEY",
			"PADDLE_SECRET_KEY",
			"PADDLE_API_TOKEN",
			"PADDLE_TOKEN",
		),
		ClientToken: firstPaddleEnv(
			"PADDLE_CLIENT_TOKEN",
			"PADDLE_CLIENT_SIDE_TOKEN",
			"PADDLE_CLIENTSIDE_TOKEN",
			"NEXT_PUBLIC_PADDLE_CLIENT_TOKEN",
			"PUBLIC_PADDLE_CLIENT_TOKEN",
			"VITE_PADDLE_CLIENT_TOKEN",
		),
		WebhookSecret: firstPaddleEnv(
			"PADDLE_WEBHOOK_SECRET",
			"PADDLE_WEBHOOK_KEY",
			"PADDLE_WEBHOOK_SECRET_KEY",
			"PADDLE_NOTIFICATION_SECRET",
			"PADDLE_ENDPOINT_SECRET",
		),
		StarterPriceID: firstPaddleEnv(
			"PADDLE_STARTER_PRICE_ID",
			"PADDLE_STARTER_PRICE_USD_ID",
			"PADDLE_PRICE_STARTER_ID",
			"PADDLE_PRICE_ID_STARTER",
		),
		ProfessionalPriceID: firstPaddleEnv(
			"PADDLE_PROFESSIONAL_PRICE_ID",
			"PADDLE_PROFESSIONAL_PRICE_USD_ID",
			"PADDLE_BUILDER_PRICE_ID",
			"PADDLE_PRO_PRICE_ID",
			"PADDLE_PRICE_PROFESSIONAL_ID",
			"PADDLE_PRICE_PRO_ID",
			"PADDLE_PRICE_ID_PROFESSIONAL",
		),
		EnterprisePriceID: firstPaddleEnv(
			"PADDLE_ENTERPRISE_PRICE_ID",
			"PADDLE_ENTERPRISE_PRICE_USD_ID",
			"PADDLE_STUDIO_PRICE_ID",
			"PADDLE_PRICE_ENTERPRISE_ID",
			"PADDLE_PRICE_ID_ENTERPRISE",
		),
		StarterProductID: firstPaddleEnv(
			"PADDLE_STARTER_PRODUCT_ID",
			"PADDLE_PRODUCT_STARTER_ID",
			"PADDLE_PRODUCT_ID_STARTER",
		),
		ProfessionalProductID: firstPaddleEnv(
			"PADDLE_PROFESSIONAL_PRODUCT_ID",
			"PADDLE_BUILDER_PRODUCT_ID",
			"PADDLE_PRO_PRODUCT_ID",
			"PADDLE_PRODUCT_PROFESSIONAL_ID",
			"PADDLE_PRODUCT_ID_PROFESSIONAL",
		),
		EnterpriseProductID: firstPaddleEnv(
			"PADDLE_ENTERPRISE_PRODUCT_ID",
			"PADDLE_STUDIO_PRODUCT_ID",
			"PADDLE_PRODUCT_ENTERPRISE_ID",
			"PADDLE_PRODUCT_ID_ENTERPRISE",
		),
		PublicAppURL: resolvePaddlePublicAppURL(),
		CheckoutURL:  resolvePaddleCheckoutURL(),
	}

	cfg.APIKeyConfigured = cfg.APIKey != ""
	cfg.ClientTokenConfigured = cfg.ClientToken != ""
	cfg.WebhookConfigured = cfg.WebhookSecret != ""
	cfg.ConfiguredPlanCount = paddleConfiguredPlanCount(cfg)
	cfg.AllPlansReady = cfg.ConfiguredPlanCount == 3

	// Transactions are created server-side. Product IDs and a client token are
	// useful metadata, but neither is required to create Paddle transactions.
	cfg.CheckoutReady = cfg.APIKeyConfigured && cfg.PublicAppURL != "" && cfg.ConfiguredPlanCount > 0
	cfg.AutomationReady = cfg.CheckoutReady && cfg.WebhookConfigured
	cfg.Enabled = cfg.AutomationReady
	cfg.MissingFields = paddleMissingFields(cfg)
	return cfg
}

func (c PaddleConfig) PriceID(plan string) string {
	switch normalizePaddlePlan(plan) {
	case "starter":
		return c.StarterPriceID
	case "professional":
		return c.ProfessionalPriceID
	case "enterprise":
		return c.EnterprisePriceID
	default:
		return ""
	}
}

func (c PaddleConfig) ProductID(plan string) string {
	switch normalizePaddlePlan(plan) {
	case "starter":
		return c.StarterProductID
	case "professional":
		return c.ProfessionalProductID
	case "enterprise":
		return c.EnterpriseProductID
	default:
		return ""
	}
}

func (c PaddleConfig) PlanReady(plan string) bool {
	return c.APIKeyConfigured && c.PublicAppURL != "" && c.PriceID(plan) != ""
}

func (c PaddleConfig) PublicStatus() map[string]any {
	return map[string]any{
		"configured":                     c.Enabled,
		"status":                         paddleConfigStatus(c),
		"environment":                    c.Environment,
		"checkout_ready":                 c.CheckoutReady,
		"automation_ready":               c.AutomationReady,
		"all_plans_ready":                c.AllPlansReady,
		"configured_plan_count":          c.ConfiguredPlanCount,
		"api_key_configured":             c.APIKeyConfigured,
		"client_token_configured":        c.ClientTokenConfigured,
		"client_token_required":          false,
		"webhook_configured":             c.WebhookConfigured,
		"starter_price_configured":       c.StarterPriceID != "",
		"professional_price_configured":  c.ProfessionalPriceID != "",
		"enterprise_price_configured":    c.EnterprisePriceID != "",
		"starter_product_configured":     c.StarterProductID != "",
		"professional_product_configured": c.ProfessionalProductID != "",
		"enterprise_product_configured":  c.EnterpriseProductID != "",
		"product_ids_required":           false,
		"public_app_url_configured":      c.PublicAppURL != "",
		"checkout_url_configured":        c.CheckoutURL != "",
		"starter_ready":                  c.PlanReady("starter"),
		"professional_ready":             c.PlanReady("professional"),
		"enterprise_ready":               c.PlanReady("enterprise"),
		"missing_fields":                 append([]string(nil), c.MissingFields...),
	}
}

func resolvePaddlePublicAppURL() string {
	value := strings.TrimSpace(firstPaddleEnv(
		"PUBLIC_APP_URL",
		"NEXT_PUBLIC_APP_URL",
		"APP_URL",
		"BASE_URL",
		"PUBLIC_URL",
		"RAILWAY_STATIC_URL",
	))
	if value == "" {
		if domain := paddleEnvValue(os.Getenv("RAILWAY_PUBLIC_DOMAIN")); domain != "" {
			if !strings.Contains(domain, "://") {
				value = "https://" + domain
			} else {
				value = domain
			}
		}
	}
	return strings.TrimRight(value, "/")
}

func resolvePaddleCheckoutURL() string {
	if value := strings.TrimSpace(firstPaddleEnv(
		"PADDLE_CHECKOUT_URL",
		"PADDLE_DEFAULT_PAYMENT_LINK",
		"PADDLE_PAYMENT_LINK",
	)); value != "" {
		return strings.TrimRight(value, "/")
	}
	if appURL := resolvePaddlePublicAppURL(); appURL != "" {
		return appURL + "/pricing"
	}
	return ""
}

func paddleConfiguredPlanCount(c PaddleConfig) int {
	count := 0
	if c.StarterPriceID != "" {
		count++
	}
	if c.ProfessionalPriceID != "" {
		count++
	}
	if c.EnterprisePriceID != "" {
		count++
	}
	return count
}

func paddleMissingFields(c PaddleConfig) []string {
	missing := []string{}
	if !c.APIKeyConfigured {
		missing = append(missing, "PADDLE_API_KEY")
	}
	if !c.WebhookConfigured {
		missing = append(missing, "PADDLE_WEBHOOK_SECRET")
	}
	if c.StarterPriceID == "" {
		missing = append(missing, "PADDLE_STARTER_PRICE_ID")
	}
	if c.ProfessionalPriceID == "" {
		missing = append(missing, "PADDLE_PROFESSIONAL_PRICE_ID")
	}
	if c.EnterprisePriceID == "" {
		missing = append(missing, "PADDLE_ENTERPRISE_PRICE_ID")
	}
	if c.PublicAppURL == "" {
		missing = append(missing, "PUBLIC_APP_URL")
	}
	return missing
}

func paddleConfigStatus(c PaddleConfig) string {
	if c.AutomationReady && c.AllPlansReady {
		return "configured"
	}
	if c.AutomationReady {
		return "configured_partial_catalog"
	}
	if c.CheckoutReady {
		return "checkout_ready_webhook_incomplete"
	}
	if c.APIKeyConfigured && c.WebhookConfigured && c.ConfiguredPlanCount == 0 {
		return "credentials_ready_catalog_incomplete"
	}
	if c.APIKeyConfigured || c.ClientTokenConfigured || c.WebhookConfigured || c.ConfiguredPlanCount > 0 || c.PublicAppURL != "" {
		return "partial"
	}
	return "not_configured"
}

func normalizePaddlePlan(plan string) string {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "starter":
		return "starter"
	case "professional", "builder", "pro":
		return "professional"
	case "enterprise", "studio":
		return "enterprise"
	default:
		return ""
	}
}

func firstPaddleEnv(keys ...string) string {
	for _, key := range keys {
		if value := paddleEnvValue(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func paddleEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			value = strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return value
}
