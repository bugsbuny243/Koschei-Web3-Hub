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

Return compact JSON only.
Do not include markdown.
Do not include commentary.
Do not include reasoning text.
Arrays must contain 3 to 5 items maximum where applicable.
If unsure, still return valid JSON with safe defensive placeholders.

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

func buildCyberAnalysis(raw string) (map[string]any, string, error) {
	analysis := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &analysis); err == nil {
		return analysis, "", nil
	}
	first := strings.Index(raw, "{")
	last := strings.LastIndex(raw, "}")
	if first >= 0 && last > first {
		extracted := raw[first : last+1]
		if err := json.Unmarshal([]byte(extracted), &analysis); err == nil {
			return analysis, extracted, nil
		}
	}
	if strings.TrimSpace(raw) != "" {
		return nil, "", errors.New("provider_invalid_response")
	}
	return nil, "", errors.New("empty_ai_response")
}

func sanitizeCyberAnalysis(analysis map[string]any, prompt string) (map[string]any, []string, bool) {
	warnings := []string{}
	blocked := false
	safe := map[string]any{}
	for k, v := range analysis {
		safe[k] = v
	}
	safe["human_review_required"] = true

	dangerTerms := []string{"exploit code", "credential theft", "bypass authentication", "malware", "persistence", "exfiltration", "unauthorized access", "destructive shutdown"}
	sensitiveTerms := []string{"bank", "government", "server room", "camera", "smart glasses", "physical security"}

	resultBlob := strings.ToLower(prompt + " " + stringifyJSON(safe))
	for _, term := range dangerTerms {
		if strings.Contains(resultBlob, term) {
			blocked = true
			warnings = append(warnings, "Defensive-only guardrail blocked unsafe content.")
			break
		}
	}

	blockedActions := normalizeStringSlice(safe["blocked_actions"])
	nextSteps := normalizeStringSlice(safe["next_steps"])
	compliance := normalizeStringSlice(safe["compliance_notes"])

	for _, term := range sensitiveTerms {
		if strings.Contains(resultBlob, term) {
			compliance = appendIfMissing(compliance, "Human approval required for sensitive physical/cyber workflow scenarios.")
			blockedActions = appendIfMissing(blockedActions, "autonomous shutdown not allowed")
			blockedActions = appendIfMissing(blockedActions, "unauthorized access not allowed")
			break
		}
	}
	if len(blockedActions) == 0 {
		blockedActions = []string{}
	}
	if len(nextSteps) == 0 {
		nextSteps = []string{"Review findings with security leadership.", "Define approved remediation owners and timelines.", "Track controls with audit logs and periodic validation."}
	}

	risks, _ := safe["risks"].([]any)
	for i := range risks {
		if r, ok := risks[i].(map[string]any); ok {
			r["human_approval_required"] = true
			risks[i] = r
		}
	}
	safe["risks"] = risks
	safe["blocked_actions"] = blockedActions
	safe["next_steps"] = nextSteps
	safe["compliance_notes"] = compliance

	if blocked {
		safe = map[string]any{
			"executive_summary":     "Request/result blocked by defensive-only guardrails.",
			"scope":                 []string{},
			"risks":                 []string{},
			"compliance_notes":      []string{"Defensive-only policy triggered."},
			"incident_steps":        []string{},
			"human_review_required": true,
			"blocked_actions":       []string{"offensive or unauthorized security content"},
			"next_steps":            []string{"Reframe the request as defensive audit, monitoring, or policy review."},
		}
	}

	return safe, warnings, blocked
}

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

	userPrompt := "Mode: " + mode + "\nReturn the JSON object only.\nScenario:\n" + prompt
	respText, callErr := h.callTogetherWithSystemTimeoutAndMaxTokens(model, cyberSystemPrompt, userPrompt, timeout, maxTokens)
	primaryModel := model
	fallbackModel := ""
	fallbackUsed := false
	tryFallback := false
	callErrText := ""
	if callErr != nil {
		callErrText = strings.ToLower(callErr.Error())
		tryFallback = strings.Contains(callErrText, "timeout") || strings.Contains(callErrText, "deadline") || strings.Contains(callErrText, "empty_ai_response") || strings.Contains(callErrText, "provider_invalid_response") || strings.Contains(callErrText, "invalid character")
	}
	if tryFallback {
		for _, candidate := range []string{firstEnv("TOGETHER_MODEL_COMPLEX"), firstEnv("TOGETHER_MODEL"), firstEnv("TOGETHER_MODEL_REASONING")} {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" || candidate == model {
				continue
			}
			fallbackModel = candidate
			fallbackUsed = true
			respText, callErr = h.callTogetherWithSystemTimeoutAndMaxTokens(candidate, cyberSystemPrompt, userPrompt, timeout, maxTokens)
			model = candidate
			break
		}
	}
	if callErr != nil {
		detail := shortError(callErr.Error())
		errType := "generation_failed"
		detailLower := strings.ToLower(detail)
		switch {
		case strings.Contains(detailLower, "empty_ai_response"):
			errType = "empty_ai_response"
		case strings.Contains(detailLower, "provider_invalid_response"), strings.Contains(detailLower, "invalid character"):
			errType = "provider_invalid_response"
		case strings.Contains(detailLower, "timeout"), strings.Contains(detailLower, "deadline"):
			errType = "generation_timeout"
			detail = "Cyber analysis provider timed out. Credits not charged."
		}
		_ = h.insertCyberAnalysis(claims.Sub, claims.Email, mode, prompt, "failed", model, map[string]any{}, detail, false, map[string]any{"raw_ai_output": respText, "primary_model": primaryModel, "fallback_model": fallbackModel, "fallback_used": fallbackUsed})
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": errType, "detail": detail, "credits_charged": false, "model": model, "fallback_model": fallbackModel, "fallback_used": fallbackUsed})
		return
	}

	analysis, extractedRaw, parseErr := buildCyberAnalysis(respText)
	if parseErr != nil {
		errType := parseErr.Error()
		detail := "Provider returned invalid JSON."
		if errType == "empty_ai_response" {
			detail = "Cyber model returned an empty answer. Credits not charged."
		}
		_ = h.insertCyberAnalysis(claims.Sub, claims.Email, mode, prompt, "failed", model, map[string]any{}, errType, false, map[string]any{"raw_ai_output": respText, "primary_model": primaryModel, "fallback_model": fallbackModel, "fallback_used": fallbackUsed})
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": errType, "detail": detail, "credits_charged": false, "model": model, "fallback_model": fallbackModel, "fallback_used": fallbackUsed})
		return
	}
	if extractedRaw != "" {
		analysis["_json_extracted"] = true
	}
	sanitized, warnings, blocked := sanitizeCyberAnalysis(analysis, prompt)
	if blocked {
		_ = h.insertCyberAnalysis(claims.Sub, claims.Email, mode, prompt, "blocked", model, sanitized, "defensive_guardrail_blocked", false, nil)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "blocked_content", "detail": "Request/result blocked by defensive-only guardrails.", "credits_charged": false})
		return
	}
	if err := h.insertCyberAnalysisAndCharge(claims.Sub, claims.Email, mode, prompt, "completed", model, sanitized, isPrivileged, map[string]any{"primary_model": primaryModel, "fallback_model": fallbackModel, "fallback_used": fallbackUsed}); err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "insufficient") {
			writeJSON(w, http.StatusPaymentRequired, map[string]any{"error": "insufficient_credits", "credits_charged": false})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "db_failed", "credits_charged": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "completed", "analysis": sanitized, "credits_charged": !isPrivileged, "warnings": warnings, "model": model, "fallback_used": fallbackUsed})
}

