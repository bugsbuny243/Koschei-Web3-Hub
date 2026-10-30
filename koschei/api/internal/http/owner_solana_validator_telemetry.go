package http

import (
	"context"
	"errors"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/networktarget"
	"koschei/api/internal/services"
)

var errOwnerValidatorTelemetryInvalid = errors.New("invalid validator telemetry observation")

const (
	ownerValidatorSampleLimit = 12
	ownerValidatorSuccessTTL  = 10 * time.Minute
	ownerValidatorFailureTTL  = 30 * time.Second
	ownerValidatorRPCTimeout  = 12 * time.Second
)

type ownerValidatorProbe func(context.Context, string, time.Time) (services.SolanaValidatorTelemetryReport, error)

type ownerValidatorTelemetryRow struct {
	VotePubkey    string   `json:"vote_pubkey"`
	NodePubkey    string   `json:"node_pubkey"`
	Delinquent    bool     `json:"delinquent"`
	StakeSharePct *float64 `json:"stake_share_pct,omitempty"`
	EvidenceID    string   `json:"evidence_id"`
	EvidenceState string   `json:"evidence_state"`
	DecisionState string   `json:"decision_state"`
}

type ownerValidatorTelemetryResponse struct {
	SchemaVersion    string                       `json:"schema_version"`
	Network          string                       `json:"network"`
	Source           string                       `json:"source"`
	ObservedAt       time.Time                    `json:"observed_at"`
	EvidenceStatus   string                       `json:"evidence_status"`
	LiveAvailability string                       `json:"live_availability"`
	AnalysisPerformed bool                        `json:"analysis_performed"`
	CurrentCount     int                          `json:"current_count"`
	DelinquentCount  int                          `json:"delinquent_count"`
	TotalStake       uint64                       `json:"total_activated_stake"`
	Sampled          int                          `json:"sampled"`
	SampleLimit      int                          `json:"sample_limit"`
	Truncated        bool                         `json:"truncated"`
	Persisted        bool                         `json:"persisted"`
	SafetyVerdict    string                       `json:"safety_verdict"`
	FromCache        bool                         `json:"from_cache"`
	ProbeAttempts    uint64                       `json:"probe_attempts"`
	ProbeFailures    uint64                       `json:"probe_failures"`
	Rows             []ownerValidatorTelemetryRow `json:"observations"`
}

// This is a deliberate operator-triggered read path. It does not start a
// background worker, persist evidence, or influence ARVIS customer verdicts.
// The probe remains disabled unless the operator explicitly enables it.
type ownerValidatorTelemetryGate struct {
	mu           sync.Mutex
	enabled      bool
	rpcURL       string
	probe        ownerValidatorProbe
	now          func() time.Time
	nextAllowed  time.Time
	cached       *ownerValidatorTelemetryResponse
	attempts     uint64
	failures     uint64
}

func newOwnerValidatorTelemetryGate(enabled bool, rpcURL string, probe ownerValidatorProbe) *ownerValidatorTelemetryGate {
	if probe == nil {
		probe = services.ProbeSolanaValidatorTelemetry
	}
	return &ownerValidatorTelemetryGate{
		enabled: enabled,
		rpcURL: strings.TrimSpace(rpcURL),
		probe: probe,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func writeOwnerValidatorTelemetryError(w http.ResponseWriter, code string, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": false,
		"code": code,
		"analysis_performed": false,
		"evidence_status": "unknown",
		"safety_verdict": "not_evaluated",
	})
}

