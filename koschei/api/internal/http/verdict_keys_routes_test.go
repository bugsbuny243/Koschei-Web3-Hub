package http

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func getVerdictKeys(t *testing.T) (int, string, map[string]any) {
	t.Helper()
	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewServer(nil, "", "", "", staticDir))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + verdictKeysPath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("verdict keys response is not JSON (%d): %s", resp.StatusCode, body)
	}
	return resp.StatusCode, string(body), payload
}

func TestVerdictKeysEndpointPublishesOnlyThePublicKey(t *testing.T) {
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", "")
	status, _, payload := getVerdictKeys(t)
	if status != http.StatusOK || payload["status"] != "signer_unconfigured" || len(payload["keys"].([]any)) != 0 {
		t.Fatalf("unconfigured signer: %d %#v", status, payload)
	}

	// Test fixture key; never a production secret.
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x5a}, ed25519.SeedSize))
	seed := base64.RawURLEncoding.EncodeToString(key.Seed())
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "endpoint-test-key")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", seed)
	status, body, payload := getVerdictKeys(t)
	if status != http.StatusOK || payload["status"] != "active" {
		t.Fatalf("configured signer: %d %s", status, body)
	}
	keys := payload["keys"].([]any)
	if len(keys) != 1 {
		t.Fatalf("expected one published key: %s", body)
	}
	published := keys[0].(map[string]any)
	want := base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
	if published["key_id"] != "endpoint-test-key" || published["public_key_base64url"] != want {
		t.Fatalf("unexpected published key: %s", body)
	}
	for _, secret := range []string{seed, base64.RawURLEncoding.EncodeToString(key), base64.StdEncoding.EncodeToString(key.Seed())} {
		if strings.Contains(body, secret) {
			t.Fatal("verdict keys endpoint leaked private key material")
		}
	}
}

func TestVerdictKeysEndpointFailsClosedOnInvalidSigner(t *testing.T) {
	t.Setenv("KOSCHEI_VERDICT_SIGNING_KEY_ID", "broken-key")
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", "c2hvcnQ")
	status, body, payload := getVerdictKeys(t)
	if status != http.StatusServiceUnavailable || payload["status"] != "signer_invalid" || len(payload["keys"].([]any)) != 0 {
		t.Fatalf("invalid signer must publish no key: %d %s", status, body)
	}
	if strings.Contains(body, "c2hvcnQ") || strings.Contains(body, "32-byte") {
		t.Fatalf("invalid signer response echoed configuration details: %s", body)
	}
}
