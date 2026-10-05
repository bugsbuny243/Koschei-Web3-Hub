package sentinelclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCaseFromARVISEnvelopeRequiresSignedLiveEvidence(t *testing.T) {
	base := map[string]any{
		"target":            "MintABC",
		"network":           "solana-mainnet",
		"has_live_evidence": true,
		"final_verdict": map[string]any{
			"grade":     "D",
			"signed":    true,
			"signature": "signature-123456",
			"verdict":   "deterministic ARVIS verdict",
			"triggered_rules": []any{
				map[string]any{"rule_id": "URD-C001"},
			},
		},
		"arms": []any{
			map[string]any{
				"module_id":         "pump_sybil_radar",
				"evidence_verified": true,
				"evidence":          []any{"Verified funding cluster relation."},
			},
		},
	}
	securityCase, ok := CaseFromARVISEnvelope(base)
	if !ok {
		t.Fatal("signed evidence-backed ARVIS result did not produce Sentinel case")
	}
	if securityCase.TargetRef != "MintABC" || securityCase.Network != "solana-mainnet" {
		t.Fatalf("case identity drifted: %#v", securityCase)
	}
	if len(securityCase.Evidence) != 1 || securityCase.Evidence[0].Confidence != "VERIFIED" {
		t.Fatalf("verified evidence was not projected: %#v", securityCase.Evidence)
	}
	if len(securityCase.SignedVerdict.TriggeredRules) != 1 || securityCase.SignedVerdict.TriggeredRules[0] != "URD-C001" {
		t.Fatalf("triggered rule projection drifted: %#v", securityCase.SignedVerdict.TriggeredRules)
	}

	base["has_live_evidence"] = false
	if _, ok := CaseFromARVISEnvelope(base); ok {
		t.Fatal("Sentinel case was created without live evidence")
	}
}

func TestConfigRequiresAuthenticatedObserveBoundary(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if (Config{Enabled: true, BaseURL: server.URL}).Ready() {
		t.Fatal("Sentinel adapter became ready without API token")
	}
	if !(Config{Enabled: true, BaseURL: server.URL, Token: "fabric-secret"}).Ready() {
		t.Fatal("authenticated Sentinel adapter did not become ready")
	}
}

func TestObservePreservesARVISAuthorityAndIdentity(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	securityCase := SecurityCase{
		SchemaVersion: caseSchemaVersion,
		CaseID:        "arvis-case-1",
		TargetRef:     "MintABC",
		Network:       "solana-mainnet",
		SignedVerdict: SignedVerdict{Grade: "D", Signature: "signature-123456", Summary: "ARVIS final", TriggeredRules: []string{"URD-C001"}},
		Evidence: []EvidenceItem{{EvidenceID: "e-1", Kind: "pump_sybil_radar", Statement: "Verified evidence", Confidence: "VERIFIED", RuleIDs: []string{}, Attributes: map[string]any{"source": "arvis"}}},
		Limitations: []string{},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/opinions" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer fabric-secret" {
			t.Fatalf("missing authenticated Fabric request: %q", r.Header.Get("Authorization"))
		}
		var received SecurityCase
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		if received.SignedVerdict.Signature != securityCase.SignedVerdict.Signature {
			t.Fatalf("signature changed in transit: %#v", received.SignedVerdict)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CheckedOpinion{
			Accepted: true,
			Opinion: Opinion{
				SchemaVersion:      "sentinel.opinion.v1",
				CaseID:             securityCase.CaseID,
				VerdictSignature:   securityCase.SignedVerdict.Signature,
				Authority:          "The signed deterministic verdict is final; this output is commentary only.",
				Assessment:         "EXPLANATION_ONLY",
				Claims:             []EvidenceClaim{},
				Limitations:        []string{},
				RecommendedActions: []string{},
				Engine:             "sentinel-baseline-v0.1",
			},
			PolicyViolations: []string{},
		})
	}))
	defer server.Close()

	observation, err := Observe(t.Context(), server.Client(), Config{Enabled: true, BaseURL: server.URL, Token: "fabric-secret"}, securityCase)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Mode != "observe" || observation.Authority != "commentary_only_arvis_verdict_final" || !observation.Accepted {
		t.Fatalf("authority boundary drifted: %#v", observation)
	}
	if !strings.Contains(observation.Opinion.Authority, "deterministic verdict is final") {
		t.Fatalf("Sentinel authority disclaimer missing: %#v", observation.Opinion)
	}
}
