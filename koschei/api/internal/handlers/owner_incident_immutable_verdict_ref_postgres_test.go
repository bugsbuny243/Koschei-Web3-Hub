package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"koschei/api/internal/services"

	_ "github.com/lib/pq"
)

// Exercises the real persistence path, external public-key registry and owner
// case API with a disposable PostgreSQL schema (never production data).
func TestOwnerSOCImmutableCanonicalReceiptPostgres(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("isolated PostgreSQL is not configured")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var uid string
	if err := admin.QueryRow("SELECT replace(gen_random_uuid()::text,'-','')").Scan(&uid); err != nil {
		t.Fatal(err)
	}
	schema := "immutable_soc_" + uid
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, migration := range []string{
		"058_unified_radar_verdict_history.sql",
		"120_unified_verdict_ed25519_authentication.sql",
		"123_security_center_incident_workflow.sql",
		"137_soc_operations.sql",
		"138_unified_verdict_immutable_receipts.sql",
		"139_unified_verdict_receipt_recent_proof_index.sql",
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", migration))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("migration %s: %v", migration, err)
		}
	}

	emptyReadiness := probeOwnerSOCSignedReceipts(context.Background(), db)
	if emptyReadiness.State != "not_accepted_no_persisted_signed_receipts" || !emptyReadiness.LedgerTableReadable {
		t.Fatalf("empty immutable ledger claimed acceptance: %+v", emptyReadiness)
	}
	configureHandlerVerdictTestSigner(t)
	key, configured, err := services.ActiveVerdictVerificationKey()
	if err != nil || !configured {
		t.Fatalf("test signer public key: %v", err)
	}
	keys, _ := json.Marshal([]services.GlobalRadarTrustedVerdictKey{{
		KeyID: key.KeyID, PublicKeyBase64URL: key.PublicKeyBase64URL,
	}})
	t.Setenv("KOSCHEI_ARVIS_STREAM_TRUSTED_VERDICT_KEYS_JSON", string(keys))
	final := services.FinalizeUnifiedRadarVerdictContractForNetwork("MintImmutableSOC", "solana-mainnet", services.UnifiedRadarVerdict{
		RulesetVersion: services.UnifiedRadarRulesetVersion,
		ActorRuleset:   services.ActorDefenseRulesetVersion,
		TriggeredRules: []services.ActorDefenseRuleHit{{
			RuleID: services.ActorRuleCompoundCreatorReuse,
			Tier:   "compounding", EvidenceStatus: "verified", Summary: "verified creator evidence",
		}},
	})
	if !final.Signed {
		t.Fatal("signed test final unavailable")
	}
	store := services.NewUnifiedRadarVerdictStore(db)
	record, err := store.Persist(context.Background(), final.Network, "token", final.Target, final, services.UnifiedRadarBehaviorReport{})
	if err != nil || record.LedgerID == "" {
		t.Fatalf("signed final lacked atomic immutable receipt: record=%+v err=%v", record, err)
	}
	history, err := store.History(context.Background(), final.Network, "token", final.Target, 5)
	if err != nil || len(history) != 1 || history[0].LedgerID != record.LedgerID {
		t.Fatalf("immutable receipt not returned by durable history: %+v err=%v", history, err)
	}

	verifiedReadiness := probeOwnerSOCSignedReceipts(context.Background(), db)
	if verifiedReadiness.State != "bounded_historical_signatures_verified_not_end_to_end_accepted" ||
		verifiedReadiness.SampledReceipts != 1 || verifiedReadiness.VerifiedReceipts != 1 ||
		!verifiedReadiness.IndependentTrustLoaded {
		t.Fatalf("authenticated historical receipt was not verified: %+v", verifiedReadiness)
	}
	ref := immutableSOCVerdictRefPrefix + record.LedgerID + ":" + record.Signature
	h := &Handler{DB: db, DBRead: db}
	candidateRead := func() (int, []ownerSOCVerdictCandidate) {
		t.Helper()
		w := httptest.NewRecorder()
		h.OwnerSOCVerdictCandidates(w, httptest.NewRequest(http.MethodGet, "/api/owner/soc/verdict-candidates", nil))
		var result struct {
			Items []ownerSOCVerdictCandidate `json:"items"`
		}
		if w.Code == http.StatusOK {
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("SOC signed candidate response is cacheable")
			}
		}
		return w.Code, result.Items
	}
	if status, items := candidateRead(); status != http.StatusOK || len(items) != 1 ||
		items[0].Linked || items[0].EvidenceRef != ref || items[0].Target != final.Target {
		t.Fatalf("signed unlinked candidate not offered: status=%d items=%+v", status, items)
	}
	postCreate := func(network, target, evidence string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"title": "Immutable signed receipt review", "severity": "medium",
			"network": network, "target": target,
			"summary":       "Owner manually linked cryptographically verified immutable receipt",
			"evidence_refs": []string{evidence},
		})
		w := httptest.NewRecorder()
		h.OwnerIncidents(w, httptest.NewRequest(http.MethodPost, "/api/owner/incidents", strings.NewReader(string(body))))
		return w
	}
	for name, invalid := range map[string][3]string{
		"wrong network":   {"solana-devnet", final.Target, ref},
		"wrong target":    {final.Network, "DifferentMint", ref},
		"wrong signature": {final.Network, final.Target, ref[:len(ref)-1] + alternateSignatureSuffix(record.Signature)},
	} {
		t.Run(name, func(t *testing.T) {
			w := postCreate(invalid[0], invalid[1], invalid[2])
			if w.Code != http.StatusConflict {
				t.Fatalf("invalid immutable proof accepted: %d %s", w.Code, w.Body.String())
			}
		})
	}

	t.Setenv("KOSCHEI_ARVIS_STREAM_TRUSTED_VERDICT_KEYS_JSON", "")
	if status, _ := candidateRead(); status != http.StatusServiceUnavailable {
		t.Fatalf("untrusted candidate list returned HTTP %d", status)
	}
	if probe := probeOwnerSOCSignedReceipts(context.Background(), db); probe.State != "not_accepted_independent_trust_unavailable" ||
		probe.VerifiedReceipts != 0 || probe.IndependentTrustLoaded {
		t.Fatalf("missing independent keys claimed verified signatures: %+v", probe)
	}
	if w := postCreate(final.Network, final.Target, ref); w.Code != http.StatusConflict {
		t.Fatalf("no independent trust registry accepted: %d %s", w.Code, w.Body.String())
	}
	t.Setenv("KOSCHEI_ARVIS_STREAM_TRUSTED_VERDICT_KEYS_JSON", string(keys))
	if w := postCreate(final.Network, final.Target, ref); w.Code != http.StatusCreated ||
		!strings.Contains(w.Body.String(), immutableSOCVerdictRefSource) {
		// The action audit stores the source in its timeline; the creation response
		// only guarantees the created case, not its action payload.
		if w.Code != http.StatusCreated {
			t.Fatalf("verified immutable owner evidence rejected: %d %s", w.Code, w.Body.String())
		}
	}

	if status, items := candidateRead(); status != http.StatusOK || len(items) != 1 || !items[0].Linked {
		t.Fatalf("owner case did not link verified candidate: status=%d items=%+v", status, items)
	}
	// A later mutation of the mutable fingerprint projection must NOT destroy
	// the older signature's immutable decision identity.
	if _, err := db.Exec("UPDATE security_unified_radar_verdicts SET signature=$2 WHERE id=$1::uuid",
		record.ID, record.Signature[:len(record.Signature)-1]+alternateSignatureSuffix(record.Signature)); err != nil {
		t.Fatal(err)
	}

	if w := postCreate(final.Network, final.Target, ref); w.Code != http.StatusCreated {
		t.Fatalf("later rescan invalidated immutable receipt: %d %s", w.Code, w.Body.String())
	}
	if probe := probeOwnerSOCSignedReceipts(context.Background(), db); probe.State != "bounded_historical_signatures_verified_not_end_to_end_accepted" ||
		probe.VerifiedReceipts != 1 {
		t.Fatalf("mutable rescan broke historical signature verification: %+v", probe)
	}
	if _, err := db.Exec("UPDATE security_unified_verdict_receipts SET target_id='tampered' WHERE id=$1::uuid", record.LedgerID); err == nil {
		t.Fatal("append-only ledger permitted UPDATE")
	}
	if _, err := db.Exec("DELETE FROM security_unified_verdict_receipts WHERE id=$1::uuid", record.LedgerID); err == nil {
		t.Fatal("append-only ledger permitted DELETE")
	}

	if _, err := db.Exec("TRUNCATE security_unified_verdict_receipts"); err == nil {
		t.Fatal("append-only ledger permitted TRUNCATE")
	}
	// Direct SQL inserts are not a trust root: even if a database writer injects
	// plausible metadata, only an independently verified signed payload counts.
	if _, err := db.Exec(`
		INSERT INTO security_unified_verdict_receipts
			(verdict_id,network,target_kind,target_id,signature,signature_algorithm,key_id,payload_hash,signed_payload)
		SELECT verdict_id,network,target_kind,target_id,repeat('A',86),signature_algorithm,
		       key_id,payload_hash,signed_payload
		FROM security_unified_verdict_receipts WHERE id=$1::uuid
	`, record.LedgerID); err != nil {
		t.Fatalf("failed to insert deliberately untrusted test-only receipt: %v", err)
	}

	if status, _ := candidateRead(); status != http.StatusServiceUnavailable {
		t.Fatalf("forged database signature was listed as a candidate: status=%d", status)
	}
	if probe := probeOwnerSOCSignedReceipts(context.Background(), db); probe.State != "not_accepted_immutable_receipt_integrity_failure" ||
		probe.VerifiedReceipts == probe.SampledReceipts {
		t.Fatalf("forged database receipt was presented as verified: %+v", probe)
	}
}
