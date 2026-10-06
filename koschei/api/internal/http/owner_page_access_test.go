package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"koschei/api/internal/handlers"
)

func TestOwnerPrivateApplicationRequiresOwnerAuth(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-test-secret")
	t.Setenv("OWNER_WALLET", "")

	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "owner-login.html"), []byte("PUBLIC OWNER LOGIN"), 0o600); err != nil {
		t.Fatalf("write owner login fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "owner-production.html"), []byte("PRIVATE OWNER APP"), 0o600); err != nil {
		t.Fatalf("write owner app fixture: %v", err)
	}

	mux := http.NewServeMux()
	registerOwnerRoutes(mux, &handlers.Handler{}, staticDir)

	loginRecorder := httptest.NewRecorder()
	mux.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/owner", nil))
	if loginRecorder.Code != http.StatusOK || !strings.Contains(loginRecorder.Body.String(), "PUBLIC OWNER LOGIN") {
		t.Fatalf("public owner login status=%d body=%q", loginRecorder.Code, loginRecorder.Body.String())
	}

	for _, path := range []string{"/owner-production", "/owner-production.html"} {
		t.Run("unauthenticated_"+strings.TrimPrefix(path, "/"), func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("unauthenticated owner app status=%d want=%d body=%q", recorder.Code, http.StatusNotFound, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "PRIVATE OWNER APP") {
				t.Fatal("private owner application body leaked without owner authentication")
			}
		})
	}

	authenticated := httptest.NewRequest(http.MethodGet, "/owner-production", nil)
	authenticated.Header.Set("x-koschei-secret", "owner-test-secret")
	authenticatedRecorder := httptest.NewRecorder()
	mux.ServeHTTP(authenticatedRecorder, authenticated)
	if authenticatedRecorder.Code != http.StatusOK || !strings.Contains(authenticatedRecorder.Body.String(), "PRIVATE OWNER APP") {
		t.Fatalf("authenticated owner app status=%d body=%q", authenticatedRecorder.Code, authenticatedRecorder.Body.String())
	}
}
