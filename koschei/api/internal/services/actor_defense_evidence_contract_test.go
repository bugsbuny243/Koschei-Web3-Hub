package services

import (
	"encoding/json"
	"testing"
	"time"
)

func TestActorEvidenceLineDirectSOLTransferIsComplete(t *testing.T) {
	item := ActorDefenseEvidenceRecord{
		ActorWallet: "CreatorWallet", CounterpartKind: "wallet", CounterpartID: "HolderWallet",
		Relation: "direct_sol_transfer_out", VerificationStatus: "verified",
		Signature: "signature-one", Slot: 123, ObservedAt: time.Unix(1700000000, 0).UTC(),
		AmountNative: 2.5, Metadata: map[string]any{"actor_role": "creator_deployer"},
	}
	line := BuildActorDefenseEvidenceLine(item)
	if line.SourceWallet != "CreatorWallet" || line.DestinationWallet != "HolderWallet" {
		t.Fatalf("direction=%s -> %s", line.SourceWallet, line.DestinationWallet)
	}
	if line.Program != "system" || !line.EvidenceLineComplete || len(line.EvidenceGaps) != 0 {
		t.Fatalf("line=%#v", line)
	}
}

func TestActorEvidenceLineLiquidityWithoutPoolStaysIncomplete(t *testing.T) {
	item := ActorDefenseEvidenceRecord{
		ActorWallet: "CreatorWallet", CounterpartKind: "transaction", CounterpartID: "signature-two",
		Relation: "liquidity_remove_activity", VerificationStatus: "observed",
		Signature: "signature-two", Slot: 456, ObservedAt: time.Unix(1700000100, 0).UTC(),
		TokenMint: "MintOne", Metadata: map[string]any{"actor_signed": true},
	}
	line := BuildActorDefenseEvidenceLine(item)
	if line.EvidenceLineComplete {
		t.Fatalf("incomplete liquidity evidence became complete: %#v", line)
	}
	if !containsActorEvidenceGap(line.EvidenceGaps, "destination_wallet") || !containsActorEvidenceGap(line.EvidenceGaps, "program") {
		t.Fatalf("gaps=%v", line.EvidenceGaps)
	}
}

func TestActorEvidenceMarshalIncludesCanonicalFields(t *testing.T) {
	item := ActorDefenseEvidenceRecord{
		ActorWallet: "Actor", CounterpartKind: "wallet", CounterpartID: "Other",
		Relation: "direct_token_transfer_in", VerificationStatus: "verified",
		Signature: "signature-three", Slot: 789, ObservedAt: time.Unix(1700000200, 0).UTC(),
		TokenMint: "MintTwo", TokenAmount: 10,
	}
	payload, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"timestamp", "source_wallet", "destination_wallet", "program", "amount", "evidence_line_complete"} {
		if _, exists := decoded[key]; !exists {
			t.Fatalf("missing canonical field %q in %s", key, string(payload))
		}
	}
	if decoded["source_wallet"] != "Other" || decoded["destination_wallet"] != "Actor" || decoded["program"] != "spl-token" {
		t.Fatalf("unexpected canonical line: %#v", decoded)
	}
}

func containsActorEvidenceGap(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
