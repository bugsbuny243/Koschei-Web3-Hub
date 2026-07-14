package services

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDeriveActorDefenseTrackState(t *testing.T) {
	tests := []struct {
		name    string
		track   ActorDefenseTrack
		related []ActorDefenseRelatedActor
		want    string
	}{
		{name: "single observation", track: ActorDefenseTrack{CreatedTokenCount: 1}, want: "detected"},
		{name: "multiple observed tokens", track: ActorDefenseTrack{CreatedTokenCount: 2}, want: "tracked"},
		{name: "repeat actor across tokens", track: ActorDefenseTrack{CreatedTokenCount: 2, RelatedActorCount: 1}, related: []ActorDefenseRelatedActor{{SharedTokenCount: 2}}, want: "correlated"},
		{name: "verified transaction evidence", track: ActorDefenseTrack{CreatedTokenCount: 1, VerifiedEvidenceCount: 1}, want: "verified"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DeriveActorDefenseTrackState(test.track, test.related); got != test.want {
				t.Fatalf("state=%q want=%q", got, test.want)
			}
		})
	}
}

func TestNormalizeActorEvidenceStatus(t *testing.T) {
	if got := normalizeActorEvidenceStatus("VERIFIED"); got != "verified" {
		t.Fatalf("verified normalization=%q", got)
	}
	if got := normalizeActorEvidenceStatus("anything-else"); got != "observed" {
		t.Fatalf("fallback normalization=%q", got)
	}
}

func TestActorDefenseCorrelationDefaultsAreBounded(t *testing.T) {
	previous, existed := os.LookupEnv("ACTOR_DEFENSE_CORRELATION_SECONDS")
	defer func() {
		if existed {
			_ = os.Setenv("ACTOR_DEFENSE_CORRELATION_SECONDS", previous)
		} else {
			_ = os.Unsetenv("ACTOR_DEFENSE_CORRELATION_SECONDS")
		}
	}()
	_ = os.Unsetenv("ACTOR_DEFENSE_CORRELATION_SECONDS")
	if got := actorDefenseCorrelationInterval(); got != 10*time.Minute {
		t.Fatalf("default interval=%s", got)
	}
	_ = os.Setenv("ACTOR_DEFENSE_CORRELATION_SECONDS", "30")
	if got := actorDefenseCorrelationInterval(); got != 10*time.Minute {
		t.Fatalf("unsafe fast interval was accepted: %s", got)
	}
	_ = os.Setenv("ACTOR_DEFENSE_CORRELATION_SECONDS", "900")
	if got := actorDefenseCorrelationInterval(); got != 15*time.Minute {
		t.Fatalf("configured interval=%s", got)
	}
}

func TestActorDefenseCorrelationQueriesUsePersistentActorIndex(t *testing.T) {
	for name, query := range map[string]string{
		"creator": actorDefenseCreatorCorrelationSQL,
		"holder":  actorDefenseRepeatHolderCorrelationSQL,
	} {
		lower := strings.ToLower(query)
		if !strings.Contains(lower, "security_actor_evidence") {
			t.Fatalf("%s correlation query does not use persistent actor memory", name)
		}
		if strings.Contains(lower, "security_radar_events") || strings.Contains(lower, "security_radar_holder_snapshots") {
			t.Fatalf("%s correlation query regressed to retention-bound raw sensor tables", name)
		}
		if strings.Contains(lower, "interval '30 days'") {
			t.Fatalf("%s correlation query must not forget actors after 30 days", name)
		}
		if strings.Contains(lower, "gettransaction") || strings.Contains(lower, "rpc") {
			t.Fatalf("%s correlation query must remain SQL-only", name)
		}
	}
}
