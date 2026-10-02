package services

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestGlobalCampaignTemporalCorrelationArrivalOrderAndReplayStable(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	events := []GlobalCampaignTemporalEvent{
		{Ref: "obs:3", Network: "solana", Kind: "exit", ObservedAt: base.Add(12 * time.Minute), EvidenceState: "observed"},
		{Ref: "obs:1", Network: "solana", Kind: "funding", ObservedAt: base, EvidenceState: "verified", RelationRefs: []string{"relation:funded", "relation:funded"}},
		{Ref: "obs:2", Network: "solana", Kind: "creation", ObservedAt: base.Add(4 * time.Minute), EvidenceState: "verified", RelationRefs: []string{"relation:funded"}},
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
	event := GlobalCampaignTemporalEvent{Ref: "obs:1", Network: "solana", Kind: "funding", ObservedAt: at, EvidenceState: "verified"}
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
		{Ref: "obs:1", Network: "solana", Kind: "funding", ObservedAt: at, EvidenceState: "verified"},
		{Ref: "obs:1", Network: "solana", Kind: "creation", ObservedAt: at, EvidenceState: "verified"},
	})
	if !errors.Is(err, ErrGlobalCampaignTemporalEvidenceConflict) {
		t.Fatalf("expected divergent evidence conflict, got %v", err)
	}
}

func TestGlobalCampaignTemporalCorrelationMissingTimestampFailsClosed(t *testing.T) {
	_, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "obs:1", Network: "solana", Kind: "funding", EvidenceState: "verified"},
	})
	if !errors.Is(err, ErrGlobalCampaignTemporalTimestampRequired) {
		t.Fatalf("expected timestamp error, got %v", err)
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

func TestGlobalCampaignTemporalCrossNetworkTimingAloneCannotVerify(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "sol:tx:1", Network: "solana", Kind: "bridge_source", ObservedAt: base, EvidenceState: "verified"},
		{Ref: "eth:tx:1", Network: "ethereum", Kind: "bridge_destination", ObservedAt: base.Add(30 * time.Second), EvidenceState: "verified"},
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

func TestGlobalCampaignTemporalCrossNetworkVerifiedBridgeCanVerifyContinuity(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "sol:tx:1", Network: "solana", Kind: "bridge_source", ObservedAt: base, EvidenceState: "verified", BridgeLinkRefs: []string{"bridge-link:abc"}},
		{Ref: "eth:tx:1", Network: "ethereum", Kind: "bridge_destination", ObservedAt: base.Add(30 * time.Second), EvidenceState: "verified", BridgeLinkRefs: []string{"bridge-link:abc"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Correlations[0]
	if got.EvidenceState != "verified" || got.WatchOnly || !got.CrossNetwork {
		t.Fatalf("explicit verified bridge was not preserved: %#v", got)
	}
	if !reflect.DeepEqual(got.BridgeLinkRefs, []string{"bridge-link:abc"}) {
		t.Fatalf("unexpected bridge refs: %#v", got.BridgeLinkRefs)
	}
}

func TestGlobalCampaignTemporalInferredOnlyStaysWatchOnly(t *testing.T) {
	base := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	report, err := BuildGlobalCampaignTemporalCorrelation("KCAM1-A", []GlobalCampaignTemporalEvent{
		{Ref: "hyp:1", Network: "solana", Kind: "hypothesis", ObservedAt: base, EvidenceState: "inferred", RelationRefs: []string{"relation:x"}},
		{Ref: "hyp:2", Network: "solana", Kind: "hypothesis", ObservedAt: base.Add(time.Second), EvidenceState: "inferred", RelationRefs: []string{"relation:x"}},
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
		{Ref: "obs:reorged", Network: "ethereum", Kind: "transfer", ObservedAt: base, EvidenceState: "verified", RelationRefs: []string{"relation:x"}, Invalidated: true},
		{Ref: "obs:live", Network: "ethereum", Kind: "transfer", ObservedAt: base.Add(time.Second), EvidenceState: "verified", RelationRefs: []string{"relation:x"}},
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
		{Ref: "artifact:1", Network: "solana", Kind: "signed_verdict", ObservedAt: base, EvidenceState: "signed_artifact", RelationRefs: []string{"relation:x"}},
		{Ref: "obs:1", Network: "solana", Kind: "transfer", ObservedAt: base.Add(time.Second), EvidenceState: "verified", RelationRefs: []string{"relation:x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := report.Correlations[0]
	if got.EvidenceState == "verified" || !got.WatchOnly {
		t.Fatalf("signed artifact upgraded temporal evidence: %#v", got)
	}
}
