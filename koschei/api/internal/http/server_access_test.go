package http

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"koschei/api/internal/handlers"
)

func TestProductRouteTierMapAndFreeCore(t *testing.T) {
	mux := http.NewServeMux()
	h := &handlers.Handler{}
	tiers := []string{}
	gate := func(tier string, next http.HandlerFunc) http.HandlerFunc {
		tiers = append(tiers, tier)
		return next
	}
	registerProductRoutes(mux, h, gate)

	// Basic includes the existing three security products plus the canonical
	// durable-job create routes and radar detail. Job reads remain authenticated
	// but are not counted here because they do not consume a new scan quota.
	want := []string{"basic", "basic", "basic", "basic", "basic", "basic", "pro", "pro", "pro", "pro", "pro", "pro"}
	if !reflect.DeepEqual(tiers, want) {
		t.Fatalf("route tiers=%v want=%v", tiers, want)
	}

	// A GET reaches the free route's method guard directly. A KOSCH/quota gate
	// would have been registered through gate above and changed the tier list.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/token/scan", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("free token scan unexpectedly gated: status=%d body=%s", rr.Code, rr.Body.String())
	}
}
