package handlers

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func resetJWKSCacheForTest() {
	jwksRefreshMu.Lock()
	defer jwksRefreshMu.Unlock()
	jwksMu.Lock()
	defer jwksMu.Unlock()
	jwksCache = nil
	jwksCacheExpiresAt = time.Time{}
	jwksNegativeCache = map[string]time.Time{}
}

func TestLoadJWKSKeyNegativeCachesUnknownKid(t *testing.T) {
	resetJWKSCacheForTest()
	t.Cleanup(resetJWKSCacheForTest)

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kid":"known","kty":"RSA","n":"AQ","e":"Aw"}]}`))
	}))
	defer server.Close()
	t.Setenv("NEON_AUTH_JWKS_URL", server.URL)

	for i := 0; i < 3; i++ {
		if _, err := loadJWKSKey("missing"); err == nil {
			t.Fatal("expected unknown key error")
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("JWKS requests = %d; want 1 while negative cache is valid", got)
	}
}

func TestLoadJWKSKeyRefreshesExpiredCache(t *testing.T) {
	resetJWKSCacheForTest()
	t.Cleanup(resetJWKSCacheForTest)

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kid":"rotated","kty":"RSA","n":"AQ","e":"Aw"}]}`))
	}))
	defer server.Close()
	t.Setenv("NEON_AUTH_JWKS_URL", server.URL)

	jwksMu.Lock()
	jwksCache = map[string]neonJWK{"removed": {Kid: "removed", Kty: "RSA", N: "AQ", E: "Aw"}}
	jwksCacheExpiresAt = time.Now().Add(-time.Second)
	jwksMu.Unlock()

	if _, err := loadJWKSKey("removed"); err == nil {
		t.Fatal("expected removed key to be rejected after cache expiry and refresh")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("JWKS requests = %d; want 1 refresh", got)
	}
	jwksMu.RLock()
	_, stillCached := jwksCache["removed"]
	jwksMu.RUnlock()
	if stillCached {
		t.Fatal("removed key remained cached after successful refresh")
	}
}

func TestLoadJWKSKeyCoalescesConcurrentRefreshes(t *testing.T) {
	resetJWKSCacheForTest()
	t.Cleanup(resetJWKSCacheForTest)

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(25 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[{"kid":"shared","kty":"RSA","n":"AQ","e":"Aw"}]}`))
	}))
	defer server.Close()
	t.Setenv("NEON_AUTH_JWKS_URL", server.URL)

	const workers = 12
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, err := loadJWKSKey("shared")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent load failed: %v", err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("JWKS requests = %d; want 1 coalesced refresh", got)
	}
}
