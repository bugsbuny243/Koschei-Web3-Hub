package services

import (
	"os"
	"strings"
)

func init() {
	canonicalizePaddleEnv()
}

func canonicalizePaddleEnv() {
	setPaddleCanonicalEnv("PADDLE_WEBHOOK_SECRET", firstPaddleEnv("PADDLE_WEBHOOK_SECRET", "PADDLE_WEBHOOK_KEY"))
	setPaddleCanonicalEnv("PADDLE_STARTER_PRICE_ID", firstPaddleEnv("PADDLE_STARTER_PRICE_ID", "PADDLE_STARTER_PRICE_USD_ID"))
	setPaddleCanonicalEnv("PADDLE_PROFESSIONAL_PRICE_ID", firstPaddleEnv("PADDLE_PROFESSIONAL_PRICE_ID", "PADDLE_PROFESSIONAL_PRICE_USD_ID", "PADDLE_BUILDER_PRICE_ID"))
	setPaddleCanonicalEnv("PADDLE_ENTERPRISE_PRICE_ID", firstPaddleEnv("PADDLE_ENTERPRISE_PRICE_ID", "PADDLE_ENTERPRISE_PRICE_USD_ID", "PADDLE_STUDIO_PRICE_ID"))
	setPaddleCanonicalEnv("PUBLIC_APP_URL", resolvePaddlePublicAppURL())
}

func setPaddleCanonicalEnv(key, value string) {
	if strings.TrimSpace(os.Getenv(key)) != "" || strings.TrimSpace(value) == "" {
		return
	}
	_ = os.Setenv(key, strings.TrimSpace(value))
}

type PaddleConfig struct {
	Enabled             bool     `json:"enabled"`
	CheckoutReady       bool     `json:"checkout_ready"`
	AutomationReady     bool     `json:"automation_ready"`
	Environment         string   `json:"environment"`
	APIKeyConfigured    bool     `json:"api_key_configured"`
	WebhookConfigured   bool     `json:"webhook_configured"`
	StarterPriceID      string   `json:"-"`
	ProfessionalPriceID string   `json:"-"`
	EnterprisePriceID   string   `json:"-"`
	PublicAppURL        string   `json:"-"`
	MissingFields       []string `json:"-"`
}

func LoadPaddleConfigFromEnv() PaddleConfig {
	env := strings.ToLower(strings.TrimSpace(firstPaddleEnv("PADDLE_ENV", "PADDLE_ENVIRONMENT")))
	if env != "sandbox" {
		env = "production"
	}
	cfg := PaddleConfig{
		Environment:         env,
		APIKeyConfigured:    strings.TrimSpace(firstPaddleEnv("PADDLE_API_KEY")) != "",
		WebhookConfigured:   strings.TrimSpace(firstPaddleEnv("PADDLE_WEBHOOK_SECRET", "PADDLE_WEBHOOK_KEY")) != "",
		StarterPriceID:      firstPaddleEnv("PADDLE_STARTER_PRICE_ID", "PADDLE_STARTER_PRICE_USD_ID"),
		ProfessionalPriceID: firstPaddleEnv("PADDLE_PROFESSIONAL_PRICE_ID", "PADDLE_PROFESSIONAL_PRICE_USD_ID", "PADDLE_BUILDER_PRICE_ID"),
		EnterprisePriceID:   firstPaddleEnv("PADDLE_ENTERPRISE_PRICE_ID", "PADDLE_ENTERPRISE_PRICE_USD_ID", "PADDLE_STUDIO_PRICE_ID"),
		PublicAppURL:        resolvePaddlePublicAppURL(),
	}
	cfg.CheckoutReady = cfg.APIKeyConfigured && cfg.PublicAppURL != "" && cfg.StarterPriceID != "" && cfg.ProfessionalPriceID != "" && cfg.EnterprisePriceID != ""
	cfg.AutomationReady = cfg.CheckoutReady && cfg.WebhookConfigured
	cfg.Enabled = cfg.AutomationReady
	cfg.MissingFields = paddleMissingFields(cfg)
	return cfg
}

func (c PaddleConfig) PublicStatus() map[string]any {
	return map[string]any{
		"configured":                    c.Enabled,
		"status":                        paddleConfigStatus(c),
		"environment":                   c.Environment,
		"checkout_ready":                c.CheckoutReady,
		"automation_ready":              c.AutomationReady,
		"api_key_configured":            c.APIKeyConfigured,
		"webhook_configured":            c.WebhookConfigured,
		"starter_price_configured":      c.StarterPriceID != "",
		"professional_price_configured": c.ProfessionalPriceID != "",
		"enterprise_price_configured":   c.EnterprisePriceID != "",
		"public_app_url_configured":     c.PublicAppURL != "",
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
		missing = append(missing, "PUBLIC_APP_URL_or_RAILWAY_PUBLIC_DOMAIN")
	}
	return missing
}

func paddleConfigStatus(c PaddleConfig) string {
	if c.Enabled {
		return "configured"
	}
	if c.APIKeyConfigured || c.WebhookConfigured || c.StarterPriceID != "" || c.ProfessionalPriceID != "" || c.EnterprisePriceID != "" || c.PublicAppURL != "" {
		return "partial"
	}
	return "not_configured"
}

func firstPaddleEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
