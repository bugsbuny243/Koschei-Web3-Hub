package handlers

import (
	"database/sql"
	"strings"
	"testing"
)

func TestIncidentVerdictLineageRequiresExactSinglePersistedRecord(t *testing.T) {
	base := ownerIncidentAlertSource{Source: "arvis", EventType: "arvis.verdict.created"}
	for _, tc := range []struct {
		name string
		src  ownerIncidentAlertSource
		want string
	}{
		{"unresolved", base, "no_exact_persisted_verdict_match"},
		{"ambiguous", ownerIncidentAlertSource{Source: base.Source, EventType: base.EventType, VerdictMatches: 2}, "ambiguous_persisted_verdict_matches"},
		{"incomplete", ownerIncidentAlertSource{Source: base.Source, EventType: base.EventType, VerdictMatches: 1}, "persisted_verdict_metadata_incomplete"},
		{"not_arvis", ownerIncidentAlertSource{Source: "global_radar_runtime", EventType: "global_radar.coverage.attention", VerdictMatches: 1}, "not_arvis_verdict_alert"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := incidentReviewVerdictLineage(tc.src)
			if got.Status != tc.want || got.VerdictID != "" || got.Network != "" {
				t.Fatalf("verdict identity invented: %#v", got)
			}
		})
	}
	confirmed := base
	confirmed.VerdictMatches = 1
	confirmed.VerdictID = sql.NullString{String: "11111111-1111-4111-8111-111111111111", Valid: true}
	confirmed.VerdictNetwork = sql.NullString{String: "solana-mainnet", Valid: true}
	got := incidentReviewVerdictLineage(confirmed)
	if got.Status != "matched_persisted_signed_verdict_metadata_only" ||
		got.VerdictID != confirmed.VerdictID.String || got.Network != "solana-mainnet" ||
		!strings.Contains(got.Authority, "not_independent_chain") {
		t.Fatalf("persisted metadata lineage did not preserve trust boundary: %#v", got)
	}
	if ownerIncidentReviewCandidateFromAlert(confirmed).ChainEvidenceVerified {
		t.Fatal("matching signed metadata must not prove independent chain verification")
	}
}
