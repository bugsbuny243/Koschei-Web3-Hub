package handlers

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

var requiredProductionAuthEnv = [][]string{
	{"NEON_AUTH_BASE_URL"},
	{"NEON_AUTH_ISSUER"},
	{"NEON_AUTH_JWKS_URL"},
	{"DATABASE_URL"},
	{"CORS_ORIGIN", "CORS_ALLOWED_ORIGIN"},
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

func configuredPublicAppURL() string {
	for _, name := range []string{"PUBLIC_APP_URL", "APP_BASE_URL", "SITE_URL"} {
		if value := normalizeAbsoluteBaseURL(trimmedEnv(name)); value != "" {
			return value
		}
	}
	return ""
}

func publicBaseURL(r *http.Request) string {
	if value := configuredPublicAppURL(); value != "" {
		return value
	}
	if r == nil {
		return ""
	}
	host := getHost(r)
	if host == "" {
		return ""
	}
	return strings.TrimRight(getScheme(r)+"://"+host, "/")
}

func absolutePublicURL(r *http.Request, path string) string {
	baseURL := publicBaseURL(r)
	if baseURL == "" {
		return ""
	}
	if parsed, err := url.Parse(strings.TrimSpace(path)); err == nil && parsed.IsAbs() {
		return strings.TrimRight(parsed.String(), "/")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}

func normalizeAbsoluteBaseURL(raw string) string {
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return value
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
	for _, alternatives := range requiredProductionAuthEnv {
		found := false
		for _, name := range alternatives {
			if trimmedEnv(name) != "" {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, strings.Join(alternatives, " or "))
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
