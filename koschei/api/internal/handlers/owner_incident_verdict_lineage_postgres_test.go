package handlers

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// Synthetic signed *metadata*, deliberately not a cryptographic signature.
// This verifies only exact PostgreSQL identity joins, never Ed25519 validity.
func TestOwnerIncidentVerdictLineagePostgres17(t *testing.T) {
	dsn := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	target := "synthetic-lineage-" + suffix
	rawSignature := sha512.Sum512([]byte("synthetic:"+suffix))
	signature := base64.RawURLEncoding.EncodeToString(rawSignature[:])
	digestBytes := sha256.Sum256([]byte("synthetic:"+suffix))
	digest := hex.EncodeToString(digestBytes[:])
	payload, err := json.Marshal(map[string]string{
		"signature": signature,
		"evidence_ref": signature,
		"target": target,
		"grade": "F",
		"risk_level": "high",
		"rule_version": "test-lineage-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	var alertID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO security_alert_events
			(source,event_type,severity,target,title,message,dedupe_key,payload)
		VALUES ('arvis','arvis.verdict.created','high',$1,
		        'Synthetic test alert','Test-only, no cryptographic provenance',$2,$3::jsonb)
		RETURNING id::text
	`, target, "synthetic-verdict-link:"+suffix, string(payload)).Scan(&alertID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_alert_events WHERE id=$1::uuid", alertID)
	})

	var verdictID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO security_radar_verdicts
			(module_id,target,target_type,network,grade,risk_index,risk_level,verdict,
			 recommendation,evidence,signals,rule_version,evidence_verified,digest,
			 signed,signature,signature_algorithm,key_id,payload_hash,source)
		VALUES ('final_verdict_engine',$1,'token','solana-mainnet','F',80,'high',
		        'synthetic stored verdict','manual review','[]'::jsonb,'{}'::jsonb,
		        'test-lineage-v1',false,$2,true,$3,'ed25519','fixture-only-key',$4,'ci')
		RETURNING id::text
	`, target, "koschei-radar:"+digest, signature, "sha256:"+digest).Scan(&verdictID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM security_radar_verdicts WHERE id=$1::uuid", verdictID)
	})

	h := &Handler{DB: db, DBRead: db}
	fetch := func() ownerIncidentReviewCandidate {
		t.Helper()
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet,
			"/api/owner/incidents?suggestions=1&target="+url.QueryEscape(target), nil)
		h.OwnerIncidents(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET review: %d %s", response.Code, response.Body.String())
		}
		var out struct {
			Suggestions []ownerIncidentReviewCandidate `json:"suggestions"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Suggestions) != 1 {
			t.Fatalf("expected only synthetic alert, got %d: %s", len(out.Suggestions), response.Body.String())
		}
		return out.Suggestions[0]
	}
	matched := fetch()
	if matched.AlertID != alertID || matched.VerdictLineage.Status != "matched_persisted_signed_verdict_metadata_only" ||
		matched.VerdictLineage.VerdictID != verdictID ||
		matched.VerdictLineage.Network != "solana-mainnet" || matched.ChainEvidenceVerified {
		t.Fatalf("exact persisted metadata join was incorrect: %#v", matched)
	}
	// A single changed rule-version must remove both the link and its identity.
	if _, err := db.ExecContext(ctx, `
		UPDATE security_alert_events
		SET payload=jsonb_set(payload,'{rule_version}','"different-rule"')
		WHERE id=$1::uuid
	`, alertID); err != nil {
		t.Fatal(err)
	}
	mismatched := fetch()
	if mismatched.VerdictLineage.Status != "no_exact_persisted_verdict_match" ||
		mismatched.VerdictLineage.VerdictID != "" || mismatched.VerdictLineage.Network != "" ||
		mismatched.ChainEvidenceVerified {
		t.Fatalf("metadata mismatch was silently joined: %#v", mismatched)
	}

	// A correct-looking hash or signature from another alert is not enough.
	if _, err := db.ExecContext(ctx, `
		UPDATE security_alert_events
		SET payload=jsonb_set(payload,'{rule_version}','"test-lineage-v1"')
		WHERE id=$1::uuid
	`, alertID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		UPDATE security_alert_events
		SET payload=jsonb_set(payload,'{evidence_ref}','"mismatched-evidence-ref"')
		WHERE id=$1::uuid
	`, alertID); err != nil {
		t.Fatal(err)
	}
	badRef := fetch()
	if badRef.VerdictLineage.Status != "no_exact_persisted_verdict_match" ||
		badRef.VerdictLineage.VerdictID != "" {
		t.Fatalf("unmatched evidence_ref was promoted: %#v", badRef)
	}

	var incidentCount int
	if err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM security_incident_cases WHERE target=$1", target).Scan(&incidentCount); err != nil {
		t.Fatal(err)
	}
	if incidentCount != 0 {
		t.Fatalf("read-only lookup created %d incident rows", incidentCount)
	}
	if strings.Contains(matched.VerdictLineage.Authority, "independently_verified") {
		t.Fatal("metadata match must not claim independent verification")
	}
}
