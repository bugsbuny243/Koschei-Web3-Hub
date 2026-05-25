package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type cyberAnalyzeRequest struct {
	Mode   string `json:"mode"`
	Prompt string `json:"prompt"`
}

var allowedCyberModes = map[string]bool{
	"security_audit":       true,
	"risk_assessment":      true,
	"incident_response":    true,
	"compliance_checklist": true,
	"asset_review":         true,
	"policy_review":        true,
}

const cyberSystemPrompt = `You are Koschei Cyber Defense Analyst.
You only provide defensive cybersecurity analysis.
You do not provide exploit code, unauthorized access steps, credential theft, malware, persistence, evasion, bypass instructions, or destructive actions.
You may help with:
- risk assessment
- security audits
- compliance checklists
- incident response planning
- defensive monitoring
- asset review
- policy review
- human-approved remediation planning

For bank, government, server room, camera, smart glasses, physical/cyber workflows:
- require human approval
- do not propose autonomous shutdown
- do not propose unauthorized access
- do not perform live scanning
- recommend audit logs, access control, monitoring, reporting, and approval workflows

Return ONLY valid JSON.
No markdown fences.

Required JSON:
{
  "executive_summary": "string",
  "scope": ["string"],
  "risks": [
    {
      "title": "string",
      "severity": "low|medium|high|critical",
      "description": "string",
      "defensive_recommendation": "string",
      "human_approval_required": true
    }
  ],
  "compliance_notes": ["string"],
  "incident_steps": ["string"],
  "human_review_required": true,
  "blocked_actions": ["string"],
  "next_steps": ["string"]
}`

func (h *Handler) CyberAnalyze(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req cyberAnalyzeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_body", "credits_charged": false})
		return
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	prompt := strings.TrimSpace(req.Prompt)
	if !allowedCyberModes[mode] || prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_mode_or_prompt", "credits_charged": false})
		return
	}
	if !togetherAIEnabled() || strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "ai_provider_not_configured", "credits_charged": false})
		return
	}

	isPrivileged, credits, err := h.userCreditsAndRole(claims.Sub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "db_failed", "credits_charged": false})
		return
	}
	if !isPrivileged && credits <= 0 {
		writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": "insufficient_credits", "credits_charged": false})
		return
	}

	model := firstEnv("TOGETHER_MODEL_SECURITY", "TOGETHER_MODEL_REASONING", "TOGETHER_MODEL_COMPLEX", "TOGETHER_MODEL")
	if model == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "security_model_not_configured", "credits_charged": false})
		return
	}
	timeout := 120 * time.Second
	if v := strings.TrimSpace(os.Getenv("TOGETHER_SECURITY_TIMEOUT_SECONDS")); v != "" {
		if parsed, parseErr := time.ParseDuration(v + "s"); parseErr == nil && parsed >= 5*time.Second {
			timeout = parsed
		}
	}
	maxTokens := 2500
	if v := strings.TrimSpace(os.Getenv("TOGETHER_SECURITY_MAX_TOKENS")); v != "" {
		if parsed, parseErr := strconv.Atoi(v); parseErr == nil && parsed > 100 {
			maxTokens = parsed
		}
	}

	userPrompt := "Mode: " + mode + "\n\nSecurity Scenario / Audit Prompt:\n" + prompt
	respText, callErr := h.callTogetherWithSystemTimeoutAndMaxTokens(model, cyberSystemPrompt, userPrompt, timeout, maxTokens)
	if callErr != nil {
		_ = h.insertCyberAnalysis(claims.Sub, claims.Email, mode, prompt, "failed", model, map[string]any{}, shortError(callErr.Error()), false)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "generation_failed", "detail": shortError(callErr.Error()), "credits_charged": false})
		return
	}

	var analysis map[string]any
	if err := json.Unmarshal([]byte(respText), &analysis); err != nil {
		_ = h.insertCyberAnalysis(claims.Sub, claims.Email, mode, prompt, "failed", model, map[string]any{}, "provider_non_json_response", false)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "provider_invalid_response", "credits_charged": false})
		return
	}
	analysis["human_review_required"] = true

	creditsCharged := false
	if err := h.insertCyberAnalysisAndCharge(claims.Sub, claims.Email, mode, prompt, "completed", model, analysis, isPrivileged); err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "insufficient") {
			writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": "insufficient_credits", "credits_charged": false})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "db_failed", "credits_charged": false})
		return
	}
	if !isPrivileged {
		creditsCharged = true
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "completed", "analysis": analysis, "credits_charged": creditsCharged})
}

func (h *Handler) insertCyberAnalysisAndCharge(authSub, email, mode, prompt, status, model string, result map[string]any, isPrivileged bool) error {
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	resultRaw, _ := json.Marshal(result)
	creditsCharged := !isPrivileged
	if _, err := tx.Exec(`INSERT INTO cyber_analyses (auth_subject, user_email, mode, prompt, status, model, result, credits_charged) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, authSub, email, mode, prompt, status, model, resultRaw, creditsCharged); err != nil {
		return err
	}
	if !isPrivileged {
		if err := h.applyCreditChargeTxWithReason(tx, authSub, email, "cyber_analysis"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (h *Handler) insertCyberAnalysis(authSub, email, mode, prompt, status, model string, result map[string]any, errMsg string, creditsCharged bool) error {
	resultRaw, _ := json.Marshal(result)
	_, err := h.DB.Exec(`INSERT INTO cyber_analyses (auth_subject, user_email, mode, prompt, status, model, result, error_message, credits_charged) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, authSub, email, mode, prompt, status, model, resultRaw, errMsg, creditsCharged)
	return err
}

func (h *Handler) ListCyberAnalyses(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rows, err := h.DB.Query(`SELECT id, mode, prompt, status, model, result, error_message, credits_charged, created_at FROM cyber_analyses WHERE auth_subject=$1 ORDER BY created_at DESC LIMIT 20`, claims.Sub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id, mode, prompt, status, model string
		var errorMsg sql.NullString
		var creditsCharged bool
		var createdAt time.Time
		var resultRaw []byte
		if err := rows.Scan(&id, &mode, &prompt, &status, &model, &resultRaw, &errorMsg, &creditsCharged, &createdAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
			return
		}
		result := map[string]any{}
		_ = json.Unmarshal(resultRaw, &result)
		items = append(items, map[string]any{
			"id":              id,
			"mode":            mode,
			"prompt":          prompt,
			"status":          status,
			"model":           model,
			"analysis":        result,
			"error_message":   errorMsg.String,
			"credits_charged": creditsCharged,
			"created_at":      createdAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"analyses": items})
}
