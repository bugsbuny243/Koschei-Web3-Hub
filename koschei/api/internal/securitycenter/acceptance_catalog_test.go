package securitycenter

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func TestAcceptanceCatalogCovers170ChecksAndEightGlobalGates(t *testing.T) {
	catalog := CurrentAcceptanceCatalog()
	if catalog.SchemaVersion != AcceptanceCatalogSchemaVersion {
		t.Fatalf("unexpected schema %q", catalog.SchemaVersion)
	}
	if catalog.AreaCount != 34 || catalog.ControlCount != 170 || catalog.CommonGateCount != 8 || len(catalog.Checks) != 178 {
		t.Fatalf("incomplete security acceptance register: areas=%d controls=%d gates=%d checks=%d",
			catalog.AreaCount, catalog.ControlCount, catalog.CommonGateCount, len(catalog.Checks))
	}
	seen := make(map[string]bool)
	perArea := make(map[string]int)
	idPattern := regexp.MustCompile(`^(WSC-[0-9]{2}\.[1-5]|G0[1-8])$`)
	for _, check := range catalog.Checks {
		if !idPattern.MatchString(check.ID) || seen[check.ID] {
			t.Fatalf("duplicate or malformed control id: %q", check.ID)
		}
		seen[check.ID] = true
		if strings.TrimSpace(check.Description) == "" || strings.TrimSpace(check.Lead) == "" || check.Phase < 1 || check.Phase > 5 {
			t.Fatalf("untraceable control: %+v", check)
		}
		if check.Verification != AcceptanceUnverified || check.Applicability != AcceptanceNotAssessed || len(check.EvidenceRefs) != 0 {
			t.Fatalf("catalog alone cannot establish successful acceptance: %+v", check)
		}
		if strings.HasPrefix(check.ID, "WSC-") {
			perArea[check.AreaID]++
		}
	}
	for i := 1; i <= 34; i++ {
		id := fmt.Sprintf("%02d", i)
		if perArea[id] != 5 {
			t.Fatalf("area %s has %d controls, expected five", id, perArea[id])
		}
		for c := 1; c <= 5; c++ {
			if !seen[fmt.Sprintf("WSC-%02d.%d", i, c)] {
				t.Fatalf("area %s has a missing control", id)
			}
		}
	}
	for i := 1; i <= 8; i++ {
		if !seen[fmt.Sprintf("G%02d", i)] {
			t.Fatalf("common acceptance gate G%02d missing", i)
		}
	}
	if catalog.VerifiedCount != 0 {
		t.Fatal("a static capability catalog must not claim validated, live enforcement")
	}
}

func TestAcceptanceCatalogDoesNotMutateBetweenRequests(t *testing.T) {
	first := CurrentAcceptanceCatalog()
	first.Checks[0].EvidenceRefs = append(first.Checks[0].EvidenceRefs, "fabricated-proof")
	first.Checks[0].Verification = "verified"
	first.Areas[0].Name = "changed"
	second := CurrentAcceptanceCatalog()
	if second.Checks[0].Verification != AcceptanceUnverified || len(second.Checks[0].EvidenceRefs) != 0 {
		t.Fatal("acceptance snapshot state leaked across requests")
	}
	if second.Areas[0].Name == "changed" {
		t.Fatal("area catalog state leaked across requests")
	}
}

func TestSecurityCenterAcceptanceUsesARVISOwnershipOnly(t *testing.T) {
	snapshot := CurrentAcceptanceCatalog()
	for _, area := range snapshot.Areas {
		if area.Lead != "ARVIS" {
			t.Fatalf("area %s has unexpected separate product %q", area.ID, area.Lead)
		}
	}
	for _, check := range snapshot.Checks {
		if check.Lead != "ARVIS" {
			t.Fatalf("control %s has unexpected separate product %q", check.ID, check.Lead)
		}
	}
}
