package services

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestGlobalCampaignTemporalCorrelationArrivalOrderAndReplayStable(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	events := []GlobalCampaignTemporalEvent{
		{Ref: "obs:3", SubjectID: "subject:c", Network: "solana", Kind: "exit", ObservedAt: base.Add(12 * time.Minute), EvidenceState: "observed"},
		{
			Ref: "obs:1", SubjectID: "subject:a", Network: "solana", Kind: "funding", ObservedAt: base, EvidenceState: "verified",
			RelationRefs: []string{"relation:funded", "relation:funded"}, VerifiedRelationRefs: []string{"relation:funded"},
		},
		{
			Ref: "obs:2", SubjectID: "subject:b", Network: "solana", Kind: "creation", ObservedAt: base.Add(4 * time.Minute), EvidenceState: "verified",
			RelationRefs: []string{"relation:funded"}, VerifiedRelationRefs: []string{"relation:funded"},
		},
	}

	first, err := BuildGlobalCampaignTemporalCorrelation(" KCAM1-EXAMPLE ", events)
	if err != nil {
		t.Fatal(err)
	}
	shuffled := []GlobalCampaignTemporalEvent{events[1], events[2], events[0]}
	second, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-EXAMPLE", shuffled)
	if err != nil {
		t.Fatal(err)
	}
	if first.FingerprintSHA256 != second.FingerprintSHA256 {
		t.Fatalf("arrival order changed fingerprint: %s != %s", first.FingerprintSHA256, second.FingerprintSHA256)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("arrival order changed report:\n%#v\n%#v", first, second)
	}
	if first.VerifiedLinkCount != 1 {
		t.Fatalf("expected one explicit verified relation, got %d", first.VerifiedLinkCount)
	}
	if first.VerdictAuthority || first.GradeAuthority || first.ContainmentAuthority || first.SameOperatorClaim || first.RealWorldIdentityClaim || first.WrongdoingClaim {
		t.Fatalf("temporal report crossed authority boundary: %#v", first)
	}
}

func TestGlobalCampaignTemporalCorrelationDuplicateEvidenceIsIdempotent(t *testing.T) {
	at := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	event := GlobalCampaignTemporalEvent{Ref: "obs:1", SubjectID: "subject:a", Network: "solana", Kind: "funding", ObservedAt: at, EvidenceState: "verified"}
	one, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{event})
	if err != nil {
		t.Fatal(err)
	}
	two, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{event, event})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, two) {
		t.Fatalf("exact duplicate changed result:\n%#v\n%#v", one, two)
	}
}

func TestGlobalCampaignTemporalCorrelationDivergentDuplicateFailsClosed(t *testing.T) {
	at := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	_, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "obs:1", SubjectID: "subject:a", Network: "solana", Kind: "funding", ObservedAt: at, EvidenceState: "verified"},
		{Ref: "obs:1", SubjectID: "subject:a", Network: "solana", Kind: "creation", ObservedAt: at, EvidenceState: "verified"},
	})
	if !errors.Is(err, ErrGlobalCampaignTemporalEvidenceConflict) {
		t.Fatalf("expected divergent evidence conflict, got %v", err)
	}
}

func TestGlobalCampaignTemporalCorrelationMissingTimestampFailsClosed(t *testing.T) {
	_, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "obs:1", SubjectID: "subject:a", Network: "solana", Kind: "funding", EvidenceState: "verified"},
	})
	if !errors.Is(err, ErrGlobalCampaignTemporalTimestampRequired) {
		t.Fatalf("expected timestamp error, got %v", err)
	}
}

func TestGlobalCampaignTemporalVerifiedRefMustBeCanonicalContextRef(t *testing.T) {
	at := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	_, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{
			Ref: "obs:1", SubjectID: "subject:a", Network: "solana", Kind: "funding", ObservedAt: at, EvidenceState: "verified",
			VerifiedRelationRefs: []string{"relation:verified-but-not-context"},
		},
	})
	if !errors.Is(err, ErrGlobalCampaignTemporalVerifiedRefBoundary) {
		t.Fatalf("expected verified-ref boundary error, got %v", err)
	}
}

func TestGlobalCampaignTemporalWindowBoundariesAreInclusiveAndDeterministic(t *testing.T) {
	cases := []struct {
		delta time.Duration
		want  string
		ok    bool
	}{
		{delta: time.Minute, want: "1m", ok: true},
		{delta: time.Minute + time.Nanosecond, want: "5m", ok: true},
		{delta: 5 * time.Minute, want: "5m", ok: true},
		{delta: 15 * time.Minute, want: "15m", ok: true},
		{delta: time.Hour, want: "1h", ok: true},
		{delta: 24 * time.Hour, want: "24h", ok: true},
		{delta: 24*time.Hour + time.Nanosecond, want: "", ok: false},
	}
	for _, tc := range cases {
		got, ok := globalCampaignTemporalWindow(tc.delta)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("delta %s: got (%q,%v), want (%q,%v)", tc.delta, got, ok, tc.want, tc.ok)
		}
	}
}

