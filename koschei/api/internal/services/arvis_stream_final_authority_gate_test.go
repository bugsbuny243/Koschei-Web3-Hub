package services

import (
	"errors"
	"testing"
)

func TestArvisStreamWorkerIsFailClosedUntilCanonicalFinalWired(t *testing.T) {
	if arvisStreamCanonicalFinalPipelineWired {
		t.Fatal("stream verdict worker must remain withheld until canonical signed final + durable persistence acceptance")
	}
}

func TestArvisStreamFinalAuthorityGateDistinguishesEvidenceGapFromMissingAuthority(t *testing.T) {
	// No verified collector evidence is a real investigation evidence gap.
	empty := SecurityRadarBundle{}
	if err := arvisStreamFinalAuthorityGate(empty); !errors.Is(err, errArvisStreamInsufficientEvidence) {
		t.Fatalf("no evidence: got %v; want insufficient evidence", err)
	}

	// A verified arm is *not* a signed canonical decision. The current
	// ArvisFinalFromBundle is deliberately an unsigned compatibility placeholder.
	live := SecurityRadarBundle{
		Metadata: map[string]any{
			"arvis_arms": []SecurityRadarVerdict{
				{ModuleID: ModuleHolderConcentration, EvidenceVerified: true},
			},
		},
	}
	if !SecurityRadarHasLiveEvidence(live) {
		t.Fatal("test setup: verified evidence must be detectable")
	}
	err := arvisStreamFinalAuthorityGate(live)
	if !errors.Is(err, errArvisStreamCanonicalFinalUnavailable) {
		t.Fatalf("verified evidence but unsigned final: got %v; want canonical authority failure", err)
	}
	if errors.Is(err, errArvisStreamInsufficientEvidence) {
		t.Fatal("integration failure was incorrectly classified as missing evidence")
	}
}

func TestArvisStreamFinalAuthorityGateRejectsMissingFinalPersistence(t *testing.T) {
	live := SecurityRadarBundle{
		Metadata: map[string]any{
			"arvis_arms": []SecurityRadarVerdict{
				{ModuleID: ModuleProgramRelationScan, EvidenceVerified: true},
			},
		},
	}
	// Even a forged legacy final-verdict arm cannot establish canonical authority.
	// An explicit signed-final proof and network/target binding must be wired
	// through a separate accepted runtime path before promotion can occur.
	live.Metadata["arvis_arms"] = append(live.Metadata["arvis_arms"].([]SecurityRadarVerdict),
		SecurityRadarVerdict{ModuleID: ModuleFinalVerdictEngine, EvidenceVerified: true})
	err := arvisStreamFinalAuthorityGate(live)
	if !errors.Is(err, errArvisStreamCanonicalFinalUnavailable) {
		t.Fatalf("no final persistence: got %v; want fail-closed authority error", err)
	}
}
