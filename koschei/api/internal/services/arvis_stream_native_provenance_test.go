package services

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func arvisStreamNativeProofFixture(t *testing.T) (
	arvisStreamTarget, SolanaTransactionResult,
	ActorDefenseRuleVerdict, UnifiedRadarBehaviorReport, GlobalRadarTrustedVerdictRegistry,
) {
	t.Helper()
	event, actor, behavior, registry := arvisStreamCanonicalEvidenceProducerFixture(t)
	// Test-only Solana-format identifier; there is no corresponding live chain
	// transaction. Only the pure native-response verifier sees this fixture.
	event.Signature = strings.Repeat("2", 88)
	event.ProgramID = defaultPumpProgramID
	event.Slot = 3489021
	actor.TriggeredRules = append([]ActorDefenseRuleHit(nil), actor.TriggeredRules...)
	actor.TriggeredRules[0].Signatures = []string{event.Signature}

	native := SolanaTransactionResult{
		"slot": float64(event.Slot),
		"transaction": map[string]any{
			"signatures": []any{event.Signature},
			"message": map[string]any{
				"accountKeys": []any{map[string]any{"pubkey": event.Target, "signer": false}},
				"instructions": []any{map[string]any{
					"programId": event.ProgramID,
					"parsed":    map[string]any{"type": "mintTo", "info": map[string]any{"mint": event.Target}},
				}},
			},
		},
		"meta": map[string]any{
			"err":               nil,
			"preTokenBalances":  []any{map[string]any{"mint": event.Target}},
			"postTokenBalances": []any{map[string]any{"mint": event.Target}},
		},
	}
	return event, native, actor, behavior, registry
}

func TestArvisStreamNativeTransactionGateRequiresExactSourceProof(t *testing.T) {
	event, native, _, _, _ := arvisStreamNativeProofFixture(t)
	if err := validateArvisStreamNativeTransaction(event, native); err != nil {
		t.Fatalf("source-bound native transaction refused: %v", err)
	}

	clone := func() SolanaTransactionResult {
		raw := native["transaction"].(map[string]any)
		message := raw["message"].(map[string]any)
		meta := native["meta"].(map[string]any)
		altered := make(SolanaTransactionResult)
		for k, v := range native {
			altered[k] = v
		}
		newRaw := make(map[string]any)
		for k, v := range raw {
			newRaw[k] = v
		}
		newMessage := make(map[string]any)
		for k, v := range message {
			newMessage[k] = v
		}
		newMeta := make(map[string]any)
		for k, v := range meta {
			newMeta[k] = v
		}
		newRaw["message"] = newMessage
		altered["transaction"] = newRaw
		altered["meta"] = newMeta
		return altered
	}

	cases := []struct {
		name   string
		mutate func(*arvisStreamTarget, SolanaTransactionResult)
	}{
		{"cross_target_mint", func(e *arvisStreamTarget, tx SolanaTransactionResult) { e.Target = defaultRaydiumProgramID }},
		{"wrong_stream_signature", func(e *arvisStreamTarget, tx SolanaTransactionResult) { e.Signature = strings.Repeat("3", 88) }},
		{"wrong_slot", func(e *arvisStreamTarget, tx SolanaTransactionResult) { e.Slot++ }},
		{"unsupported_network", func(e *arvisStreamTarget, tx SolanaTransactionResult) { e.Network = "solana-devnet" }},
		{"unsupported_source_program", func(e *arvisStreamTarget, tx SolanaTransactionResult) {
			e.ProgramID = "11111111111111111111111111111111"
		}},
		{"invalid_signature_format", func(e *arvisStreamTarget, tx SolanaTransactionResult) { e.Signature = "not-a-solana-signature" }},
		{"missing_positive_slot", func(e *arvisStreamTarget, tx SolanaTransactionResult) { tx["slot"] = float64(0) }},
		{"no_meta", func(e *arvisStreamTarget, tx SolanaTransactionResult) { delete(tx, "meta") }},
		{"absent_execution_status", func(e *arvisStreamTarget, tx SolanaTransactionResult) { delete(tx["meta"].(map[string]any), "err") }},
		{"failed_transaction", func(e *arvisStreamTarget, tx SolanaTransactionResult) {
			tx["meta"].(map[string]any)["err"] = map[string]any{"InstructionError": []any{0, "Custom"}}
		}},
		{"wrong_response_signature", func(e *arvisStreamTarget, tx SolanaTransactionResult) {
			tx["transaction"].(map[string]any)["signatures"] = []any{strings.Repeat("4", 88)}
		}},
		{"missing_signatures", func(e *arvisStreamTarget, tx SolanaTransactionResult) {
			delete(tx["transaction"].(map[string]any), "signatures")
		}},
		{"no_program_invocation", func(e *arvisStreamTarget, tx SolanaTransactionResult) {
			tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"] = []any{map[string]any{"programId": defaultRaydiumProgramID}}
		}},
		{"missing_mint", func(e *arvisStreamTarget, tx SolanaTransactionResult) {
			tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"] = []any{map[string]any{"programId": e.ProgramID}}
			tx["meta"].(map[string]any)["preTokenBalances"] = nil
			tx["meta"].(map[string]any)["postTokenBalances"] = nil
		}},
		{"mint_only_in_account_keys", func(e *arvisStreamTarget, tx SolanaTransactionResult) {
			tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"] = []any{map[string]any{"programId": e.ProgramID}}
			tx["meta"].(map[string]any)["preTokenBalances"] = []any{}
			tx["meta"].(map[string]any)["postTokenBalances"] = []any{}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := event
			tx := clone()
			tc.mutate(&source, tx)
			if err := validateArvisStreamNativeTransaction(source, tx); !errors.Is(err, errArvisStreamCanonicalFinalUnavailable) {
				t.Fatalf("untrusted source proof accepted: %v", err)
			}
		})
	}
	t.Run("raydium_inner_program_and_token_balances", func(t *testing.T) {
		source := event
		source.ModuleID = ModuleRaydiumPoolGuardian
		source.ProgramID = defaultRaydiumProgramID
		tx := clone()
		tx["transaction"].(map[string]any)["message"].(map[string]any)["instructions"] = []any{}
		tx["meta"].(map[string]any)["innerInstructions"] = []any{map[string]any{
			"instructions": []any{map[string]any{"programId": defaultRaydiumProgramID}},
		}}
		if err := validateArvisStreamNativeTransaction(source, tx); err != nil {
			t.Fatalf("native Raydium inner-instruction proof rejected: %v", err)
		}
	})
}

