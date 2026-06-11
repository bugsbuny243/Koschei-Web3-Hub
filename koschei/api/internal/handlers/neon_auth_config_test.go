package handlers

import "testing"

func TestNeonAuthConfigDefaultsKeepAuthEnabled(t *testing.T) {
	t.Setenv("NEON_AUTH_BASE_URL", "")
	t.Setenv("EXPO_PUBLIC_NEON_AUTH_URL", "")
	t.Setenv("NEON_AUTH_ISSUER", "")
	t.Setenv("NEON_AUTH_JWKS_URL", "")

	if got := configuredNeonAuthBaseURL(); got != defaultNeonAuthBaseURL {
		t.Fatalf("configuredNeonAuthBaseURL() = %q, want %q", got, defaultNeonAuthBaseURL)
	}
	if got := configuredPublicNeonAuthURL(); got != defaultNeonAuthBaseURL {
		t.Fatalf("configuredPublicNeonAuthURL() = %q, want %q", got, defaultNeonAuthBaseURL)
	}
	if got := configuredNeonAuthIssuer(); got != defaultNeonAuthIssuer {
		t.Fatalf("configuredNeonAuthIssuer() = %q, want %q", got, defaultNeonAuthIssuer)
	}
	if got := configuredNeonAuthJWKSURL(); got != defaultNeonAuthJWKSURL {
		t.Fatalf("configuredNeonAuthJWKSURL() = %q, want %q", got, defaultNeonAuthJWKSURL)
	}
}

func TestConfiguredPublicNeonAuthURLPrefersExplicitPublicURL(t *testing.T) {
	t.Setenv("NEON_AUTH_BASE_URL", "https://private.example/neondb/auth")
	t.Setenv("EXPO_PUBLIC_NEON_AUTH_URL", "https://public.example/neondb/auth/")

	if got, want := configuredPublicNeonAuthURL(), "https://public.example/neondb/auth"; got != want {
		t.Fatalf("configuredPublicNeonAuthURL() = %q, want %q", got, want)
	}
}
