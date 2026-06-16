package services

import (
	"os"
	"strings"
)

type PaddleConfig struct {
	Enabled             bool   `json:"enabled"`
	Environment         string `json:"environment"`
	APIKeyConfigured    bool   `json:"api_key_configured"`
	WebhookConfigured   bool   `json:"webhook_configured"`
	StarterPriceID      string `json:"-"`
	ProfessionalPriceID string `json:"-"`
	EnterprisePriceID   string `json:"-"`
	PublicAppURL        string `json:"-"`
}

func LoadPaddleConfigFromEnv() PaddleConfig {
	env := strings.ToLower(strings.TrimSpace(firstPaddleEnv("PADDLE_ENV", "PADDLE_ENVIRONMENT")))
	if env != "sandbox" {
		env = "production"
	}
	cfg := PaddleConfig{
		Environment:         env,
		APIKeyConfigured:    strings.TrimSpace(os.Getenv("PADDLE_API_KEY")) != "",
		WebhookConfigured:   strings.TrimSpace(os.Getenv("PADDLE_WEBHOOK_SECRET")) != "",
		StarterPriceID:      strings.TrimSpace(os.Getenv("PADDLE_STARTER_PRICE_ID")),
		ProfessionalPriceID: firstPaddleEnv("PADDLE_PROFESSIONAL_PRICE_ID", "PADDLE_PROFESSIONAL_PRICE_USD_ID", "PADDLE_BUILDER_PRICE_ID"),
		EnterprisePriceID:   firstPaddleEnv("PADDLE_ENTERPRISE_PRICE_ID", "PADDLE_STUDIO_PRICE_ID"),
		PublicAppURL:        strings.TrimRight(firstPaddleEnv("PUBLIC_APP_URL", "NEXT_PUBLIC_APP_URL"), "/"),
	}
	cfg.Enabled = cfg.APIKeyConfigured && cfg.PublicAppURL != "" && cfg.StarterPriceID != "" && cfg.ProfessionalPriceID != "" && cfg.EnterprisePriceID != ""
	return cfg
}

func (c PaddleConfig) PublicStatus() map[string]any {
	return map[string]any{
		"configured":                    c.Enabled,
		"status":                        boolStatus(c.Enabled),
		"environment":                   c.Environment,
		"api_key_configured":            c.APIKeyConfigured,
		"webhook_configured":            c.WebhookConfigured,
		"starter_price_configured":       c.StarterPriceID != "",
		"professional_price_configured":  c.ProfessionalPriceID != "",
		"enterprise_price_configured":    c.EnterprisePriceID != "",
		"public_app_url_configured":      c.PublicAppURL != "",
	}
}

func boolStatus(ok bool) string {
	if ok {
		return "configured"
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
