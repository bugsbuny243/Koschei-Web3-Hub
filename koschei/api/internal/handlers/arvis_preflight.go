package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"koschei/api/internal/router"
)

type arvisPreflightRequest struct {
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Intent string `json:"intent"`
	Note   string `json:"note"`
}

type arvisPreflightResponse struct {
	OK             bool     `json:"ok"`
	Decision       string   `json:"decision"`
	RiskLevel      string   `json:"risk_level"`
	Score          int      `json:"score"`
	Reasons        []string `json:"reasons"`
	NextSteps      []string `json:"next_steps"`
	HumanMessage   string   `json:"human_message"`
	AIProvider     string   `json:"ai_provider,omitempty"`
	AIModel        string   `json:"ai_model,omitempty"`
	AIExplanation  string   `json:"ai_explanation,omitempty"`
	CreditsCharged bool     `json:"credits_charged"`
}

var solanaPreflightAddressLike = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)

const arvisPreflightSystemPrompt = `You are ARVIS, Koschei's defensive Web3 security analyst.
Help users avoid fraud before they buy, sign, connect a wallet, or trust a token.
Be evidence-based, concise and conservative.
Never promise profit. Never give investment advice. Never help with abuse, harassment, evasion or retaliation.
Return a short Turkish risk explanation with concrete reasons and safe next steps.`

func (h *Handler) ARVISPreflight(w http.ResponseWriter, r *http.Request) {
	if h.Limiter != nil && !h.Limiter.allow("arvis-preflight:"+clientIP(r), 20, time.Minute) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}
	var req arvisPreflightRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	resp := evaluateARVISPreflight(req)
	if aiProviderConfigured() && resp.RiskLevel != "low" {
		prompt := "Target: " + strings.TrimSpace(req.Target) + "\nKind: " + strings.TrimSpace(req.Kind) + "\nIntent: " + strings.TrimSpace(req.Intent) + "\nNote: " + strings.TrimSpace(req.Note) + "\nLocal decision: " + resp.Decision + "\nLocal reasons: " + strings.Join(resp.Reasons, "; ")
		ai, err := router.Chat(r.Context(), router.ChatRequest{System: arvisPreflightSystemPrompt, Prompt: prompt, MaxTokens: 450, Temperature: 0.1, Timeout: 18 * time.Second})
		if err == nil {
			resp.AIProvider = ai.Provider
			resp.AIModel = ai.Model
			resp.AIExplanation = strings.TrimSpace(ai.Content)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func evaluateARVISPreflight(req arvisPreflightRequest) arvisPreflightResponse {
	target := strings.TrimSpace(req.Target)
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	intent := strings.ToLower(strings.TrimSpace(req.Intent + " " + req.Note))
	resp := arvisPreflightResponse{OK: true, Decision: "review", RiskLevel: "medium", Score: 45, CreditsCharged: false}
	addReason := func(reason string) {
		for _, existing := range resp.Reasons {
			if existing == reason {
				return
			}
		}
		resp.Reasons = append(resp.Reasons, reason)
	}
	addStep := func(step string) {
		for _, existing := range resp.NextSteps {
			if existing == step {
				return
			}
		}
		resp.NextSteps = append(resp.NextSteps, step)
	}
	if target == "" {
		resp.Decision = "blocked"
		resp.RiskLevel = "high"
		resp.Score = 90
		addReason("Kontrol edilecek adres, site, token veya işlem verisi eksik.")
		addStep("İşlem yapmadan önce hedefi doğrula.")