package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLivenessReportsConfiguredPersistenceWithoutQueryingDatabase(t *testing.T) {
	// An uninitialised handle would fail/panic if liveness queried it.
	h := &Handler{DB: new(sql.DB)}
	res := httptest.NewRecorder()
	h.Health(res, httptest.NewRequest(http.MethodGet, "/health", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"persistence":"postgresql"`) || !strings.Contains(res.Body.String(), `"database":"configured"`) {
		t.Fatalf("incorrect persistence metadata: %s", res.Body.String())
	}
}
