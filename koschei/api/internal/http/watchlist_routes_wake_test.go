package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"koschei/api/internal/workerwake"
)

func TestWatchlistPostWakesWebhookDeliveryAfterHandler(t *testing.T) {
	gate := workerwake.Get(workerwake.WebhookDelivery)
	gate.Drain()
	handled := false
	handler := wakeWebhookDeliveryAfterWatchlistPost(func(w http.ResponseWriter, r *http.Request) {
		handled = true
		w.WriteHeader(http.StatusNoContent)
	})

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/watchlist/refresh", nil))
	if !handled {
		t.Fatal("wrapped handler was not called")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if signalled := gate.Wait(ctx, time.Second); !signalled {
		t.Fatal("POST did not wake webhook delivery")
	}
}

func TestWatchlistReadDoesNotWakeWebhookDelivery(t *testing.T) {
	gate := workerwake.Get(workerwake.WebhookDelivery)
	gate.Drain()
	handler := wakeWebhookDeliveryAfterWatchlistPost(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/watchlist", nil))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if signalled := gate.Wait(ctx, 20*time.Millisecond); signalled {
		t.Fatal("GET unexpectedly woke webhook delivery")
	}
}

func TestWatchlistReadsAreGatedWithoutConsumingOutputs(t *testing.T) {
	var gates []string
	gate := func(name string) routeGate {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) { gates = append(gates, name); next(w, r) }
		}
	}
	handler := meterWatchlistWrites(gate("metered"), gate("access"))(func(w http.ResponseWriter, r *http.Request) {})
	for _, tc := range []struct{ method, want string }{
		{http.MethodGet, "access"}, {http.MethodHead, "access"},
		{http.MethodPost, "metered"}, {http.MethodPatch, "metered"}, {http.MethodDelete, "metered"},
	} {
		gates = nil
		handler(httptest.NewRecorder(), httptest.NewRequest(tc.method, "/api/watchlist", nil))
		if len(gates) != 1 || gates[0] != tc.want {
			t.Fatalf("%s /api/watchlist passed gates %v, want only %s", tc.method, gates, tc.want)
		}
	}
}
