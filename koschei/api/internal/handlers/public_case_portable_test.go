package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicCasePortablePageRejectsInvalidCaseRef(t *testing.T) {
	t.Setenv("KOSCHEI_PUBLIC_REGISTRY_BACKEND", "drive")
	req := httptest.NewRequest(http.MethodGet, "/case/not-a-case", nil)
	resp := httptest.NewRecorder()
	(&Handler{}).PublicCasePortablePage(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want %d", resp.Code, http.StatusNotFound)
	}
}

func TestPublicCasePortablePageRejectsUnsupportedBackend(t *testing.T) {
	t.Setenv("KOSCHEI_PUBLIC_REGISTRY_BACKEND", "unknown-backend")
	req := httptest.NewRequest(http.MethodGet, "/case/KD1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil)
	resp := httptest.NewRecorder()
	(&Handler{}).PublicCasePortablePage(resp, req)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", resp.Code, http.StatusServiceUnavailable)
	}
}

func TestPublicCasePortablePageDatabaseModePreservesDBRequirement(t *testing.T) {
	t.Setenv("KOSCHEI_PUBLIC_REGISTRY_BACKEND", "database")
	req := httptest.NewRequest(http.MethodGet, "/case/KD1-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil)
	resp := httptest.NewRecorder()
	(&Handler{}).PublicCasePortablePage(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want %d", resp.Code, http.StatusNotFound)
	}
}