func (h *Handler) insertCyberAnalysisAndCharge(authSub, email, mode, prompt, status, model string, result map[string]any, isPrivileged bool, metadata map[string]any) error {
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	resultRaw, _ := json.Marshal(result)
	metadataRaw, _ := json.Marshal(metadata)
	creditsCharged := !isPrivileged
	if _, err := tx.Exec(`INSERT INTO cyber_analyses (auth_subject, user_email, mode, prompt, status, model, result, metadata, credits_charged) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, authSub, email, mode, prompt, status, model, resultRaw, metadataRaw, creditsCharged); err != nil {
		return err
	}
	if !isPrivileged {
		if err := h.applyCreditChargeTxWithReason(tx, authSub, email, "cyber_analysis"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (h *Handler) insertCyberAnalysis(authSub, email, mode, prompt, status, model string, result map[string]any, errMsg string, creditsCharged bool, metadata map[string]any) error {
	resultRaw, _ := json.Marshal(result)
	metadataRaw, _ := json.Marshal(metadata)
	_, err := h.DB.Exec(`INSERT INTO cyber_analyses (auth_subject, user_email, mode, prompt, status, model, result, error_message, metadata, credits_charged) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, authSub, email, mode, prompt, status, model, resultRaw, errMsg, metadataRaw, creditsCharged)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "cyber_analyses") {
		return errors.New("db_failed")
	}
	return err
}

func (h *Handler) ListCyberAnalyses(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rows, err := h.DB.Query(`SELECT id, mode, prompt, status, model, result, error_message, metadata, credits_charged, created_at FROM cyber_analyses WHERE auth_subject=$1 ORDER BY created_at DESC LIMIT 20`, claims.Sub)
	hasMetadata := true
	if err != nil {
		rows, err = h.DB.Query(`SELECT id, mode, prompt, status, model, result, error_message, credits_charged, created_at FROM cyber_analyses WHERE auth_subject=$1 ORDER BY created_at DESC LIMIT 20`, claims.Sub)
		hasMetadata = false
	}
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
		var resultRaw, metadataRaw []byte
		if hasMetadata {
			if err := rows.Scan(&id, &mode, &prompt, &status, &model, &resultRaw, &errorMsg, &metadataRaw, &creditsCharged, &createdAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
				return
			}
		} else {
			if err := rows.Scan(&id, &mode, &prompt, &status, &model, &resultRaw, &errorMsg, &creditsCharged, &createdAt); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
				return
			}
		}
		result := map[string]any{}
		if len(resultRaw) > 0 {
			_ = json.Unmarshal(resultRaw, &result)
		}
		metadata := map[string]any{}
		if len(metadataRaw) > 0 {
			_ = json.Unmarshal(metadataRaw, &metadata)
		}
		items = append(items, map[string]any{"id": id, "mode": mode, "prompt": prompt, "status": status, "model": model, "analysis": result, "metadata": metadata, "error_message": errorMsg.String, "credits_charged": creditsCharged, "created_at": createdAt})
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"analyses": items})
}

func normalizeStringSlice(v any) []string {
	if arr, ok := v.([]string); ok {
		return arr
	}
	out := []string{}
	if arr, ok := v.([]any); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func appendIfMissing(in []string, item string) []string {
	for _, v := range in {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(item)) {
			return in
		}
	}
	return append(in, item)
}

func stringifyJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
