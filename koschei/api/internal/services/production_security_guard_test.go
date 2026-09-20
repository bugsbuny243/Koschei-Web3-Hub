package services

import (
	"slices"
	"testing"
)

func TestMissingProductionSecurityEnvRequiresVerdictSigner(t *testing.T) {
	configureProductionSecurityGuardTestEnv(t)
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")

	missing := MissingProductionSecurityEnv()
	if !slices.Contains(missing, unifiedVerdictSigningKeyIDEnv) {
		t.Fatalf("missing production env did not include %s: %#v", unifiedVerdictSigningKeyIDEnv, missing)
	}
	if !slices.Contains(missing, unifiedVerdictSigningPrivateKeyEnv) {
		t.Fatalf("missing production env did not include %s: %#v", unifiedVerdictSigningPrivateKeyEnv, missing)
	}
}

func TestMissingProductionSecurityEnvAcceptsConfiguredVerdictSigner(t *testing.T) {
	configureProductionSecurityGuardTestEnv(t)
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "production-test-key-v1")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "U1NTU1NTU1NTU1NTU1NTU1NTU1NTU1NTU1NTU1NTU1M")

	if missing := MissingProductionSecurityEnv(); len(missing) != 0 {
		t.Fatalf("unexpected missing production env: %#v", missing)
	}
}

func TestMissingProductionSecurityEnvDoesNotRequireSignerOutsideProduction(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")

	if missing := MissingProductionSecurityEnv(); missing != nil {
		t.Fatalf("non-production guard unexpectedly required env: %#v", missing)
	}
}

func configureProductionSecurityGuardTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "production")
	t.Setenv("KOSCHEI_NEON_AUTH_ONLY", "true")
	t.Setenv("API_KEY_PEPPER", "test-pepper")
	t.Setenv("USER_SESSION_SECRET", "test-session-secret")
	t.Setenv("OWNER_SECRET", "test-owner-secret")
	t.Setenv("NEON_AUTH_JWKS_URL", "https://example.invalid/.well-known/jwks.json")
}