func (g *ownerValidatorTelemetryGate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if g == nil || !g.enabled {
		writeOwnerValidatorTelemetryError(w, "validator_probe_disabled", http.StatusServiceUnavailable)
		return
	}
	if g.rpcURL == "" {
		writeOwnerValidatorTelemetryError(w, "validator_rpc_not_configured", http.StatusServiceUnavailable)
		return
	}
	if len(r.URL.Query()) != 0 {
		writeOwnerValidatorTelemetryError(w, "query_not_supported", http.StatusBadRequest)
		return
	}

	now := g.now().UTC()
	g.mu.Lock()
	if g.cached != nil && now.Before(g.nextAllowed) {
		result := *g.cached
		result.FromCache = true
		result.ProbeAttempts = g.attempts
		result.ProbeFailures = g.failures
		g.mu.Unlock()
		g.writeResponse(w, result)
		return
	}
	if now.Before(g.nextAllowed) {
		g.mu.Unlock()
		writeOwnerValidatorTelemetryError(w, "validator_probe_cooldown", http.StatusTooManyRequests)
		return
	}
	// A past success must not be reused after its ten-minute freshness window,
	// including when the next attempt fails.
	g.cached = nil
	// Reserve one request slot before releasing the mutex. Concurrent refreshes
	// never multiply external RPC requests, including on provider failure.
	g.nextAllowed = now.Add(ownerValidatorFailureTTL)
	g.attempts++
	g.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), ownerValidatorRPCTimeout)
	defer cancel()
	report, err := g.probe(ctx, g.rpcURL, now)
	var result ownerValidatorTelemetryResponse
	if err == nil {
		result, err = projectOwnerValidatorTelemetry(report)
	}

	g.mu.Lock()
	if err != nil {
		g.failures++
		g.mu.Unlock()
		// Never echo provider errors: RPC URLs may contain access credentials.
		writeOwnerValidatorTelemetryError(w, "validator_probe_unavailable", http.StatusBadGateway)
		return
	}
	g.cached = &result
	g.nextAllowed = g.now().UTC().Add(ownerValidatorSuccessTTL)
	result.ProbeAttempts = g.attempts
	result.ProbeFailures = g.failures
	g.mu.Unlock()
	g.writeResponse(w, result)
}

func (g *ownerValidatorTelemetryGate) writeResponse(w http.ResponseWriter, result ownerValidatorTelemetryResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true,
		"telemetry": result,
	})
}

func projectOwnerValidatorTelemetry(report services.SolanaValidatorTelemetryReport) (ownerValidatorTelemetryResponse, error) {
	if report.Network != "solana-mainnet" || report.SchemaVersion != networktarget.NetworkTelemetrySchemaVersion ||
		!report.AnalysisPerformed || report.LiveAvailability != "checked" ||
		report.ObservedAt.IsZero() || report.CurrentCount < 0 || report.DelinquentCount < 0 ||
		report.CurrentCount+report.DelinquentCount != len(report.Validators) ||
		len(report.Validators) == 0 || report.Source != "solana-rpc:getVoteAccounts" {
		return ownerValidatorTelemetryResponse{}, errOwnerValidatorTelemetryInvalid
	}
	sampled := report
	if len(sampled.Validators) > ownerValidatorSampleLimit {
		sampled.Validators = sampled.Validators[:ownerValidatorSampleLimit]
	}
	evidence, err := services.ProjectSolanaValidatorTelemetryToGlobalRadar(sampled)
	if err != nil || len(evidence) != len(sampled.Validators) {
		return ownerValidatorTelemetryResponse{}, errOwnerValidatorTelemetryInvalid
	}
	rows := make([]ownerValidatorTelemetryRow, 0, len(evidence))
	for i, projected := range evidence {
		if projected.Evidence.Status != services.IntelligenceEvidenceObserved ||
			projected.DecisionState != "evidence_only_no_verdict_created" ||
			projected.Evidence.ID == "" {
			return ownerValidatorTelemetryResponse{}, errOwnerValidatorTelemetryInvalid
		}
		val := sampled.Validators[i]
		rows = append(rows, ownerValidatorTelemetryRow{
			VotePubkey: val.VotePubkey,
			NodePubkey: val.NodePubkey,
			Delinquent: val.Delinquent,
			StakeSharePct: val.Observation.StakeSharePct,
			EvidenceID: projected.Evidence.ID,
			EvidenceState: projected.Evidence.Status,
			DecisionState: projected.DecisionState,
		})
	}
	return ownerValidatorTelemetryResponse{
		SchemaVersion: "koschei.owner-validator-telemetry.v1",
		Network: report.Network,
		Source: report.Source,
		ObservedAt: report.ObservedAt.UTC(),
		EvidenceStatus: "observed",
		LiveAvailability: report.LiveAvailability,
		AnalysisPerformed: true,
		CurrentCount: report.CurrentCount,
		DelinquentCount: report.DelinquentCount,
		TotalStake: report.TotalActivatedStake,
		Sampled: len(rows),
		SampleLimit: ownerValidatorSampleLimit,
		Truncated: len(rows) < len(report.Validators),
		Persisted: false,
		SafetyVerdict: "not_evaluated",
		Rows: rows,
	}, nil
}
