package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type shieldPreflightRequest struct {
	Target           string         `json:"target"`
	TargetMint       string         `json:"target_mint"`
	Address          string         `json:"address"`
	Network          string         `json:"network"`
	Wallet           string         `json:"wallet"`
	Transaction      string         `json:"transaction"`
	Encoding         string         `json:"encoding"`
	ExpectedPrograms []string       `json:"expected_programs"`
	Context          map[string]any `json:"context"`
}

func (h *Handler) ShieldPreflight(w http.ResponseWriter, r *http.Request) {
	var input shieldPreflightRequest
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "code": "invalid_request", "message": "Invalid request body"})
		return
	}
	if strings.TrimSpace(input.Transaction) != "" {
		h.transactionFirewallSimulate(w, r, input)
		return
	}
	target := strings.TrimSpace(firstNonEmptyString(input.TargetMint, input.Target, input.Address))
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "code": "target_required", "message": "target, target_mint, address or transaction is required"})
		return
	}
	network := strings.TrimSpace(input.Network)
	if network == "" {
		network = "solana-mainnet"
	}
	started := time.Now()
	bundle := services.AnalyzeSecurityRadars(services.SecurityRadarRequest{Target: target, Network: network, Mode: "don2n_preflight"})
	final := services.FinalSecurityRadarVerdict(bundle)
	action := shieldAction(final.RiskLevel, final.RiskIndex)
	reason := shieldReason(bundle, final)
	requestID := shieldRequestID(target, network, started)
	_ = h.saveSecurityRadarBundle(r.Context(), "api_preflight", "don2n_preflight", bundle)
	latencyMS := time.Since(started).Milliseconds()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"request_id":      requestID,
		"product":         "Koschei Shield",
		"mode":            "don2n_preflight",
		"target":          target,
		"network":         network,
		"wallet":          strings.TrimSpace(input.Wallet),
		"action":          action,
		"grade":           final.Grade,
		"risk_index":      final.RiskIndex,
		"risk_level":      final.RiskLevel,
		"verdict":         final.Verdict,
		"recommendation":  final.Recommendation,
		"reason":          reason,
		"signed":          final.Signed,
		"signature":       final.Signature,
		"latency_ms":      latencyMS,
		"evidence_quality": map[string]any{
			"pump_fun_sybil_radar":  bundle.PumpSybilRadar.Signals["data_quality"],
			"raydium_pool_guardian": bundle.RaydiumPoolGuardian.Signals["data_quality"],
			"score_source":          bundle.Metadata["score_source"],
			"deterministic_scoring": bundle.Metadata["deterministic_scoring"],
			"ai_final_scoring":      bundle.Metadata["ai_final_scoring"],
		},
		"modules": []any{bundle.PumpSybilRadar, bundle.RaydiumPoolGuardian},
	})
}

func shieldAction(level string, risk int) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical":
		return "block"
	case "high":
		return "warn"
	case "medium":
		if risk >= 50 {
			return "warn"
		}
		return "allow_with_monitoring"
	default:
		return "allow"
	}
}

func shieldReason(bundle services.SecurityRadarBundle, final services.SecurityRadarFinalVerdict) string {
	parts := []string{}
	if final.Verdict != "" {
		parts = append(parts, final.Verdict)
	}
	if v, ok := bundle.PumpSybilRadar.Signals["real_onchain_evidence"].(bool); ok && !v {
		parts = append(parts, "Pump.fun radar has insufficient live on-chain evidence")
	}
	if v, ok := bundle.RaydiumPoolGuardian.Signals["real_onchain_evidence"].(bool); ok && !v {
		parts = append(parts, "Raydium guardian has insufficient live on-chain evidence")
	}
	if len(parts) == 0 {
		return "Koschei Shield preflight completed"
	}
	return strings.Join(parts, " · ")
}

func shieldRequestID(target, network string, ts time.Time) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(target) + "|" + strings.TrimSpace(network) + "|" + ts.UTC().Format(time.RFC3339Nano)))
	return "shield_" + hex.EncodeToString(h[:])[:24]
}
