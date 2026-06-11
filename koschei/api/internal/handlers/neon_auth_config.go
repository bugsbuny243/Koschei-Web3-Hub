package handlers

import (
	"os"
	"strings"
)

const (
	defaultNeonAuthIssuer  = "https://ep-long-hill-al1bj0wc.neonauth.c-3.eu-central-1.aws.neon.tech"
	defaultNeonAuthBaseURL = defaultNeonAuthIssuer + "/neondb/auth"
	defaultNeonAuthJWKSURL = defaultNeonAuthBaseURL + "/.well-known/jwks.json"
)

func configuredNeonAuthBaseURL() string {
	return envOrDefault("NEON_AUTH_BASE_URL", defaultNeonAuthBaseURL)
}

func configuredPublicNeonAuthURL() string {
	if value := strings.TrimSpace(os.Getenv("EXPO_PUBLIC_NEON_AUTH_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return strings.TrimRight(configuredNeonAuthBaseURL(), "/")
}

func configuredNeonAuthIssuer() string {
	return envOrDefault("NEON_AUTH_ISSUER", defaultNeonAuthIssuer)
}

func configuredNeonAuthJWKSURL() string {
	return envOrDefault("NEON_AUTH_JWKS_URL", defaultNeonAuthJWKSURL)
}

func envOrDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
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
