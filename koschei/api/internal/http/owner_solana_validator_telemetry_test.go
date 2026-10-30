package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

func validatorReportFixture(t *testing.T, at time.Time) services.SolanaValidatorTelemetryReport {
	t.Helper()
	observation, err := networktarget.NormalizeNetworkTelemetry(networktarget.NetworkTelemetryInput{
		NetworkID: "solana-mainnet",
		SubjectKind: "validator",
		SubjectID: "Vote111",
		Source: "solana-rpc:getVoteAccounts",
		ObservedAt: at,
		EvidenceStatus: "observed",
	})
	if err != nil {
		t.Fatal(err)
	}
	return services.SolanaValidatorTelemetryReport{
		SchemaVersion: networktarget.NetworkTelemetrySchemaVersion,
		Network: "solana-mainnet",
		CurrentCount: 1,
		Validators: []services.SolanaVoteAccountTelemetry{{
			VotePubkey: "Vote111", NodePubkey: "Node111",
			ActivatedStake: 100, Commission: 5, LastVote: 123,
			Observation: observation,
		}},
		TotalActivatedStake: 100,
		Source: "solana-rpc:getVoteAccounts",
		ObservedAt: at,
		AnalysisPerformed: true,
		LiveAvailability: "checked",
	}
}

func TestOwnerValidatorTelemetryGateProducesEvidenceOnlyAndCaches(t *testing.T) {
	at := time.Date(2026, 10, 30, 5, 0, 0, 0, time.UTC)
	calls := 0
	probe := func(_ context.Context, endpoint string, observedAt time.Time) (services.SolanaValidatorTelemetryReport, error) {
		calls++
		if endpoint != "https://example.invalid/secret" {
			t.Fatalf("unexpected endpoint %q", endpoint)
		}
		return validatorReportFixture(t, observedAt), nil
	}
	gate := newOwnerValidatorTelemetryGate(true, "https://example.invalid/secret", probe)
	gate.now = func() time.Time { return at }

	request := httptest.NewRequest(http.MethodGet, "/api/owner/radar/solana/validators", nil)
	first := httptest.NewRecorder()
	gate.ServeHTTP(first, request)
	if first.Code != http.StatusOK || first.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("first probe status=%d response=%s", first.Code, first.Body.String())
	}
	var result struct {
		OK        bool                            `json:"ok"`
		Telemetry ownerValidatorTelemetryResponse `json:"telemetry"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Telemetry.EvidenceStatus != "observed" ||
		result.Telemetry.SafetyVerdict != "not_evaluated" || result.Telemetry.Persisted ||
		result.Telemetry.Sampled != 1 || len(result.Telemetry.Rows) != 1 ||
		result.Telemetry.Rows[0].EvidenceState != services.IntelligenceEvidenceObserved ||
		result.Telemetry.Rows[0].DecisionState != "evidence_only_no_verdict_created" {
		t.Fatalf("invalid evidence-only response: %#v", result)
	}
	if strings.Contains(first.Body.String(), "/secret") {
		t.Fatal("provider URL leaked into owner response")
	}
	second := httptest.NewRecorder()
	gate.ServeHTTP(second, request)
	if second.Code != http.StatusOK || calls != 1 || !strings.Contains(second.Body.String(), "\"from_cache\":true") {
		t.Fatalf("cached response invalid, calls=%d status=%d body=%s", calls, second.Code, second.Body.String())
	}
	at = at.Add(ownerValidatorSuccessTTL + time.Second)
	third := httptest.NewRecorder()
	gate.ServeHTTP(third, request)
	if third.Code != http.StatusOK || calls != 2 {
		t.Fatalf("fresh probe after cache expiry failed: calls=%d response=%s", calls, third.Body.String())
	}
}

func TestOwnerValidatorTelemetryRejectsInvalidAndNeverCachesStaleSuccess(t *testing.T) {
	at := time.Date(2026, 10, 30, 5, 0, 0, 0, time.UTC)
	calls := 0
	gate := newOwnerValidatorTelemetryGate(true, "https://rpc.example/secret", func(_ context.Context, _ string, seen time.Time) (services.SolanaValidatorTelemetryReport, error) {
		calls++
		if calls == 1 {
			return validatorReportFixture(t, seen), nil
		}
		return services.SolanaValidatorTelemetryReport{}, errors.New("secret provider URL /secret")
	})
	gate.now = func() time.Time { return at }
	req := httptest.NewRequest(http.MethodGet, "/api/owner/radar/solana/validators", nil)
	gate.ServeHTTP(httptest.NewRecorder(), req)
	at = at.Add(ownerValidatorSuccessTTL + time.Second)
	failure := httptest.NewRecorder()
	gate.ServeHTTP(failure, req)
	if failure.Code != http.StatusBadGateway || strings.Contains(failure.Body.String(), "secret") {
		t.Fatalf("provider failure leaked or had wrong code: %d %s", failure.Code, failure.Body.String())
	}
	cooldown := httptest.NewRecorder()
	gate.ServeHTTP(cooldown, req)
	if cooldown.Code != http.StatusTooManyRequests || calls != 2 || strings.Contains(cooldown.Body.String(), "\"ok\":true") {
		t.Fatalf("expired success was reused or RPC hammered: %d calls=%d response=%s", cooldown.Code, calls, cooldown.Body.String())
	}
}

func TestOwnerValidatorTelemetryStrictResponseValidation(t *testing.T) {
	at := time.Date(2026, 10, 30, 5, 0, 0, 0, time.UTC)
	base := validatorReportFixture(t, at)
	for _, tc := range []struct{
		name string
		mutate func(*services.SolanaValidatorTelemetryReport)
	}{
		{"not_checked",func(r *services.SolanaValidatorTelemetryReport){r.LiveAvailability="not_checked"}},
		{"fabricated_network",func(r *services.SolanaValidatorTelemetryReport){r.Network="ethereum-mainnet"}},
		{"incomplete_count",func(r *services.SolanaValidatorTelemetryReport){r.CurrentCount=2}},
		{"missing_source",func(r *services.SolanaValidatorTelemetryReport){r.Source=""}},
		{"empty_validators",func(r *services.SolanaValidatorTelemetryReport){r.Validators=nil;r.CurrentCount=0}},
	} {
		t.Run(tc.name,func(t *testing.T){
			invalid:=base
			tc.mutate(&invalid)
			if _,err:=projectOwnerValidatorTelemetry(invalid);err==nil{
				t.Fatalf("%s accepted invalid provider report",tc.name)
			}
		})
	}
}

func TestRegisteredOwnerValidatorRoutesAreProtectedAndOptIn(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OWNER_SECRET", "owner-validator-fixture")
	t.Setenv("OWNER_WALLET", "")
	t.Setenv("SOLANA_RPC_URL", "")
	t.Setenv("KOSCHEI_OWNER_SOLANA_VALIDATOR_PROBE_ENABLED", "0")
	srv := NewServer(nil, "", "", "", t.TempDir())
	for _, path := range []string{"/api/owner/radar/solana/validators", "/owner/solana-validators"} {
		anonymous := httptest.NewRecorder()
		srv.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, path, nil))
		if anonymous.Code != http.StatusNotFound {
			t.Fatalf("anonymous %s returned %d, want 404", path, anonymous.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/owner/radar/solana/validators", nil)
	req.Header.Set("x-koschei-secret", "owner-validator-fixture")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "validator_probe_disabled") {
		t.Fatalf("disabled opt-in must fail closed: %d %s", rec.Code, rec.Body.String())
	}
	t.Setenv("KOSCHEI_OWNER_SOLANA_VALIDATOR_PROBE_ENABLED", "1")
	enabled := NewServer(nil, "", "", "", t.TempDir())
	rec = httptest.NewRecorder()
	enabled.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "validator_rpc_not_configured") {
		t.Fatalf("enabled without rpc must fail closed: %d %s", rec.Code, rec.Body.String())
	}
	pageReq := httptest.NewRequest(http.MethodGet, "/owner/solana-validators", nil)
	pageReq.Header.Set("x-koschei-secret", "owner-validator-fixture")
	page := httptest.NewRecorder()
	enabled.ServeHTTP(page, pageReq)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `credentials:"same-origin"`) ||
		!strings.Contains(page.Body.String(), "OBSERVED ≠ VERIFIED") {
		t.Fatalf("private owner page missing evidence boundary: %d %s", page.Code, page.Body.String())
	}
	post := httptest.NewRequest(http.MethodPost, "/api/owner/radar/solana/validators", nil)
	post.Header.Set("x-koschei-secret", "owner-validator-fixture")
	rec = httptest.NewRecorder()
	enabled.ServeHTTP(rec, post)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method accepted: %d", rec.Code)
	}
}
