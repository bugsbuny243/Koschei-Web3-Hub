package handlers

import "testing"

func configureHandlerVerdictTestSigner(t *testing.T) {
	t.Helper()
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "test-suite-verdict-key-v1")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", "U1NTU1NTU1NTU1NTU1NTU1NTU1NTU1NTU1NTU1NTU1M")
}
