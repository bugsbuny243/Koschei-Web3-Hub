package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCachedArvisHealthSnapshotDoesNotTriggerCollection(t *testing.T) {
	resetArvisHealthCache()
	t.Cleanup(resetArvisHealthCache)
	snapshot := cachedArvisHealthSnapshot()
	if snapshot["pipeline_status"] != "live_provider_mode" || snapshot["cached"] != false {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	if snapshot["details_url"] != evidenceHealthURL {
		t.Fatalf("details_url=%v", snapshot["details_url"])
	}
}

func TestEvidenceHealthRefreshPopulatesAndReusesSnapshot(t *testing.T) {
	resetArvisHealthCache()
	t.Cleanup(resetArvisHealthCache)
	h := &Handler{}
	recorder := httptest.NewRecorder()
	h.Health(recorder, httptest.NewRequest(http.MethodGet, evidenceHealthURL, nil))
	var body struct {
		Arvis map[string]any `json:"arvis"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || body.Arvis["cached"] != true {
		t.Fatalf("refresh did not collect a snapshot: %d %#v", recorder.Code, body.Arvis)
	}
	if body.Arvis["pipeline_status"] != "database_unavailable" {
		t.Fatalf("missing observation store must stay unverified: %#v", body.Arvis)
	}
	expiresAt, err := time.Parse(time.RFC3339, body.Arvis["cache_expires_at"].(string))
	if err != nil || !expiresAt.After(time.Now()) {
		t.Fatalf("refresh did not bind freshness: %#v", body.Arvis)
	}

	// A plain liveness read sees the same observation and does not extend its TTL.
	snapshot := cachedArvisHealthSnapshot()
	if snapshot["cache_expires_at"] != body.Arvis["cache_expires_at"] {
		t.Fatal("liveness extended observation freshness")
	}
	second := h.cachedArvisHealth(context.Background())
	if second["cache_expires_at"] != body.Arvis["cache_expires_at"] {
		t.Fatal("fresh cache was recollected")
	}
	second["pipeline_status"] = "mutated"
	if cachedArvisHealthSnapshot()["pipeline_status"] != "database_unavailable" {
		t.Fatal("refresh returned a mutable shared cache map")
	}
}

func TestEvidenceHealthCancelledRefreshCannotPublish(t *testing.T) {
	resetArvisHealthCache()
	t.Cleanup(resetArvisHealthCache)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, evidenceHealthURL, nil).WithContext(ctx)
	(&Handler{}).Health(recorder, request)
	if !strings.Contains(recorder.Body.String(), `"pipeline_status":"unavailable"`) {
		t.Fatalf("cancelled refresh was not rejected: %s", recorder.Body.String())
	}
	if cachedArvisHealthSnapshot()["cached"] != false {
		t.Fatal("cancelled collection populated the public cache")
	}
}

func TestEvidenceHealthConcurrentRefreshDoesNotBlockLiveness(t *testing.T) {
	resetArvisHealthCache()
	t.Cleanup(resetArvisHealthCache)
	arvisHealthRefresh.Lock()
	defer arvisHealthRefresh.Unlock()
	for _, path := range []string{"/health", evidenceHealthURL} {
		completed := make(chan struct{})
		go func() {
			defer close(completed)
			(&Handler{}).Health(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		}()
		select {
		case <-completed:
		case <-time.After(250 * time.Millisecond):
			t.Fatalf("%s waited for a running evidence collection", path)
		}
	}
}

func TestEvidenceHealthRequiresCompletePositiveMeasurements(t *testing.T) {
	stats := map[string]any{
		"pipeline_status": "healthy", "raw_stream_events": int64(1),
		"enriched_mints": int64(1), "processing_active": int64(0),
		"processing_stale_active": int64(0), "processing_completed": int64(1),
		"processing_completed_recent": int64(1), "processing_failed_recent": int64(0),
		"last_stream_event_at": time.Now().Format(time.RFC3339),
		"last_processed_at":    time.Now().Format(time.RFC3339),
	}
	if measuredArvisHealthStatus(stats) != "healthy" {
		t.Fatal("complete healthy observation rejected")
	}
	for key, value := range stats {
		if key == "pipeline_status" {
			continue
		}
		delete(stats, key)
		if measuredArvisHealthStatus(stats) != "unverified" {
			t.Errorf("missing observation %s was treated as healthy", key)
		}
		stats[key] = value
	}
}

func TestCachedArvisHealthSnapshotCopiesCachedData(t *testing.T) {
	resetArvisHealthCache()
	t.Cleanup(resetArvisHealthCache)
	arvisHealthCache.Lock()
	arvisHealthCache.data = map[string]any{"pipeline_status": "operational", "visible_verdicts": int64(3)}
	arvisHealthCache.expiresAt = time.Now().Add(time.Minute)
	arvisHealthCache.Unlock()

	snapshot := cachedArvisHealthSnapshot()
	if snapshot["pipeline_status"] != "operational" || snapshot["cached"] != true {
		t.Fatalf("snapshot=%#v", snapshot)
	}
	snapshot["pipeline_status"] = "mutated"
	arvisHealthCache.RLock()
	defer arvisHealthCache.RUnlock()
	if arvisHealthCache.data["pipeline_status"] != "operational" {
		t.Fatal("public snapshot mutated shared cache")
	}
}

func TestCachedArvisHealthSnapshotRejectsExpiredEvidence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		expiresAt time.Time
	}{
		{"expired", time.Now().Add(-time.Second)},
		{"expires_now", time.Now()},
		{"missing_expiry", time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetArvisHealthCache()
			t.Cleanup(resetArvisHealthCache)
			arvisHealthCache.Lock()
			arvisHealthCache.data = map[string]any{"pipeline_status": "healthy", "visible_verdicts": int64(3)}
			arvisHealthCache.expiresAt = tc.expiresAt
			arvisHealthCache.Unlock()

			recorder := httptest.NewRecorder()
			(&Handler{}).Health(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("stale evidence must not fail process liveness: %d", recorder.Code)
			}
			var body struct {
				Arvis map[string]any `json:"arvis"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Arvis["pipeline_status"] != "stale" || body.Arvis["cached"] != true {
				t.Fatalf("expired snapshot advertised a current pipeline: %#v", body.Arvis)
			}
			arvisHealthCache.RLock()
			defer arvisHealthCache.RUnlock()
			if arvisHealthCache.data["pipeline_status"] != "healthy" {
				t.Fatal("public snapshot mutated the cached observation")
			}
		})
	}
}

func TestPublicHealthStaysReadyInStatelessRuntimeWithoutLeakingDBError(t *testing.T) {
	resetArvisHealthCache()
	t.Cleanup(resetArvisHealthCache)
	t.Setenv("APP_ENV", "production")
	h := &Handler{DBInitError: "secret database connection detail"}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	started := time.Now()
	h.Health(recorder, req)
	if elapsed := time.Since(started); elapsed > publicHealthTimeout {
		t.Fatalf("health exceeded public timeout: %s", elapsed)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret database connection detail") {
		t.Fatalf("production health leaked database details: %s", recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, leaked := body["details"]; leaked {
		t.Fatalf("production health leaked database details: %#v", body)
	}
	if body["service"] != "koschei-web3" || body["status"] != "ok" || body["database"] != "not_used" || body["persistence"] != "stateless" {
		t.Fatalf("body=%#v", body)
	}
}
