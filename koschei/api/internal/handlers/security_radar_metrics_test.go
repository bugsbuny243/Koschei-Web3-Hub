package handlers

import "testing"

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

func TestArvisPipelineStatus(t *testing.T) {
	cases := []struct {
		name    string
		metrics map[string]any
		want    string
	}{
		{name: "processing", metrics: map[string]any{"processing_active": int64(1)}, want: "processing"},
		{name: "healthy", metrics: map[string]any{"processing_completed": int64(4)}, want: "healthy"},
		{name: "degraded with output", metrics: map[string]any{"processing_completed": int64(4), "processing_failed": int64(1)}, want: "degraded"},
		{name: "degraded only", metrics: map[string]any{"processing_failed": int64(2)}, want: "degraded"},
		{name: "waiting target", metrics: map[string]any{"raw_stream_events": int64(3)}, want: "waiting_for_enriched_targets"},
		{name: "waiting stream", metrics: map[string]any{}, want: "waiting_for_stream"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := arvisPipelineStatus(tc.metrics); got != tc.want {
				t.Fatalf("arvisPipelineStatus()=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestMetricInt64(t *testing.T) {
	metrics := map[string]any{"int64": int64(7), "int": 8, "float": float64(9)}
	if metricInt64(metrics, "int64") != 7 || metricInt64(metrics, "int") != 8 || metricInt64(metrics, "float") != 9 {
		t.Fatal("metricInt64 failed numeric conversion")
	}
	if metricInt64(metrics, "missing") != 0 {
		t.Fatal("missing metric should return zero")
	}
}
