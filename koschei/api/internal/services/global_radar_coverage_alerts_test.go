package services

import (
	"testing"

	"koschei/api/internal/runtimehealth"
)

func TestCoverageLifecycleAction(t *testing.T) {
	open := coverageEpisode{
		ID:             "episode-1",
		ComponentID:    "head",
		CoverageStatus: runtimehealth.CoverageBlindSpot,
		CoverageReason: "head_ingest_freshness_deadline_expired",
		OpenAlertID:    "alert-1",
	}
	tests := []struct {
		name    string
		entry   runtimehealth.Entry
		hasOpen bool
		open    coverageEpisode
		want    string
	}{
		{
			name: "new attention opens episode",
			entry: runtimehealth.Entry{
				Kind:              "head_ingest",
				CoverageStatus:    runtimehealth.CoverageBlindSpot,
				CoverageReason:    "head_ingest_freshness_deadline_expired",
				CoverageAttention: true,
			},
			want: "open",
		},
		{
			name: "missing alert is repaired",
			entry: runtimehealth.Entry{
				Kind:              "head_ingest",
				CoverageStatus:    runtimehealth.CoverageBlindSpot,
				CoverageReason:    "head_ingest_freshness_deadline_expired",
				CoverageAttention: true,
			},
			hasOpen: true,
			open: coverageEpisode{
				ID:             "episode-1",
				CoverageStatus: runtimehealth.CoverageBlindSpot,
				CoverageReason: "head_ingest_freshness_deadline_expired",
			},
			want: "ensure_alert",
		},
		{
			name: "escalation updates same episode",
			entry: runtimehealth.Entry{
				Kind:              "head_ingest",
				CoverageStatus:    runtimehealth.CoverageReorgGuard,
				CoverageReason:    "canonical_lineage_recheck_required",
				CoverageAttention: true,
			},
			hasOpen: true,
			open:    open,
			want:    "update",
		},
		{
			name: "same attention stays deduped",
			entry: runtimehealth.Entry{
				Kind:              "head_ingest",
				CoverageStatus:    runtimehealth.CoverageBlindSpot,
				CoverageReason:    "head_ingest_freshness_deadline_expired",
				CoverageAttention: true,
			},
			hasOpen: true,
			open:    open,
			want:    "none",
		},
		{
			name: "catching up does not close episode",
			entry: runtimehealth.Entry{
				Kind:           "head_ingest",
				CoverageStatus: runtimehealth.CoverageLagging,
				CoverageReason: "durable_cursor_is_behind_observed_head",
			},
			hasOpen: true,
			open:    open,
			want:    "none",
		},
		{
			name: "current recovers episode",
			entry: runtimehealth.Entry{
				Kind:           "head_ingest",
				CoverageStatus: runtimehealth.CoverageCurrent,
				CoverageReason: "durable_cursor_matches_observed_provider_head",
			},
			hasOpen: true,
			open:    open,
			want:    "recover",
		},
		{
			name: "non ingest is ignored",
			entry: runtimehealth.Entry{
				Kind:              "storage",
				CoverageAttention: true,
			},
			hasOpen: true,
			open:    open,
			want:    "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coverageLifecycleAction(tt.entry, tt.hasOpen, tt.open); got != tt.want {
				t.Fatalf("action=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestCoverageAlertSeverity(t *testing.T) {
	if got := coverageAlertSeverity(runtimehealth.CoverageBlindSpot); got != "high" {
		t.Fatalf("blind spot severity=%q", got)
	}
	if got := coverageAlertSeverity(runtimehealth.CoverageReorgGuard); got != "high" {
		t.Fatalf("reorg severity=%q", got)
	}
	if got := coverageAlertSeverity(runtimehealth.CoverageLagging); got != "medium" {
		t.Fatalf("lagging severity=%q", got)
	}
	if got := coverageAlertSeverity(runtimehealth.CoverageCurrent); got != "info" {
		t.Fatalf("current severity=%q", got)
	}
}
