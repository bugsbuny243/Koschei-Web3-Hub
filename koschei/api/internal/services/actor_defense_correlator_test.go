package services

import (
	"strings"
	"testing"
)

func TestActorDefenseCorrelationQueriesAvoidWriteAmplification(t *testing.T) {
	for name, query := range map[string]string{
		"creator": actorDefenseCreatorCorrelationSQL,
		"holder":  actorDefenseRepeatHolderCorrelationSQL,
	} {
		if !strings.Contains(query, "ON CONFLICT (network,target_kind,target_id)") {
			t.Fatalf("%s query lost target-level upsert", name)
		}
		if !strings.Contains(query, "\nWHERE\n") {
			t.Fatalf("%s query updates every track on every cycle", name)
		}
		if strings.Contains(query, "last_investigated_at=EXCLUDED.last_investigated_at") {
			t.Fatalf("%s correlation query must not pretend background correlation is a live investigation", name)
		}
		if !strings.Contains(query, "NOT security_threat_tracks.dossier @> EXCLUDED.dossier") {
			t.Fatalf("%s query lost dossier change detection", name)
		}
	}
}
