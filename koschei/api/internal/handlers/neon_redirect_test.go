package handlers

import "testing"

func TestNeonAuthStateSecretReusesExistingProductionSecrets(t *testing.T) {
	t.Setenv("NEON_AUTH_STATE_SECRET", "")
	t.Setenv("KOSCHEI_AUTH_STATE_SECRET", "")
	t.Setenv("USER_SESSION_SECRET", "")
	t.Setenv("OWNER_SECRET", "owner-secret")
	t.Setenv("KOSCHEI_OWNER_SECRET", "")
	t.Setenv("DATABASE_URL", "postgres://example")

	h := &Handler{AdminPassword: "admin-secret"}
	if got, want := h.neonAuthStateSecret(), "owner-secret"; got != want {
		t.Fatalf("neonAuthStateSecret() = %q, want %q", got, want)
	}
}

func TestNeonAuthStateSecretStillPrefersDedicatedOverride(t *testing.T) {
	t.Setenv("NEON_AUTH_STATE_SECRET", "state-secret")
	t.Setenv("KOSCHEI_AUTH_STATE_SECRET", "")
	t.Setenv("USER_SESSION_SECRET", "user-secret")
	t.Setenv("OWNER_SECRET", "owner-secret")
	t.Setenv("KOSCHEI_OWNER_SECRET", "")
	t.Setenv("DATABASE_URL", "postgres://example")

	h := &Handler{AdminPassword: "admin-secret"}
	if got, want := h.neonAuthStateSecret(), "state-secret"; got != want {
		t.Fatalf("neonAuthStateSecret() = %q, want %q", got, want)
	}
}
