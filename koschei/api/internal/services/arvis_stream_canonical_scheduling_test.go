package services

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func setArvisStreamCanonicalReadyEnv(t *testing.T, trustedKeyID string, trustedSeed byte) {
	t.Helper()
	key := unifiedVerdictTestPrivateKey(0x61)
	t.Setenv(unifiedVerdictSigningKeyIDEnv, "arvis-stream-test-trusted-key")
	t.Setenv(unifiedVerdictSigningPrivateKeyEnv, base64.RawURLEncoding.EncodeToString(key))
	trusted := unifiedVerdictTestPrivateKey(trustedSeed).Public().(ed25519.PublicKey)
	raw, err := json.Marshal([]GlobalRadarTrustedVerdictKey{{
		KeyID: trustedKeyID, PublicKeyBase64URL: base64.RawURLEncoding.EncodeToString(trusted),
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(arvisStreamTrustedKeysEnv, string(raw))
	t.Setenv("SOLANA_RPC_URL", "https://rpc.example.invalid")
	t.Setenv(arvisStreamCanonicalJobsEnv, "true")
}

func TestArvisStreamCanonicalJobsReadyFailsClosed(t *testing.T) {
	t.Setenv(arvisStreamCanonicalJobsEnv, "")
	if ready, reason := ArvisStreamCanonicalJobsReady(); ready || !strings.Contains(reason, arvisStreamCanonicalJobsEnv) {
		t.Fatalf("scheduling must be off by default: ready=%v reason=%q", ready, reason)
	}

	setArvisStreamCanonicalReadyEnv(t, "arvis-stream-test-trusted-key", 0x61)
	if ready, reason := ArvisStreamCanonicalJobsReady(); !ready {
		t.Fatalf("fully configured trusted signer must be ready: %q", reason)
	}

	cases := map[string]func(t *testing.T){
		"no rpc":       func(t *testing.T) { t.Setenv("SOLANA_RPC_URL", "") },
		"no registry":  func(t *testing.T) { t.Setenv(arvisStreamTrustedKeysEnv, "") },
		"bad registry": func(t *testing.T) { t.Setenv(arvisStreamTrustedKeysEnv, "not-json") },
		"no signer": func(t *testing.T) {
			t.Setenv(unifiedVerdictSigningKeyIDEnv, "")
			t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")
		},
		"untrusted id":    func(t *testing.T) { setArvisStreamCanonicalReadyEnv(t, "some-other-key", 0x61) },
		"mismatched pkey": func(t *testing.T) { setArvisStreamCanonicalReadyEnv(t, "arvis-stream-test-trusted-key", 0x62) },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			setArvisStreamCanonicalReadyEnv(t, "arvis-stream-test-trusted-key", 0x61)
			breakIt(t)
			if ready, reason := ArvisStreamCanonicalJobsReady(); ready || reason == "" {
				t.Fatalf("missing dependency must withhold scheduling: ready=%v reason=%q", ready, reason)
			}
		})
	}
}

func TestMarkArvisStreamCanonicalOutcomeNeverCompletes(t *testing.T) {
	ctx := context.Background()
	status, err := MarkArvisStreamCanonicalOutcome(ctx, nil, "event", errArvisStreamInsufficientEvidence)
	if status != "insufficient_evidence" || err == nil {
		t.Fatalf("missing database must fail: status=%q err=%v", status, err)
	}
	status, _ = MarkArvisStreamCanonicalOutcome(ctx, nil, "event", errors.New("signer unavailable"))
	if status != "failed" {
		t.Fatalf("operational errors must be retryable failures, got %q", status)
	}
	status, _ = MarkArvisStreamCanonicalOutcome(ctx, nil, "event", nil)
	if status == "completed" {
		t.Fatal("an outcome without a signed final must never be completed")
	}
}

func TestArvisStreamCanonicalSchedulingPostgres17(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	// Registered first so it runs last, after the fixture cleanup below.
	t.Cleanup(func() { db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	run := time.Now().UTC().Format("150405.000000")
	mintA := "ci-sched-mint-a-" + run
	mintB := "ci-sched-mint-b-" + run
	ids := []string{}
	insert := func(target, program, signature string) string {
		var id string
		if err := db.QueryRowContext(ctx, `
			INSERT INTO security_radar_stream_events
			(provider,stream_mode,network,module_id,event_type,target,target_type,signature,program_id,evidence_quality)
			VALUES('ci_fixture','test','solana-mainnet',$1,'test_canonical_scheduling',$2,'token',$3,$4,'fixture_only')
			RETURNING id::text
		`, ModulePumpSybilRadar, target, signature, program).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		return id
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		for _, id := range ids {
			_, _ = db.ExecContext(cleanup, `DELETE FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, id)
			_, _ = db.ExecContext(cleanup, `DELETE FROM security_radar_stream_events WHERE id=$1::uuid`, id)
		}
	})
	firstA := insert(mintA, defaultPumpProgramID, "ci-sig-a1-"+run)
	secondA := insert(mintA, defaultPumpProgramID, "ci-sig-a2-"+run)
	unsupported := insert(mintB, "UnsupportedProgram1111111111111111111111111", "ci-sig-b-"+run)

	claimed, err := ClaimArvisStreamCanonicalEvents(ctx, db, 10)
	if err != nil {
		t.Fatal(err)
	}
	mine := map[string]ArvisStreamCanonicalEvent{}
	for _, event := range claimed {
		if event.Target == mintA || event.Target == mintB {
			mine[event.ID] = event
		}
	}
	if len(mine) != 1 {
		t.Fatalf("expected exactly one claimed event for mint A, got %#v", mine)
	}
	var claimedID string
	for id := range mine {
		claimedID = id
	}
	if claimedID != firstA && claimedID != secondA {
		t.Fatalf("unexpected claimed event %s", claimedID)
	}

	status := func(id string) string {
		var value string
		err := db.QueryRowContext(ctx, `SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, id).Scan(&value)
		if errors.Is(err, sql.ErrNoRows) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	if got := status(claimedID); got != "processing" {
		t.Fatalf("claimed event status=%q want processing", got)
	}
	if got := status(unsupported); got != "insufficient_evidence" {
		t.Fatalf("unsupported source must close as insufficient evidence, got %q", got)
	}

	if got, err := MarkArvisStreamCanonicalOutcome(ctx, db, claimedID, errors.New("signer unavailable")); err != nil || got != "failed" {
		t.Fatalf("operational outcome: status=%q err=%v", got, err)
	}
	if got := status(claimedID); got != "failed" {
		t.Fatalf("operational failure must be retryable, got %q", got)
	}

	// A failed claim is re-claimed after its back-off; then a stale sweep
	// fails it again instead of leaving it in processing forever.
	if _, err := db.ExecContext(ctx, `UPDATE arvis_stream_processing SET updated_at=now()-interval '2 hours' WHERE stream_event_id=$1::uuid`, claimedID); err != nil {
		t.Fatal(err)
	}
	first := mine[claimedID]
	attempt, reclaimed := claimArvisStreamTargetAttempt(ctx, db, first.asTarget())
	if !reclaimed {
		t.Fatal("failed claim within its retry budget must be re-claimable")
	}
	retry := first
	retry.Attempt = attempt
	if first.Attempt != 1 || retry.Attempt != 2 || first.DedupeKey() == retry.DedupeKey() {
		t.Fatalf("a retry must get its own job dedupe key: first=%q retry=%q", first.DedupeKey(), retry.DedupeKey())
	}
	if _, err := db.ExecContext(ctx, `UPDATE arvis_stream_processing SET updated_at=now()-interval '2 hours' WHERE stream_event_id=$1::uuid`, claimedID); err != nil {
		t.Fatal(err)
	}

	// A queued or running investigation keeps its claim however long it takes.
	const jobType = "ci_arvis_stream_canonical_fixture"
	var jobID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO web3_jobs (job_type,status,network,target,request_payload,progress)
		VALUES ($1,'running','solana-mainnet',$2,jsonb_build_object('source',$3::text,'source_event_id',$4::text),0)
		RETURNING id::text
	`, jobType, mintA, ArvisStreamCanonicalSource, claimedID).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 15*time.Second)
		defer done()
		_, _ = db.ExecContext(cleanup, `DELETE FROM web3_jobs WHERE id=$1::uuid`, jobID)
	})
	if _, err := ReleaseStaleArvisStreamCanonicalClaims(ctx, db, jobType, time.Hour, 50); err != nil {
		t.Fatal(err)
	}
	if got := status(claimedID); got != "processing" {
		t.Fatalf("claim with a live investigation must not be released, got %q", got)
	}
	if _, err := db.ExecContext(ctx, `UPDATE web3_jobs SET status='failed' WHERE id=$1::uuid`, jobID); err != nil {
		t.Fatal(err)
	}
	released, err := ReleaseStaleArvisStreamCanonicalClaims(ctx, db, jobType, time.Hour, 50)
	if err != nil || released < 1 {
		t.Fatalf("stale claim was not released: released=%d err=%v", released, err)
	}
	if got := status(claimedID); got != "failed" {
		t.Fatalf("released stale claim status=%q want failed", got)
	}

	if got, err := MarkArvisStreamCanonicalOutcome(ctx, db, claimedID, errArvisStreamInsufficientEvidence); err != nil || got != "insufficient_evidence" {
		t.Fatalf("outcome on a non-processing claim: status=%q err=%v", got, err)
	}
	if got := status(claimedID); got != "failed" {
		t.Fatalf("outcome must only close processing claims, got %q", got)
	}
}

func (e ArvisStreamCanonicalEvent) asTarget() arvisStreamTarget {
	return arvisStreamTarget{ID: e.ID, Target: e.Target, Signature: e.Signature, Network: e.Network}
}
