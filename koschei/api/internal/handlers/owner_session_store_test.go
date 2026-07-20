package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func resetOwnerSessionMemoryForTest() {
	ownerSessionMemory.Lock()
	ownerSessionMemory.items = map[string]ownerSessionRecord{}
	ownerSessionMemory.Unlock()
}

func TestOwnerSessionUsesOpaqueTokenAndCanBeRevoked(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	resetOwnerSessionMemoryForTest()
	h := &Handler{}
	request := httptest.NewRequest("POST", "https://example.test/api/owner/login", nil)
	request.RemoteAddr = "203.0.113.4:4000"

	token, expiresAt, err := h.issueOwnerSession(request.Context(), "OwnerWallet", request)
	if err != nil {
		t.Fatalf("issueOwnerSession() error = %v", err)
	}
	if token == "" || token == "owner-secret" {
		t.Fatalf("issued token is not opaque: %q", token)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatal("session expiry is not in the future")
	}

	authRequest := httptest.NewRequest("GET", "https://example.test/api/owner/status", nil)
	authRequest.AddCookie(&http.Cookie{Name: ownerSessionCookieName, Value: token})
	wallet, ok := h.ownerSessionFromRequest(authRequest.Context(), authRequest)
	if !ok || wallet != "ownerwallet" {
		t.Fatalf("ownerSessionFromRequest() = (%q, %t)", wallet, ok)
	}

	h.revokeOwnerSession(authRequest.Context(), authRequest)
	if _, ok := h.ownerSessionFromRequest(authRequest.Context(), authRequest); ok {
		t.Fatal("revoked session remained valid")
	}
}

func TestOwnerAuthDoesNotAcceptLegacyMasterSecretCookie(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "permanent-master-secret")
	resetOwnerSessionMemoryForTest()
	h := &Handler{}
	r := httptest.NewRequest("GET", "https://example.test/api/owner/status", nil)
	r.AddCookie(&http.Cookie{Name: "koschei_owner_secret", Value: "permanent-master-secret"})
	w := httptest.NewRecorder()
	if h.OwnerAuth(w, r) {
		t.Fatal("legacy master-secret cookie authenticated an owner request")
	}
	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestOwnerSessionCookieNeverContainsMasterSecret(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	w := httptest.NewRecorder()
	setOwnerSessionCookies(w, "opaque-session-token", "wallet", time.Now().Add(time.Hour))
	for _, cookie := range w.Result().Cookies() {
		if cookie.Value == "permanent-master-secret" {
			t.Fatalf("master secret leaked through cookie %q", cookie.Name)
		}
		if cookie.Name == "koschei_owner_secret" && cookie.Value != "" {
			t.Fatalf("legacy secret cookie was populated: %q", cookie.Value)
		}
	}
}
