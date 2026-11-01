package services

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
	"os"

	_ "github.com/lib/pq"
)

func signatureJournalTestWSS() SecurityRadarStreamEventRecord {
	signature := strings.Repeat("3", 87)
	slot := int64(100100)
	return SecurityRadarStreamEventRecord{
		Provider: SecurityRadarStreamProvider, StreamMode: SecurityRadarStreamModeLogs,
		Network: "solana-mainnet", ModuleID: ModuleRaydiumPoolGuardian,
		EventType: "raydium_pool_or_liquidity", Target: signature, TargetType: "pool_or_token",
		Signature: signature, Slot: slot, ProgramID: defaultRaydiumProgramID,
		EvidenceQuality: "decoded_stream_hint",
		Decoded: map[string]any{"logs": []string{"Program log: Instruction: Swap"}},
		RawEvent: map[string]any{
			"method": "logsNotification",
			"params": map[string]any{
				"result": map[string]any{
					"context": map[string]any{"slot": float64(slot)},
					"value": map[string]any{"signature": signature, "err": nil},
				},
			},
		},
	}
}

func signatureJournalTestReplay() SecurityRadarStreamEventRecord {
	signature := strings.Repeat("4", 87)
	return SecurityRadarStreamEventRecord{
		Provider: "solana_rpc_gap_healer", StreamMode: "gap_replay",
		Network: "solana-mainnet", ModuleID: ModuleRaydiumPoolGuardian,
		EventType: "raydium_program_signature", Target: signature, TargetType: "signature",
		Signature: signature, Slot: 100101, ProgramID: defaultRaydiumProgramID,
		EvidenceQuality: "replay_signature_observed",
		Decoded: map[string]any{"rpc_method": "getSignaturesForAddress", "requires_enrichment": true},
		RawEvent: map[string]any{"signature": signature, "slot": int64(100101), "replayed": true},
	}
}

func TestSignatureJournalAcceptsOnlyProvenancedObservedSignatures(t *testing.T) {
	for _, valid := range []SecurityRadarStreamEventRecord{signatureJournalTestWSS(), signatureJournalTestReplay()} {
		if err := validateSecurityRadarSignatureJournal(valid); err != nil {
			t.Fatalf("valid low-trust Solana signature observation rejected: %v", err)
		}
	}
	for _, test := range []struct {
		name string
		mutate func(*SecurityRadarStreamEventRecord)
	}{
		{"wrong_network", func(e *SecurityRadarStreamEventRecord) { e.Network = "solana-devnet" }},
		{"missing_slot", func(e *SecurityRadarStreamEventRecord) { e.Slot = 0 }},
		{"invalid_signature", func(e *SecurityRadarStreamEventRecord) { e.Signature = "not-a-signature" }},
		{"forged_token_target", func(e *SecurityRadarStreamEventRecord) { e.Target = strings.Repeat("5", 44) }},
		{"untrusted_program", func(e *SecurityRadarStreamEventRecord) { e.ProgramID = "raydium" }},
		{"forged_stream_mode", func(e *SecurityRadarStreamEventRecord) { e.StreamMode = "signed_verdict" }},
		{"forged_verified_evidence", func(e *SecurityRadarStreamEventRecord) { e.EvidenceQuality = "transaction_enriched_mint" }},
		{"wrong_notification", func(e *SecurityRadarStreamEventRecord) { e.RawEvent["method"] = "accountNotification" }},
		{"wrong_nested_signature", func(e *SecurityRadarStreamEventRecord) {
			asRadarMap(asRadarMap(asRadarMap(e.RawEvent["params"])["result"])["value"])["signature"] = strings.Repeat("5", 87)
		}},
		{"wrong_nested_slot", func(e *SecurityRadarStreamEventRecord) {
			asRadarMap(asRadarMap(e.RawEvent["params"])["result"])["context"] = map[string]any{"slot": float64(111111)}
		}},
		{"missing_raw_wss", func(e *SecurityRadarStreamEventRecord) { e.RawEvent = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			event := signatureJournalTestWSS()
			test.mutate(&event)
			if err := validateSecurityRadarSignatureJournal(event); err == nil {
				t.Fatalf("unverified receipt input promoted into durable journal: %+v", event)
			}
		})
	}
	for _, test := range []struct {
		name string
		mutate func(*SecurityRadarStreamEventRecord)
	}{
		{"missing_replay_marker", func(e *SecurityRadarStreamEventRecord) { delete(e.RawEvent, "replayed") }},
		{"fake_replay", func(e *SecurityRadarStreamEventRecord) { e.RawEvent["signature"] = "another" }},
		{"wrong_rpc_method", func(e *SecurityRadarStreamEventRecord) { e.Decoded["rpc_method"] = "eth_getLogs" }},
		{"wrong_replay_quality", func(e *SecurityRadarStreamEventRecord) { e.EvidenceQuality = "transaction_enriched_mint" }},
		{"forged_anchor", func(e *SecurityRadarStreamEventRecord) { e.StreamMode = "gap_bootstrap_anchor" }},
		{"missing_raw", func(e *SecurityRadarStreamEventRecord) { e.RawEvent = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			event := signatureJournalTestReplay()
			test.mutate(&event)
			if err := validateSecurityRadarSignatureJournal(event); err == nil {
				t.Fatalf("unverified replay provenance promoted into durable journal: %+v", event)
			}
		})
	}
}

