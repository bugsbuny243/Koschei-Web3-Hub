package handlers

import (
	"strings"
	"testing"

	"koschei/api/internal/defense"
	"koschei/api/internal/securityevidence"
	"koschei/api/internal/services"
)

func TestDefenseValidationUnifiedSecurityProjectsAuthenticatedCasesWithoutRegrading(t *testing.T) {
	response, err := evaluateDefenseValidationAPIRequest(defenseValidationAPITestRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	unified := response.UnifiedSecurityContract
	if unified.ContractVersion != services.UnifiedSecurityContractVersion {
		t.Fatalf("contract version=%q", unified.ContractVersion)
	}
	if unified.Base.Decision.Status != services.IntelligenceEvidenceUnverified || unified.Base.Decision.Action != "investigate" {
		t.Fatalf("unified projection regraded source report: %#v", unified.Base.Decision)
	}
	if len(unified.Base.Subjects) != 2 {
		t.Fatalf("expected two authenticated validation-case subjects, got %d", len(unified.Base.Subjects))
	}
	if len(unified.Actions) != 2 {
		t.Fatalf("expected two validated case actions, got %d", len(unified.Actions))
	}
	if len(unified.AttackPaths) != 0 {
		t.Fatalf("validated caught-in-time/clean run unexpectedly produced a defense-gap path: %#v", unified.AttackPaths)
	}
	if len(unified.Base.Evidence) != 4 {
		t.Fatalf("expected signed + deterministic evidence for each case, got %d rows", len(unified.Base.Evidence))
	}
	for _, subject := range unified.Base.Subjects {
		if subject.Kind != services.IntelligenceSubjectValidationCase {
			t.Fatalf("unexpected subject kind %q", subject.Kind)
		}
		if subject.ChainFamily != services.IntelligenceChainFamilyEVM {
			t.Fatalf("expected EVM validation subject, got %q", subject.ChainFamily)
		}
		if subject.Network != "eip155:31337" {
			t.Fatalf("unexpected validation network %q", subject.Network)
		}
	}
}

func TestDefenseValidationUnifiedSecurityEmitsVerifiedLateDetectionGap(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	attack := &request.Cases[0]
	late := int64(1500)
	attack.ObservationBinding.AlertObservedOffsetMS = &late
	bindingDigest, err := defense.DefenseValidationObservationBindingDigestV02(*attack.ObservationBinding)
	if err != nil {
		t.Fatal(err)
	}

	event := *attack.ObservationEvent
	event.Authentication = nil
	event.EventSHA256 = ""
	event.Findings = append([]securityevidence.Finding(nil), event.Findings...)
	if len(event.Findings) != 1 {
		t.Fatalf("unexpected test finding count %d", len(event.Findings))
	}
	event.Findings[0].EvidenceSHA256 = bindingDigest
	signed, err := event.SignEd25519(defenseValidationAPITestPrivateKey())
	if err != nil {
		t.Fatal(err)
	}
	attack.ObservationEvent = &signed

	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Report.Verdict != defense.DefenseValidationVerdictFailedV02 {
		t.Fatalf("late detection should fail defense validation, got %q", response.Report.Verdict)
	}
	unified := response.UnifiedSecurityContract
	if len(unified.AttackPaths) != 1 {
		t.Fatalf("expected one verified defense-gap path, got %d: %#v", len(unified.AttackPaths), unified.AttackPaths)
	}
	path := unified.AttackPaths[0]
	if path.Status != services.IntelligenceEvidenceVerified {
		t.Fatalf("late-detection path status=%q", path.Status)
	}
	if len(path.Steps) != 1 || path.Steps[0].ActionID == "" || len(path.Steps[0].EvidenceRefs) == 0 {
		t.Fatalf("gap path is not evidence/action linked: %#v", path)
	}
	if len(path.Consequences) != 1 || path.Consequences[0].Kind != "defense_validation_gap" || path.Consequences[0].Status != services.IntelligenceEvidenceVerified {
		t.Fatalf("unexpected gap consequence: %#v", path.Consequences)
	}
	if !containsString(path.Preconditions, "not_a_production_exploit_or_compromise_claim") {
		t.Fatalf("production-claim boundary missing from gap path: %#v", path.Preconditions)
	}
	if unified.Base.Decision.Status != services.IntelligenceEvidenceUnverified {
		t.Fatalf("source defense verdict was incorrectly copied into customer decision: %#v", unified.Base.Decision)
	}
}

func TestDefenseValidationUnifiedSecurityMissingObservationStaysEvidenceEmpty(t *testing.T) {
	request := defenseValidationAPITestRequest(t)
	for index := range request.Cases {
		request.Cases[index].ObservationBinding = nil
		request.Cases[index].ObservationEvent = nil
	}
	response, err := evaluateDefenseValidationAPIRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	unified := response.UnifiedSecurityContract
	if len(unified.Base.Subjects) != 0 || len(unified.Base.Evidence) != 0 || len(unified.Actions) != 0 || len(unified.AttackPaths) != 0 {
		t.Fatalf("missing independent observations produced unified claims: %#v", unified)
	}
	if unified.Base.Decision.Status != services.IntelligenceEvidenceUnverified {
		t.Fatalf("missing observations changed decision status: %q", unified.Base.Decision.Status)
	}
}

func TestDefenseValidationUnifiedOutcomeGapClassifierIsNarrow(t *testing.T) {
	if !defenseValidationUnifiedOutcomeIsGap(defense.DefenseValidationOutcomeMissedV02) || !defenseValidationUnifiedOutcomeIsGap(defense.DefenseValidationOutcomeCaughtLateV02) {
		t.Fatal("verified blind-spot outcomes are not recognized")
	}
	for _, outcome := range []string{
		defense.DefenseValidationOutcomeCaughtInTimeV02,
		defense.DefenseValidationOutcomeFalsePositiveV02,
		defense.DefenseValidationOutcomeCleanV02,
		defense.DefenseValidationOutcomeIncompleteV02,
	} {
		if defenseValidationUnifiedOutcomeIsGap(outcome) {
			t.Fatalf("outcome %q incorrectly classified as attack-path gap", outcome)
		}
	}
	if defenseValidationUnifiedOutcomeIsGap(strings.TrimSpace("caught_in_time")) {
		t.Fatal("caught-in-time outcome incorrectly classified as gap")
	}
}
