package services

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func responseDispatchFixture(t *testing.T) (GlobalCampaign, FabricCampaignEvidenceContract, GlobalCampaignResponseProposal, GlobalCampaignResponseAuthorization, GlobalCampaignResponseDispatchRequest, time.Time) {
	t.Helper()
	campaign, fabric, proposal, now := responseAuthorizationFixture(t)
	authorization, err := NewGlobalCampaignResponseAuthorization(campaign, fabric, proposal, GlobalCampaignResponseAuthorization{
		AuthorizerKind:           GlobalCampaignResponseAuthorizerOperator,
		AuthorizerRef:            "operator:oncall-1",
		AuthorizationEvidenceRef: "approval:ticket-42",
		AuthorizedAt:             now.Add(-time.Minute),
		ExpiresAt:                now.Add(10 * time.Minute),
		ExplicitAuthorization:    true,
	}, now)
	if err != nil {
		t.Fatalf("authorize response: %v", err)
	}
	request := GlobalCampaignResponseDispatchRequest{
		ExternalOperatorRef:   "operator-forwarder:queue-1",
		ApprovedActionRef:     "action:response-42",
		ApprovedPayloadRef:    "payload:response-42",
		ApprovedPayloadSHA256: "9999999999999999999999999999999999999999999999999999999999999999",
	}
	return campaign, fabric, proposal, authorization, request, now
}

func TestGlobalCampaignResponseDispatchEnvelopeIsDeterministicAndNonAuthoritative(t *testing.T) {
	campaign, fabric, proposal, authorization, request, now := responseDispatchFixture(t)
	first, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now)
	if err != nil {
		t.Fatalf("build first dispatch envelope: %v", err)
	}
	second, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now)
	if err != nil {
		t.Fatalf("build second dispatch envelope: %v", err)
	}
	if first.DispatchRef == "" || first.DispatchRef != second.DispatchRef || first.ReplayKeySHA256 != second.ReplayKeySHA256 || first.EnvelopeHashSHA256 != second.EnvelopeHashSHA256 {
		t.Fatalf("dispatch envelope is not deterministic: first=%+v second=%+v", first, second)
	}
	if !first.ExternalOperatorRequired || first.AutonomousExecutionAllowed || first.SubmissionAuthority || first.SigningAuthority || first.CustodyAuthority || first.SideEffectClaim {
		t.Fatalf("dispatch envelope escaped non-authoritative boundary: %+v", first)
	}
	if err := ValidateGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, first, now); err != nil {
		t.Fatalf("validate dispatch envelope: %v", err)
	}
}

func TestGlobalCampaignResponseDispatchEnvelopeChangesReplayIdentityWithPayload(t *testing.T) {
	campaign, fabric, proposal, authorization, request, now := responseDispatchFixture(t)
	first, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now)
	if err != nil {
		t.Fatalf("build first dispatch envelope: %v", err)
	}
	request.ApprovedPayloadSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	second, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now)
	if err != nil {
		t.Fatalf("build second dispatch envelope: %v", err)
	}
	if first.DispatchRef == second.DispatchRef || first.ReplayKeySHA256 == second.ReplayKeySHA256 {
		t.Fatalf("payload change must change replay identity: first=%+v second=%+v", first, second)
	}
}

func TestGlobalCampaignResponseDispatchEnvelopeRejectsExpiredAuthorization(t *testing.T) {
	campaign, fabric, proposal, authorization, request, now := responseDispatchFixture(t)
	_, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now.Add(11*time.Minute))
	if !errors.Is(err, ErrGlobalCampaignResponseDispatchExpired) {
		t.Fatalf("expected expired dispatch authorization, got %v", err)
	}
}

func TestGlobalCampaignResponseDispatchEnvelopeRejectsMissingExternalMaterialReference(t *testing.T) {
	campaign, fabric, proposal, authorization, request, now := responseDispatchFixture(t)
	request.ApprovedPayloadRef = ""
	_, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now)
	if !errors.Is(err, ErrGlobalCampaignResponseDispatchInvalid) {
		t.Fatalf("expected invalid dispatch request, got %v", err)
	}
}

func TestGlobalCampaignResponseDispatchEnvelopeRejectsAuthorityTamper(t *testing.T) {
	campaign, fabric, proposal, authorization, request, now := responseDispatchFixture(t)
	envelope, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now)
	if err != nil {
		t.Fatalf("build dispatch envelope: %v", err)
	}
	envelope.SubmissionAuthority = true
	if err := ValidateGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, envelope, now); !errors.Is(err, ErrGlobalCampaignResponseDispatchInvalid) {
		t.Fatalf("expected authority tamper rejection, got %v", err)
	}
}

func TestGlobalCampaignResponseDispatchEnvelopeRejectsHashTamper(t *testing.T) {
	campaign, fabric, proposal, authorization, request, now := responseDispatchFixture(t)
	envelope, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now)
	if err != nil {
		t.Fatalf("build dispatch envelope: %v", err)
	}
	envelope.ApprovedPayloadSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := ValidateGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, envelope, now); !errors.Is(err, ErrGlobalCampaignResponseDispatchInvalid) {
		t.Fatalf("expected replay/hash tamper rejection, got %v", err)
	}
}

func TestGlobalCampaignResponseDispatchEnvelopeContainsNoRawPayloadOrKeyMaterial(t *testing.T) {
	campaign, fabric, proposal, authorization, request, now := responseDispatchFixture(t)
	envelope, err := BuildGlobalCampaignResponseDispatchEnvelope(campaign, fabric, proposal, authorization, request, now)
	if err != nil {
		t.Fatalf("build dispatch envelope: %v", err)
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal dispatch envelope: %v", err)
	}
	serialized := strings.ToLower(string(payload))
	for _, forbidden := range []string{"private_key", "secret_key", "seed_phrase", "raw_payload", "payload_bytes"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("dispatch envelope contains forbidden raw/custody material marker %q: %s", forbidden, serialized)
		}
	}
}