func TestArvisStreamNativeEvidenceGatePostgres17AtomicAcceptance(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() // fake proof is never committed as a source record

	event, native, actor, behavior, registry := arvisStreamNativeProofFixture(t)
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO security_radar_stream_events
		(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality,program_id,slot)
		VALUES('ci_fixture','test',$1,$2,'test_native_source_proof',$3,'token',$4,'fixture_only',$5,$6)
		RETURNING id::text
	`, event.Network, event.ModuleID, event.Target, event.Signature, event.ProgramID, event.Slot).Scan(&event.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO arvis_stream_processing(stream_event_id,target,signature,status,attempts)
		VALUES($1::uuid,$2,$3,'processing',1)
	`, event.ID, event.Target, event.Signature); err != nil {
		t.Fatal(err)
	}

	mismatched := make(SolanaTransactionResult)
	for k, v := range native {
		mismatched[k] = v
	}
	mismatched["slot"] = float64(event.Slot + 1)
	if _, err := persistArvisStreamNativeEvidenceFinalTx(ctx, tx, event, mismatched, actor, behavior, registry); !errors.Is(err, errArvisStreamCanonicalFinalUnavailable) {
		t.Fatalf("wrong native source wrote a final: %v", err)
	}
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, event.ID).Scan(&status); err != nil || status != "processing" {
		t.Fatalf("invalid proof changed claim: %q %v", status, err)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM arvis_stream_canonical_finals WHERE stream_event_id=$1::uuid`, event.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("bad proof minted receipt: count=%d err=%v", count, err)
	}

	final, err := persistArvisStreamNativeEvidenceFinalTx(ctx, tx, event, native, actor, behavior, registry)
	if err != nil {
		t.Fatalf("native source and independently evaluated canonical decision rejected: %v", err)
	}
	if final.Grade != "B" || !final.Signed {
		t.Fatalf("untrusted or wrong canonical grade=%q signed=%v", final.Grade, final.Signed)
	}
	if err := tx.QueryRowContext(ctx, `SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, event.ID).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("signed atomic completion failed: %q %v", status, err)
	}

	again, err := persistArvisStreamNativeEvidenceFinalTx(ctx, tx, event, native, actor, behavior, registry)
	if err != nil || again.Signature != final.Signature || again.PayloadHash != final.PayloadHash {
		t.Fatalf("replayed exact native proof not idempotent: err=%v", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM arvis_stream_canonical_finals WHERE stream_event_id=$1::uuid`, event.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate native final: count=%d err=%v", count, err)
	}

	other := event
	other.Target = defaultRaydiumProgramID
	if _, err := persistArvisStreamNativeEvidenceFinalTx(ctx, tx, other, native, actor, behavior, registry); err == nil {
		t.Fatal("cross-target native evidence substitution accepted")
	}
}
