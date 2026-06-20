package handlers

import (
	"testing"
	"time"
)

func TestRadarSignalsVerifiedAcceptsOnchainAndOffchainEvidence(t *testing.T) {
	cases := []struct {
		name    string
		signals map[string]any
		want    bool
	}{
		{name: "verified", signals: map[string]any{"verified_evidence": true}, want: true},
		{name: "onchain", signals: map[string]any{"real_onchain_evidence": true}, want: true},
		{name: "offchain", signals: map[string]any{"real_offchain_evidence": true}, want: true},
		{name: "missing", signals: map[string]any{}, want: false},
		{name: "nil", signals: nil, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := radarSignalsVerified(tc.signals); got != tc.want {
				t.Fatalf("radarSignalsVerified()=%t want=%t", got, tc.want)
			}
		})
	}
}

func TestArvisPipelineStatusUsesFreshness(t *testing.T) {
	now := time.Now().UTC()
	recentEvent := now.Add(-2 * time.Minute).Format(time.RFC3339Nano)
	recentProcessed := now.Add(-1 * time.Minute).Format(time.RFC3339Nano)
	staleEvent := now.Add(-20 * time.Minute).Format(time.RFC3339Nano)
	cases := []struct {
		name    string
		metrics map[string]any
		want    string
	}{
		{name: "processing", metrics: map[string]any{"processing_active": int64(1)}, want: "processing"},
		{name: "degraded stale lease", metrics: map[string]any{"processing_active": int64(1), "processing_stale_active": int64(1)}, want: "degraded"},
		{name: "degraded recent failure", metrics: map[string]any{"processing_failed_recent": int64(1)}, want: "degraded"},
		{name: "healthy recent completion", metrics: map[string]any{"raw_stream_events": int64(3), "enriched_mints": int64(1), "processing_completed": int64(4), "processing_completed_recent": int64(1), "last_stream_event_at": recentEvent, "last_processed_at": recentProcessed}, want: "healthy"},
		{name: "waiting processing", metrics: map[string]any{"raw_stream_events": int64(3), "enriched_mints": int64(1), "last_stream_event_at": recentEvent}, want: "waiting_for_processing"},
		{name: "waiting enriched target", metrics: map[string]any{"raw_stream_events": int64(3), "last_stream_event_at": recentEvent}, want: "waiting_for_enriched_targets"},
		{name: "stale stream", metrics: map[string]any{"raw_stream_events": int64(3), "enriched_mints": int64(1), "last_stream_event_at": staleEvent}, want: "stale"},
		{name: "waiting stream", metrics: map[string]any{}, want: "waiting_for_stream"},
		{name: "owner summary freshness", metrics: map[string]any{"sbx1_stream_events": int64(2), "processing_completed": int64(3), "processing_completed_recent": int64(1), "sbx1_last_event_at": recentEvent, "last_processed_at": recentProcessed}, want: "healthy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := arvisPipelineStatus(tc.metrics); got != tc.want {
				t.Fatalf("arvisPipelineStatus()=%q want=%q metrics=%#v", got, tc.want, tc.metrics)
			}
		})
	}
}

func TestMetricHelpers(t *testing.T) {
	metrics := map[string]any{"int64": int64(7), "int": 8, "float": float64(9), "rfc3339": "2026-06-20T01:02:03Z", "postgres": "2026-06-20 01:02:03+00"}
	if metricInt64(metrics, "int64") != 7 || metricInt64(metrics, "int") != 8 || metricInt64(metrics, "float") != 9 {
		t.Fatal("metricInt64 failed numeric conversion")
	}
	if metricInt64(metrics, "missing") != 0 {
		t.Fatal("missing metric should return zero")
	}
	if metricTime(metrics, "rfc3339").IsZero() {
		t.Fatal("RFC3339 metric time was not parsed")
	}
	if metricTime(metrics, "postgres").IsZero() {
		t.Fatal("Postgres metric time was not parsed")
	}
	if !metricTime(metrics, "missing").IsZero() {
		t.Fatal("missing metric time should be zero")
	}
}
