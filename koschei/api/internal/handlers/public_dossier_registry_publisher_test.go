package handlers

import (
	"testing"
	"time"
)

func TestPublicDossierRegistrySnapshotMaxRowsDefaultsAndBounds(t *testing.T) {
	t.Setenv("KOSCHEI_PUBLIC_REGISTRY_SNAPSHOT_MAX_ROWS", "")
	got, err := publicDossierRegistrySnapshotMaxRows()
	if err != nil {
		t.Fatalf("default max rows: %v", err)
	}
	if got != defaultPublicCaseRegistrySnapshotMaxRows {
		t.Fatalf("default max rows=%d want=%d", got, defaultPublicCaseRegistrySnapshotMaxRows)
	}

	t.Setenv("KOSCHEI_PUBLIC_REGISTRY_SNAPSHOT_MAX_ROWS", "25000")
	got, err = publicDossierRegistrySnapshotMaxRows()
	if err != nil {
		t.Fatalf("configured max rows: %v", err)
	}
	if got != 25000 {
		t.Fatalf("configured max rows=%d want=25000", got)
	}

	for _, bad := range []string{"0", "-1", "not-a-number", "100001"} {
		t.Setenv("KOSCHEI_PUBLIC_REGISTRY_SNAPSHOT_MAX_ROWS", bad)
		if _, err := publicDossierRegistrySnapshotMaxRows(); err == nil {
			t.Fatalf("expected invalid max rows %q to fail", bad)
		}
	}
}

func TestPublicDossierRegistrySnapshotFromLoadPreservesCompletenessTruth(t *testing.T) {
	generatedAt := time.Date(2026, 9, 8, 7, 0, 0, 0, time.UTC)
	complete := publicDossierRegistrySnapshotFromLoad(publicDossierCasesV2Load{
		Cases:                      []publicDossierCaseV2{},
		TotalPublications:          0,
		InspectedPublications:      0,
		InvalidPublications:        0,
		UninspectedPublications:    0,
		InvalidLedgerPublications:  0,
		LegacyUnlinkedPublications: 0,
	}, generatedAt)
	if !complete.RegistryComplete || complete.RegistryStatus != "operational" {
		t.Fatalf("empty verified registry must be complete/operational: %+v", complete)
	}
	if !complete.PublicationLedgerComplete || complete.PublicationLedgerStatus != "verified" {
		t.Fatalf("empty verified publication ledger must be complete/verified: %+v", complete)
	}
	if complete.PublicationPolicy["canonical_bundle_hash_reverified"] != true {
		t.Fatal("snapshot must declare canonical bundle re-verification")
	}
	if complete.PublicationPolicy["transition_identifiers_public"] != false {
		t.Fatal("snapshot must keep transition identifiers private")
	}

	partial := publicDossierRegistrySnapshotFromLoad(publicDossierCasesV2Load{
		Cases:                   []publicDossierCaseV2{},
		TotalPublications:       5,
		InspectedPublications:   3,
		UninspectedPublications: 2,
	}, generatedAt)
	if partial.RegistryComplete || partial.RegistryStatus != "partial" {
		t.Fatalf("uninspected publications must remain partial: %+v", partial)
	}
	if partial.PublicationLedgerComplete || partial.PublicationLedgerStatus != "partial" {
		t.Fatalf("uninspected publication ledger must remain partial: %+v", partial)
	}
}
