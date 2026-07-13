package services

import "testing"

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
