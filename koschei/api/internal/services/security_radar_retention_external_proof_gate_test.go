package services

import (
	"context"
	"strings"
	"testing"
	"time"
)

// The current PostgreSQL archive/delete SQL does not authenticate a durable
// external receipt for the exact source row BEFORE deletion. A successful
// connection probe or an export of an older backlog cannot supply that proof.
func TestRetentionSourceDeletionRequiresExternalRowProof(t *testing.T) {
	const reason = "external_archive_proof_before_source_delete_not_wired"
	if err := retentionRequireExternallyVerifiedSourceArchive(); err == nil || err.Error() != reason {
		t.Fatalf("missing explicit unverified-external-archive gate: %v", err)
	}
	cases := []struct {
		name string
		run  func(*runStats) int64
	}{
		{
			name: "current_v2",
			run: func(stats *runStats) int64 {
				worker := &securityRadarRetentionWorkerV2{}
				return worker.archiveAndDelete(context.Background(), "", retentionTarget{}, time.Now(), stats)
			},
		},
		{
			name: "rollback_v1",
			run: func(stats *runStats) int64 {
				worker := &securityRadarRetentionWorker{}
				return worker.archiveAndDelete(context.Background(), "", retentionTarget{}, time.Now(), stats)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stats := &runStats{PerTable: map[string]int64{}}
			if deleted := tc.run(stats); deleted != 0 {
				t.Fatalf("deleted %d original rows without independently verified archive receipt", deleted)
			}
			if stats.HaltReason != reason || !strings.Contains(stats.HaltReason, "not_wired") {
				t.Fatalf("missing fail-closed external receipt reason: %+v", stats)
			}
			if stats.Selected != 0 || stats.Archived != 0 || stats.Verified != 0 ||
				stats.Deleted != 0 || stats.Pruned != 0 {
				t.Fatalf("unverified external receipt changed progress counters: %+v", stats)
			}
		})
	}
}
