package services

import (
	"os"
	"strings"
)

func init() {
	canonicalizePaddleEnv()
}

func canonicalizePaddleEnv() {
	setPaddleCanonicalEnv("PADDLE_API_KEY", firstPaddleEnv("PADDLE_API_KEY", "PADDLE_SECRET_KEY"))
	setPaddleCanonicalEnv("PADDLE_CLIENT_TOKEN", firstPaddleEnv("PADDLE_CLIENT_TOKEN", "PADDLE_CLIENT_SIDE_TOKEN", "NEXT_PUBLIC_PADDLE_CLIENT_TOKEN", "PUBLIC_PADDLE_CLIENT_TOKEN"))
	setPaddleCanonicalEnv("PADDLE_WEBHOOK_SECRET", firstPaddleEnv("PADDLE_WEBHOOK_SECRET", "PADDLE_WEBHOOK_KEY"))
	setPaddleCanonicalEnv("PADDLE_STARTER_PRICE_ID", firstPaddleEnv("PADDLE_STARTER_PRICE_ID", "PADDLE_STARTER_PRICE_USD_ID", "PADDLE_PRICE_STARTER_ID"))
	setPaddleCanonicalEnv("PADDLE_PROFESSIONAL_PRICE_ID", firstPaddleEnv("PADDLE_PROFESSIONAL_PRICE_ID", "PADDLE_PROFESSIONAL_PRICE_USD_ID", "PADDLE_BUILDER_PRICE_ID", "PADDLE_PRICE_PROFESSIONAL_ID"))
	setPaddleCanonicalEnv("PADDLE_ENTERPRISE_PRICE_ID", firstPaddleEnv("PADDLE_ENTERPRISE_PRICE_ID", "PADDLE_ENTERPRISE_PRICE_USD_ID", "PADDLE_STUDIO_PRICE_ID", "PADDLE_PRICE_ENTERPRISE_ID"))
	setPaddleCanonicalEnv("PADDLE_STARTER_PRODUCT_ID", firstPaddleEnv("PADDLE_STARTER_PRODUCT_ID", "PADDLE_PRODUCT_STARTER_ID"))
	setPaddleCanonicalEnv("PADDLE_PROFESSIONAL_PRODUCT_ID", firstPaddleEnv("PADDLE_PROFESSIONAL_PRODUCT_ID", "PADDLE_BUILDER_PRODUCT_ID", "PADDLE_PRODUCT_PROFESSIONAL_ID"))
	setPaddleCanonicalEnv("PADDLE_ENTERPRISE_PRODUCT_ID", firstPaddleEnv("PADDLE_ENTERPRISE_PRODUCT_ID", "PADDLE_STUDIO_PRODUCT_ID", "PADDLE_PRODUCT_ENTERPRISE_ID"))
	setPaddleCanonicalEnv("PUBLIC_APP_URL", resolvePaddlePublicAppURL())
	setPaddleCanonicalEnv("PADDLE_CHECKOUT_URL", resolvePaddleCheckoutURL())
}

func setPaddleCanonicalEnv(key, value string) {
	if strings.TrimSpace(os.Getenv(key)) != "" || strings.TrimSpace(value) == "" {
		return
	}
	_ = os.Setenv(key, strings.TrimSpace(value))
}

