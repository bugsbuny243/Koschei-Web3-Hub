package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// Tests route behavior at the real NewServer boundary, not just at an inner
// handler. Everything needed for the static probe is a local synthetic fixture.
func TestAuditSensitiveStaticProbesAre404NoStoreThroughNewServer(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	root := t.TempDir()
	for name, contents := range map[string]string{
		"index.html":                    "public-home",
		".env":                          "TEST-ONLY-SECRET",
		"validation-key.txt":            "TEST-ONLY-VALIDATION-KEY",
		"js/koschei-modern.js":          "TEST-ONLY-LEGACY-SCRIPT",
		"js/__fixtures__/sample.json":   "TEST-ONLY-FIXTURE",
		"js/__tests__/example.test.cjs": "TEST-ONLY-TEST",
		"sdk/koschei-shield.js":         "public-sdk-contract",
		"full-scan-contract-v1.json":    `{"public":true}`,
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	server := NewServer(nil, "", "", "", root)
	for _, probe := range []string{
		"/.env", "/.env%2elocal", "/validation-key.txt", "/js/koschei-modern.js",
		"/js/__fixtures__/sample.json", "/js/__tests__/example.test.cjs",
		"/keys/private.pem", "/.git/config", "/railway.toml",
	} {
		t.Run(probe, func(t *testing.T) {
			res := httptest.NewRecorder()
			server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, probe, nil))
			if res.Code != http.StatusNotFound || res.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("probe=%q status=%d cache=%q", probe, res.Code, res.Header().Get("Cache-Control"))
			}
			if strings.Contains(res.Body.String(), "TEST-ONLY") {
				t.Fatalf("probe=%q leaked fixture content", probe)
			}
		})
	}
	for _, public := range []string{"/sdk/koschei-shield.js", "/full-scan-contract-v1.json"} {
		res := httptest.NewRecorder()
		server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, public, nil))
		if res.Code != http.StatusOK {
			t.Errorf("explicit public compatibility path %q status=%d", public, res.Code)
		}
	}
}

func TestAuditRetiredSyntheticRoutesAreNotRegistered(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	// Non-nil inert database avoids the readiness wrapper's intentional 503
	// without contacting any database. No handler is registered for these paths.
	server := NewServer(&sql.DB{}, "", "", "", t.TempDir())
	for _, path := range []string{
		"/api/mev/analyze", "/api/liquidity/analyze",
		"/api/dao/proposal-risk", "/api/owner/kosch-access",
	} {
		res := httptest.NewRecorder()
		server.ServeHTTP(res, httptest.NewRequest(http.MethodPost, path, nil))
		if res.Code != http.StatusNotFound {
			t.Errorf("retired API %q returned %d instead of 404", path, res.Code)
		}
	}
}

func TestAuditPublicRiskBadgeFailsClosedThroughNewServer(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FEATURE_SOLANA", "true")
	t.Setenv("KOSCHEI_PUBLIC_BADGE_ENABLED", "true")
	for _, key := range []string{"SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"} {
		t.Setenv(key, "")
	}
	server := NewServer(nil, "", "", "", t.TempDir())
	for _, tc := range []struct {
		name, method, path string
		status             int
	}{
		{"missing address", http.MethodGet, "/api/v1/risk/badge", http.StatusBadRequest},
		{"malformed address", http.MethodGet, "/api/v1/risk/badge?address=not-a-mint", http.StatusBadRequest},
		{"malformed alias", http.MethodGet, "/api/v1/risk/badge?token=not-a-mint", http.StatusBadRequest},
		{"unverified without RPC", http.MethodGet, "/api/v1/risk/badge?address=11111111111111111111111111111111", http.StatusServiceUnavailable},
		{"wrong method", http.MethodPost, "/api/v1/risk/badge", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			server.ServeHTTP(res, httptest.NewRequest(tc.method, tc.path, nil))
			if res.Code != tc.status {
				t.Fatalf("status=%d want=%d response=%s", res.Code, tc.status, res.Body.String())
			}
			var payload map[string]any
			if tc.method == http.MethodGet {
				if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if payload["signed"] == true || payload["risk_level"] == "low" {
					t.Fatalf("badge granted unverifiable safety verdict: %v", payload)
				}
				if value, found := payload["risk_index"]; found && value != nil {
					t.Fatalf("badge exposed numeric risk without verified final: %v", payload)
				}
			}
		})
	}
}

// A local RPC fixture verifies the genuine minted-target branch without any
// external provider, signer, DB or billing operation.
func TestAuditVerifiedMintBadgeWithholdsUntilCanonicalProofExists(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FEATURE_SOLANA", "true")
	t.Setenv("KOSCHEI_PUBLIC_BADGE_ENABLED", "true")
	var requests atomic.Int32
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","data":{"parsed":{"type":"mint","info":{}}}}}}`))
	}))
	defer rpc.Close()
	t.Setenv("SOLANA_RPC_URL", rpc.URL)
	for _, name := range []string{"ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"} {
		t.Setenv(name, "")
	}
	server := NewServer(nil, "", "", "", t.TempDir())
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/risk/badge?address=11111111111111111111111111111111", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("verified mint returned status %d, body: %s", res.Code, res.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["signed"] != false || got["risk_index"] != nil || got["risk_level"] != "unknown" || got["verdict"] != "withhold" {
		t.Fatalf("unsigned compatibility arm escaped as grade: %v", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("public badge unexpectedly ran provider evidence collectors: RPC requests=%d, want classification-only", requests.Load())
	}
}

// jsonParsed alone is not authoritative: a fabricated mint type on an account
// not owned by the SPL Token/Token-2022 programs must never produce a grade.
func TestAuditBadgeRejectsMintTypeWithUntrustedAccountOwner(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FEATURE_SOLANA", "true")
	t.Setenv("KOSCHEI_PUBLIC_BADGE_ENABLED", "true")
	var requests atomic.Int32
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"value":{"owner":"11111111111111111111111111111111","data":{"parsed":{"type":"mint","info":{}}}}}}`))
	}))
	defer rpc.Close()
	t.Setenv("SOLANA_RPC_URL", rpc.URL)
	for _, key := range []string{"ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"} {
		t.Setenv(key, "")
	}
	server := NewServer(nil, "", "", "", t.TempDir())
	res := httptest.NewRecorder()
	server.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/risk/badge?address=11111111111111111111111111111111", nil))
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("mismatched account owner status=%d, want 503; body=%s", res.Code, res.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["risk_index"] != nil || payload["signed"] != false || payload["risk_level"] != "unknown" {
		t.Fatalf("owner mismatch incorrectly authorized a badge: %+v", payload)
	}
	classification, _ := payload["target_classification"].(map[string]any)
	if classification["status"] != "owner_program_mismatch" || classification["type"] != "unknown" {
		t.Fatalf("untrusted owner classification lost: %v", classification)
	}
	if requests.Load() != 1 {
		t.Fatalf("expected one classification-only RPC call, got %d", requests.Load())
	}
}
