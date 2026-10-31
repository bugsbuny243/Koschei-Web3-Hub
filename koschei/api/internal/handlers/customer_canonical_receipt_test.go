package handlers

import (
	"testing"

	"koschei/api/internal/services"
)

func TestCustomerCanonicalReceiptGateRequiresExactDurableSignedFinal(t *testing.T) {
	configureHandlerVerdictTestSigner(t)
	final := services.FinalizeUnifiedRadarVerdictContractForNetwork("MintReceipt", "solana-mainnet", services.UnifiedRadarVerdict{
		RulesetVersion: services.UnifiedRadarRulesetVersion,
		ActorRuleset:   services.ActorDefenseRulesetVersion,
		TriggeredRules: []services.ActorDefenseRuleHit{{
			RuleID: services.ActorRuleCompoundCreatorReuse,
			Tier:   "compounding", EvidenceStatus: "verified",
			Summary: "actual verified creator evidence",
		}},
	})
	if !final.Signed || final.Signature == "" || final.PayloadHash == "" {
		t.Fatalf("test signer unavailable: %#v", final)
	}
	record := services.UnifiedRadarVerdictHistoryRecord{
		ID:      "saved-unified-verdict-1", LedgerID: "immutable-ledger-1",
		Network: final.Network, TargetKind: "token", TargetID: final.Target,
		Grade: final.Grade, Verdict: final.Verdict, RulesetVersion: final.RulesetVersion,
		ActorRulesetVersion: final.ActorRuleset, Digest: final.Digest, Signed: final.Signed,
		Signature: final.Signature, SignatureAlgorithm: final.SignatureAlgorithm,
		KeyID: final.KeyID, PayloadHash: final.PayloadHash, Fingerprint: "state:verified-receipt",
	}
	test := func(name string, verdict services.UnifiedRadarVerdict, live bool, persistence string, history []services.UnifiedRadarVerdictHistoryRecord, network, target, want string, accepted bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			status, got, billable := customerCanonicalReceiptGate(verdict, live, persistence, history, network, target)
			if status != want || billable != accepted {
				t.Fatalf("status=%q billable=%v; want %q/%v", status, billable, want, accepted)
			}
			if billable && (got.ID != record.ID || got.Signature != verdict.Signature) {
				t.Fatalf("returned receipt mismatches signed final: %#v", got)
			}
			if !billable && (got.ID != "" || got.Signature != "") {
				t.Fatalf("failed gate leaked a seemingly valid receipt: %#v", got)
			}
		})
	}
	test("verified signed final and database receipt", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{record}, final.Network, final.Target, "ready", true)
	test("signed final with no live evidence", final, false, "persisted", []services.UnifiedRadarVerdictHistoryRecord{record}, final.Network, final.Target, "evidence_pending", false)
	test("database unavailable", final, true, "database_unavailable", nil, final.Network, final.Target, "persistence_pending", false)
	test("compact index is not durable signed receipt", final, true, "compact_index_created", []services.UnifiedRadarVerdictHistoryRecord{record}, final.Network, final.Target, "persistence_pending", false)
	test("history retrieval failed", final, true, "history_read_failed", nil, final.Network, final.Target, "persistence_pending", false)
	test("receipt missing", final, true, "persisted", nil, final.Network, final.Target, "persistence_pending", false)
	test("receipt without ID", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{func() services.UnifiedRadarVerdictHistoryRecord { r := record; r.ID = ""; return r }()}, final.Network, final.Target, "persistence_pending", false)
	test("receipt missing immutable ledger", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{func() services.UnifiedRadarVerdictHistoryRecord { r := record; r.LedgerID = ""; return r }()}, final.Network, final.Target, "persistence_pending", false)
	test("receipt forged signed bit", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{func() services.UnifiedRadarVerdictHistoryRecord { r := record; r.Signed = false; return r }()}, final.Network, final.Target, "persistence_pending", false)
	test("receipt key substitution", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{func() services.UnifiedRadarVerdictHistoryRecord { r := record; r.KeyID = "other"; return r }()}, final.Network, final.Target, "persistence_pending", false)
	test("receipt payload hash mismatch", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{func() services.UnifiedRadarVerdictHistoryRecord {
		r := record
		r.PayloadHash = "sha256:tampered"
		return r
	}()}, final.Network, final.Target, "persistence_pending", false)
	test("receipt signature mismatch", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{func() services.UnifiedRadarVerdictHistoryRecord { r := record; r.Signature = "forged"; return r }()}, final.Network, final.Target, "persistence_pending", false)
	test("wrong request network", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{record}, "solana-devnet", final.Target, "persistence_pending", false)
	test("wrong request target", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{record}, final.Network, "OtherMint", "persistence_pending", false)
	test("wrong receipt target", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{func() services.UnifiedRadarVerdictHistoryRecord { r := record; r.TargetID = "OtherMint"; return r }()}, final.Network, final.Target, "persistence_pending", false)
	changed := final
	changed.TriggeredRules = append([]services.ActorDefenseRuleHit(nil), final.TriggeredRules...)
	changed.TriggeredRules[0].Summary = "tampered after signature"
	test("tampered customer final", changed, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{record}, changed.Network, changed.Target, "evidence_pending", false)
	fake := final
	fake.Signature = "fake"
	test("fake customer signature", fake, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{record}, fake.Network, fake.Target, "evidence_pending", false)
	t.Setenv("KOSCHEI_VERDICT_SIGNING_PRIVATE_KEY", "")
	test("signer configuration absent", final, true, "persisted", []services.UnifiedRadarVerdictHistoryRecord{record}, final.Network, final.Target, "evidence_pending", false)
}