func TestSignatureJournalRequiresDatabaseAndNeverRebrandsAsMint(t *testing.T) {
	_, err := (&SecurityRadarStore{}).InsertSignatureJournalEvent(context.Background(), signatureJournalTestWSS())
	if err == nil {
		t.Fatal("nil DB must not indicate successful signature persistence")
	}
	_, err = (&SecurityRadarStore{}).InsertStreamEvent(context.Background(), signatureJournalTestWSS())
	if err == nil {
		t.Fatal("verified-token-only production storage must keep refusing raw signatures")
	}
}

func TestSignatureJournalRealPostgresAndReplayCursorInputs(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is unavailable")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil { t.Fatal(err) }

	// Isolated per-connection temp table shadows production; test data never
	// enters a public stream, and no existing row is modified or deleted.
	if _, err := db.ExecContext(ctx, "CREATE TEMP TABLE security_radar_stream_events (LIKE public.security_radar_stream_events INCLUDING ALL)"); err != nil {
		t.Fatal(err)
	}
	store := NewSecurityRadarStore(db)
	wss := signatureJournalTestWSS()
	wssID, err := store.InsertSignatureJournalEvent(ctx, wss)
	if err != nil || wssID == "" { t.Fatalf("WSS journal row not durable: id=%q err=%v", wssID, err) }
	repeated, err := store.InsertSignatureJournalEvent(ctx, wss)
	if err != nil || repeated != "" { t.Fatalf("duplicate raw signature was counted twice: %q %v", repeated, err) }
	replay := signatureJournalTestReplay()
	replayID, err := store.InsertSignatureJournalEvent(ctx, replay)
	if err != nil || replayID == "" { t.Fatalf("replay signature not durable: %q %v", replayID, err) }

	var target sql.NullString
	var targetType, quality, mode, digest string
	if err := db.QueryRowContext(ctx, `SELECT target,target_type,evidence_quality,stream_mode,raw_event_sha256
		FROM security_radar_stream_events WHERE id=$1::uuid`, wssID).Scan(&target,&targetType,&quality,&mode,&digest); err != nil {
		t.Fatal(err)
	}
	if target.Valid || targetType != "signature" || quality != "decoded_stream_hint" ||
		mode != "logs_subscribe" || len(digest) != 64 {
		t.Fatalf("raw WSS was rebranded as verified token evidence: target=%v type=%s quality=%s mode=%s hash=%s",
			target, targetType, quality, mode, digest)
	}
	source := arvisHeartbeatSource{Label: "raydium_program", ProgramID: defaultRaydiumProgramID,
		ModuleID: ModuleRaydiumPoolGuardian, EventType: "raydium_program_signature"}
	var count int64
	var skipped int64
	count, skipped, err = (&securityRadarGapHealer{Store: store, Network: "solana-mainnet"}).
		persistReplayEntries(ctx, source, []securityRadarReplaySignature{
			{Signature: strings.Repeat("6", 87), Slot: 100102},
			{Signature: strings.Repeat("7", 87), Slot: 100103},
		})
	if err != nil || count != 2 || skipped != 0 {
		t.Fatalf("gap healer did not commit actual replay receipts: count=%d skipped=%d err=%v", count, skipped, err)
	}
	count, skipped, err = (&securityRadarGapHealer{Store: store, Network: "solana-mainnet"}).
		persistReplayEntries(ctx, source, []securityRadarReplaySignature{
			{Signature: strings.Repeat("6", 87), Slot: 100102},
			{Signature: strings.Repeat("7", 87), Slot: 100103},
		})
	if err != nil || count != 0 || skipped != 0 {
		t.Fatalf("replayed signatures counted twice: count=%d skipped=%d err=%v", count, skipped, err)
	}
	var total int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM security_radar_stream_events").Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 4 {
		t.Fatalf("unexpected number of durable WSS/replay receipts: %d", total)
	}
}