func TestGlobalCampaignTemporalGenericRelationRefCannotVerify(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "obs:1", SubjectID: "subject:a", Network: "solana", Kind: "funding", ObservedAt: base, EvidenceState: "verified", RelationRefs: []string{"relation:context-only"}},
		{Ref: "obs:2", SubjectID: "subject:b", Network: "solana", Kind: "creation", ObservedAt: base.Add(time.Second), EvidenceState: "verified", RelationRefs: []string{"relation:context-only"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Correlations[0]
	if got.EvidenceState == "verified" || !got.WatchOnly || report.VerifiedLinkCount != 0 {
		t.Fatalf("generic relation ref was over-promoted: %#v / verified=%d", got, report.VerifiedLinkCount)
	}
	if len(got.RelationRefs) != 1 || len(got.VerifiedRelationRefs) != 0 {
		t.Fatalf("unexpected relation proof projection: %#v", got)
	}
}

func TestGlobalCampaignTemporalSameSubjectVerifiedRelationCannotVerify(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{
			Ref: "obs:1", SubjectID: "subject:a", Network: "solana", Kind: "funding", ObservedAt: base, EvidenceState: "verified",
			RelationRefs: []string{"relation:verified"}, VerifiedRelationRefs: []string{"relation:verified"},
		},
		{
			Ref: "obs:2", SubjectID: "subject:a", Network: "solana", Kind: "state", ObservedAt: base.Add(time.Second), EvidenceState: "verified",
			RelationRefs: []string{"relation:verified"}, VerifiedRelationRefs: []string{"relation:verified"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Correlations[0]
	if got.EvidenceState == "verified" || !got.WatchOnly || report.VerifiedLinkCount != 0 {
		t.Fatalf("same-subject observations were over-promoted: %#v / verified=%d", got, report.VerifiedLinkCount)
	}
}

func TestGlobalCampaignTemporalCrossNetworkTimingAloneCannotVerify(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "sol:tx:1", SubjectID: "subject:sol", Network: "solana", Kind: "bridge_source", ObservedAt: base, EvidenceState: "verified"},
		{Ref: "eth:tx:1", SubjectID: "subject:eth", Network: "ethereum", Kind: "bridge_destination", ObservedAt: base.Add(30 * time.Second), EvidenceState: "verified"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Correlations) != 1 {
		t.Fatalf("expected one correlation, got %d", len(report.Correlations))
	}
	got := report.Correlations[0]
	if got.EvidenceState == "verified" || !got.WatchOnly || !got.CrossNetwork {
		t.Fatalf("timing-only cross-network correlation was over-promoted: %#v", got)
	}
}

func TestGlobalCampaignTemporalGenericBridgeRefCannotVerify(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "sol:tx:1", SubjectID: "subject:sol", Network: "solana", Kind: "bridge_source", ObservedAt: base, EvidenceState: "verified", BridgeLinkRefs: []string{"bridge-link:context-only"}},
		{Ref: "eth:tx:1", SubjectID: "subject:eth", Network: "ethereum", Kind: "bridge_destination", ObservedAt: base.Add(30 * time.Second), EvidenceState: "verified", BridgeLinkRefs: []string{"bridge-link:context-only"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Correlations[0]
	if got.EvidenceState == "verified" || !got.WatchOnly || report.VerifiedLinkCount != 0 {
		t.Fatalf("generic bridge ref was over-promoted: %#v / verified=%d", got, report.VerifiedLinkCount)
	}
	if len(got.BridgeLinkRefs) != 1 || len(got.VerifiedBridgeLinkRefs) != 0 {
		t.Fatalf("unexpected bridge proof projection: %#v", got)
	}
}

func TestGlobalCampaignTemporalCrossNetworkVerifiedBridgeCanVerifyContinuity(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{
			Ref: "sol:tx:1", SubjectID: "subject:sol", Network: "solana", Kind: "bridge_source", ObservedAt: base, EvidenceState: "verified",
			BridgeLinkRefs: []string{"bridge-link:abc"}, VerifiedBridgeLinkRefs: []string{"bridge-link:abc"},
		},
		{
			Ref: "eth:tx:1", SubjectID: "subject:eth", Network: "ethereum", Kind: "bridge_destination", ObservedAt: base.Add(30 * time.Second), EvidenceState: "verified",
			BridgeLinkRefs: []string{"bridge-link:abc"}, VerifiedBridgeLinkRefs: []string{"bridge-link:abc"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Correlations[0]
	if got.EvidenceState != "verified" || got.WatchOnly || !got.CrossNetwork {
		t.Fatalf("explicit verified bridge was not preserved: %#v", got)
	}
	if !reflect.DeepEqual(got.VerifiedBridgeLinkRefs, []string{"bridge-link:abc"}) {
		t.Fatalf("unexpected verified bridge refs: %#v", got.VerifiedBridgeLinkRefs)
	}
}

func TestGlobalCampaignTemporalInferredOnlyStaysWatchOnly(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "hyp:1", SubjectID: "subject:a", Network: "solana", Kind: "hypothesis", ObservedAt: base, EvidenceState: "inferred", RelationRefs: []string{"relation:x"}},
		{Ref: "hyp:2", SubjectID: "subject:b", Network: "solana", Kind: "hypothesis", ObservedAt: base.Add(time.Second), EvidenceState: "inferred", RelationRefs: []string{"relation:x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Correlations[0]
	if got.EvidenceState != "inferred" || !got.WatchOnly || report.VerifiedLinkCount != 0 {
		t.Fatalf("inferred-only input escalated: %#v / verified=%d", got, report.VerifiedLinkCount)
	}
}

func TestGlobalCampaignTemporalInvalidatedEvidenceCannotRemainActive(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{
			Ref: "obs:reorged", SubjectID: "subject:a", Network: "ethereum", Kind: "transfer", ObservedAt: base, EvidenceState: "verified",
			RelationRefs: []string{"relation:x"}, VerifiedRelationRefs: []string{"relation:x"}, Invalidated: true,
		},
		{
			Ref: "obs:live", SubjectID: "subject:b", Network: "ethereum", Kind: "transfer", ObservedAt: base.Add(time.Second), EvidenceState: "verified",
			RelationRefs: []string{"relation:x"}, VerifiedRelationRefs: []string{"relation:x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ActiveEventCount != 1 || report.CorrelationCount != 0 || report.VerifiedLinkCount != 0 {
		t.Fatalf("invalidated evidence remained active: %#v", report)
	}
	if !reflect.DeepEqual(report.InvalidatedRefs, []string{"obs:reorged"}) {
		t.Fatalf("unexpected invalidated refs: %#v", report.InvalidatedRefs)
	}
}

func TestGlobalCampaignTemporalSignedArtifactCannotUpgradeCorrelation(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{
			Ref: "artifact:1", SubjectID: "subject:a", Network: "solana", Kind: "signed_verdict", ObservedAt: base, EvidenceState: "signed_artifact",
			RelationRefs: []string{"relation:x"}, VerifiedRelationRefs: []string{"relation:x"},
		},
		{
			Ref: "obs:1", SubjectID: "subject:b", Network: "solana", Kind: "transfer", ObservedAt: base.Add(time.Second), EvidenceState: "verified",
			RelationRefs: []string{"relation:x"}, VerifiedRelationRefs: []string{"relation:x"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Correlations[0]
	if got.EvidenceState == "verified" || !got.WatchOnly {
		t.Fatalf("signed artifact upgraded temporal evidence: %#v", got)
	}
}

func TestGlobalCampaignTemporalEventBudgetFailsClosed(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	events := make([]GlobalCampaignTemporalEvent, GlobalCampaignTemporalMaxEvents+1)
	for i := range events {
		events[i] = GlobalCampaignTemporalEvent{
			Ref: fmt.Sprintf("obs:%05d", i), SubjectID: fmt.Sprintf("subject:%05d", i), Network: "solana", Kind: "transfer", ObservedAt: base, EvidenceState: "observed",
		}
	}
	_, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", events)
	if !errors.Is(err, ErrGlobalCampaignTemporalBudgetExceeded) {
		t.Fatalf("expected event budget failure, got %v", err)
	}
}

func TestGlobalCampaignTemporalCorrelationBudgetFailsClosed(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	// 142 events at one timestamp produce 10,011 candidate pairs, which must
	// fail closed instead of silently truncating at the 10,000 correlation cap.
	events := make([]GlobalCampaignTemporalEvent, 142)
	for i := range events {
		events[i] = GlobalCampaignTemporalEvent{
			Ref: fmt.Sprintf("obs:%03d", i), SubjectID: fmt.Sprintf("subject:%03d", i), Network: "solana", Kind: "transfer", ObservedAt: base, EvidenceState: "observed",
		}
	}
	_, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", events)
	if !errors.Is(err, ErrGlobalCampaignTemporalBudgetExceeded) {
		t.Fatalf("expected correlation budget failure, got %v", err)
	}
}
