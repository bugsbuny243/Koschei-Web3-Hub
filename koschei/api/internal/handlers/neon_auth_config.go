package handlers

import (
	"os"
	"strings"
)

var requiredProductionAuthEnv = []string{
	"NEON_AUTH_BASE_URL",
	"NEON_AUTH_ISSUER",
	"NEON_AUTH_JWKS_URL",
	"NEON_AUTH_STATE_SECRET",
	"DATABASE_URL",
	"CORS_ORIGIN",
}

func configuredNeonAuthBaseURL() string {
	return trimmedEnv("NEON_AUTH_BASE_URL")
}

func configuredPublicNeonAuthURL() string {
	if value := trimmedEnv("EXPO_PUBLIC_NEON_AUTH_URL"); value != "" {
		return strings.TrimRight(value, "/")
	}
	return strings.TrimRight(configuredNeonAuthBaseURL(), "/")
}

func configuredNeonAuthIssuer() string {
	return trimmedEnv("NEON_AUTH_ISSUER")
}

func configuredNeonAuthJWKSURL() string {
	return trimmedEnv("NEON_AUTH_JWKS_URL")
}

func trimmedEnv(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func MissingProductionAuthEnv() []string {
	missing := []string{}
	for _, name := range requiredProductionAuthEnv {
		if trimmedEnv(name) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

func ConfiguredNeonAuthJWKSURL() string {
	return configuredNeonAuthJWKSURL()
}

func ConfiguredPublicNeonAuthURL() string {
	return configuredPublicNeonAuthURL()
}

func ConfiguredNeonAuthIssuer() string {
	return configuredNeonAuthIssuer()
}
