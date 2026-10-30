package handlers

import (
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

// Uses only a disposable isolated schema. The customer scan never creates a
// SOC incident; only an authenticated owner action links the exact receipt.
func TestOwnerSOCLinksScopedCustomerSignedVerdictPostgres(t *testing.T) {
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
	schema := "signed_soc_" + uid
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
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", migration))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(raw)); err != nil {
			t.Fatalf("migration %s: %v", migration, err)
		}
	}
	configureHandlerVerdictTestSigner(t)
	target, network := "MintSOCReceipt", "solana-mainnet"
	final := services.FinalizeUnifiedRadarVerdictContractForNetwork(target, network, services.UnifiedRadarVerdict{
		RulesetVersion: services.UnifiedRadarRulesetVersion,
		ActorRuleset: services.ActorDefenseRulesetVersion,
		TriggeredRules: []services.ActorDefenseRuleHit{{
			RuleID: services.ActorRuleCompoundCreatorReuse,
			Tier: "compounding", EvidenceStatus: "verified", Summary: "creator evidence",
		}},
	})
	if !final.Signed {
		t.Fatal("final signature fixture failed")
	}
	var recordID string
	if err := db.QueryRow(`
		INSERT INTO security_unified_radar_verdicts (
			network,target_kind,target_id,grade,verdict,ruleset_version,actor_ruleset_version,
			signed,signature,signature_algorithm,key_id,payload_hash,fingerprint,digest
		) VALUES ($1,'token',$2,$3,$4,$5,$6,true,$7,$8,$9,$10,'signed-soc-fixture-1',$11)
		RETURNING id::text
	`, network,target,final.Grade,final.Verdict,final.RulesetVersion,
		final.ActorRuleset,final.Signature,final.SignatureAlgorithm,final.KeyID,
		final.PayloadHash,final.Digest).Scan(&recordID); err != nil {
		t.Fatal(err)
	}
	ref := unifiedSOCVerdictRefPrefix + recordID + ":" + final.Signature
	var count int
	if err := db.QueryRow("SELECT count(*) FROM security_incident_cases").Scan(&count); err != nil || count != 0 {
		t.Fatalf("signed verdict must not auto-create a SOC incident: count=%d err=%v", count, err)
	}

	h := &Handler{DB: db, DBRead: db}
	postCreate := func(network, target, ref string) (*httptest.ResponseRecorder, string) {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"title": "Manual SOC review", "severity": "medium",
			"network": network, "target": target,
			"summary": "Operator explicitly linked a persisted signed customer verdict",
			"evidence_refs": []string{ref},
		})
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/owner/incidents", strings.NewReader(string(body)))
		h.OwnerIncidents(w, req)
		var parsed struct {
			Incident struct { ID string `json:"id"` } `json:"incident"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &parsed)
		return w, parsed.Incident.ID
	}
	for name, tc := range map[string][3]string{
		"wrong network": {"solana-devnet", target, ref},
		"wrong target": {network, "OtherMint", ref},
		"tampered signature": {network, target, ref[:len(ref)-1] + alternateSignatureSuffix(final.Signature)},
		"bare row ID": {network, target, recordID},
		"malformed ref": {network, target, "unified-verdict:not-a-uuid:invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			w, _ := postCreate(tc[0], tc[1], tc[2])
			if w.Code != http.StatusConflict {
				t.Fatalf("%s accepted invalid SOC evidence ref: %d %s", name, w.Code, w.Body.String())
			}
		})
	}
	w, firstID := postCreate(network, target, ref)
	if w.Code != http.StatusCreated || firstID == "" {
		t.Fatalf("owner incident did not accept exact matching signed verdict: %d %s", w.Code, w.Body.String())
	}
	// The existing SOC queue reads that same incident row, not a synthetic UI
	// projection. The original case evidence reference is preserved verbatim.
	detail := httptest.NewRecorder()
	h.OwnerIncidentItem(detail, httptest.NewRequest(http.MethodGet, "/api/owner/incidents/"+firstID, nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), ref) ||
		!strings.Contains(detail.Body.String(), unifiedSOCVerdictRefSource) {
		t.Fatalf("SOC incident missing exact source-linked reference: %d %s", detail.Code, detail.Body.String())
	}

	other, secondID := postCreate(network, target, "not-a-source")
	if other.Code != http.StatusConflict {
		t.Fatalf("invented SOC evidence ref accepted: %d %s", other.Code, other.Body.String())
	}
	empty, emptyID := postCreate(network, target, "")
	if empty.Code != http.StatusConflict || emptyID != "" {
		t.Fatalf("empty manual receipt accepted: %d %s", empty.Code, empty.Body.String())
	}
	_ = secondID

	// A later row change invalidates the original signature reference. SOC
	// must never silently relink to a mutable fingerprint-only history row.
	if _, err := db.Exec("UPDATE security_unified_radar_verdicts SET signature=$2 WHERE id=$1::uuid", recordID, final.Signature[:len(final.Signature)-1]+alternateSignatureSuffix(final.Signature)); err != nil {
		t.Fatal(err)
	}
	changed, _ := postCreate(network, target, ref)
	if changed.Code != http.StatusConflict {
		t.Fatalf("rotated signed history row silently reused old receipt: %d %s", changed.Code, changed.Body.String())
	}
}

func alternateSignatureSuffix(signature string) string {
	if strings.HasSuffix(signature, "A") {
		return "B"
	}
	return "A"
}