type PaddleConfig struct {
	Enabled                       bool     `json:"enabled"`
	CheckoutReady                 bool     `json:"checkout_ready"`
	AutomationReady               bool     `json:"automation_ready"`
	Environment                   string   `json:"environment"`
	APIKeyConfigured              bool     `json:"api_key_configured"`
	ClientTokenConfigured         bool     `json:"client_token_configured"`
	WebhookConfigured             bool     `json:"webhook_configured"`
	APIKey                        string   `json:"-"`
	ClientToken                   string   `json:"-"`
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
	env := strings.ToLower(strings.TrimSpace(firstPaddleEnv("PADDLE_ENV", "PADDLE_ENVIRONMENT")))
	if env != "sandbox" {
		env = "production"
	}
	cfg := PaddleConfig{
		Environment:            env,
		APIKey:                 firstPaddleEnv("PADDLE_API_KEY", "PADDLE_SECRET_KEY"),
		ClientToken:            firstPaddleEnv("PADDLE_CLIENT_TOKEN", "PADDLE_CLIENT_SIDE_TOKEN", "NEXT_PUBLIC_PADDLE_CLIENT_TOKEN", "PUBLIC_PADDLE_CLIENT_TOKEN"),
		StarterPriceID:         firstPaddleEnv("PADDLE_STARTER_PRICE_ID", "PADDLE_STARTER_PRICE_USD_ID", "PADDLE_PRICE_STARTER_ID"),
		ProfessionalPriceID:    firstPaddleEnv("PADDLE_PROFESSIONAL_PRICE_ID", "PADDLE_PROFESSIONAL_PRICE_USD_ID", "PADDLE_BUILDER_PRICE_ID", "PADDLE_PRICE_PROFESSIONAL_ID"),
		EnterprisePriceID:      firstPaddleEnv("PADDLE_ENTERPRISE_PRICE_ID", "PADDLE_ENTERPRISE_PRICE_USD_ID", "PADDLE_STUDIO_PRICE_ID", "PADDLE_PRICE_ENTERPRISE_ID"),
		StarterProductID:       firstPaddleEnv("PADDLE_STARTER_PRODUCT_ID", "PADDLE_PRODUCT_STARTER_ID"),
		ProfessionalProductID:  firstPaddleEnv("PADDLE_PROFESSIONAL_PRODUCT_ID", "PADDLE_BUILDER_PRODUCT_ID", "PADDLE_PRODUCT_PROFESSIONAL_ID"),
		EnterpriseProductID:    firstPaddleEnv("PADDLE_ENTERPRISE_PRODUCT_ID", "PADDLE_STUDIO_PRODUCT_ID", "PADDLE_PRODUCT_ENTERPRISE_ID"),
		PublicAppURL:           resolvePaddlePublicAppURL(),
		CheckoutURL:            resolvePaddleCheckoutURL(),
	}
	cfg.APIKeyConfigured = cfg.APIKey != ""
	cfg.ClientTokenConfigured = cfg.ClientToken != ""
	cfg.WebhookConfigured = strings.TrimSpace(firstPaddleEnv("PADDLE_WEBHOOK_SECRET", "PADDLE_WEBHOOK_KEY")) != ""

	allPrices := cfg.StarterPriceID != "" && cfg.ProfessionalPriceID != "" && cfg.EnterprisePriceID != ""

	// KOSCHEİ creates Paddle transactions server-side with PADDLE_API_KEY and
	// redirects the customer to Paddle's returned checkout URL. A client token
	// is optional for this architecture and must not mark the system unavailable.
	cfg.CheckoutReady = cfg.APIKeyConfigured && cfg.PublicAppURL != "" && allPrices
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
		"configured":                    c.Enabled,
		"status":                        paddleConfigStatus(c),
		"environment":                   c.Environment,
		"checkout_ready":                c.CheckoutReady,
		"automation_ready":              c.AutomationReady,
		"api_key_configured":            c.APIKeyConfigured,
		"client_token_configured":       c.ClientTokenConfigured,
		"client_token_required":         false,
		"webhook_configured":            c.WebhookConfigured,
		"starter_price_configured":      c.StarterPriceID != "",
		"professional_price_configured": c.ProfessionalPriceID != "",
		"enterprise_price_configured":   c.EnterprisePriceID != "",
		"public_app_url_configured":     c.PublicAppURL != "",
		"checkout_url_configured":       c.CheckoutURL != "",
		"starter_ready":                 c.PlanReady("starter"),
		"professional_ready":            c.PlanReady("professional"),
		"enterprise_ready":              c.PlanReady("enterprise"),
		"missing_fields":                append([]string(nil), c.MissingFields...),
	}
}

func resolvePaddlePublicAppURL() string {
	value := strings.TrimSpace(firstPaddleEnv("PUBLIC_APP_URL", "NEXT_PUBLIC_APP_URL", "RAILWAY_STATIC_URL"))
	if value == "" {
		if domain := strings.TrimSpace(os.Getenv("RAILWAY_PUBLIC_DOMAIN")); domain != "" {
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
	if value := strings.TrimSpace(firstPaddleEnv("PADDLE_CHECKOUT_URL", "PADDLE_DEFAULT_PAYMENT_LINK")); value != "" {
		return strings.TrimRight(value, "/")
	}
	if appURL := resolvePaddlePublicAppURL(); appURL != "" {
		return appURL + "/pricing"
	}
	return ""
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
	if c.Enabled {
		return "configured"
	}
	if c.CheckoutReady {
		return "checkout_ready_webhook_incomplete"
	}
	if c.APIKeyConfigured || c.ClientTokenConfigured || c.WebhookConfigured || c.StarterPriceID != "" || c.ProfessionalPriceID != "" || c.EnterprisePriceID != "" || c.PublicAppURL != "" {
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
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
