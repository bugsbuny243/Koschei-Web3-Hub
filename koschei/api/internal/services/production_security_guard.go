package services

import (
	"os"
	"strings"
)

func MissingProductionSecurityEnv() []string {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		return nil
	}
	required := []string{"API_KEY_PEPPER", "USER_SESSION_SECRET", "OWNER_SECRET", "DATABASE_URL", "NEON_AUTH_JWKS_URL"}
	missing := []string{}
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}
