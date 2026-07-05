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
	if h.Limiter != nil && !h.Limiter.allow("arvis-preflight:"+clientIP(r), 20, int64(time.Minute)) {
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
		resp.HumanMessage = "Eksik veriyle güvenli karar veremem. İşlemi durdur ve hedefi doğrula."
		return resp
	}
	if strings.HasPrefix(strings.ToLower(target), "http://") {
		resp.Score += 25
		addReason("Bağlantı HTTPS değil; sahte site veya yönlendirme riski yüksek.")
	}
	if strings.Contains(strings.ToLower(target), "xn--") || strings.Contains(strings.ToLower(target), "airdrop") || strings.Contains(strings.ToLower(target), "claim") {
		resp.Score += 20
		addReason("Hedef metinde sahte ödül, claim veya benzer alan adı riski var.")
	}
	if kind == "token" || kind == "mint" || solanaPreflightAddressLike.MatchString(target) {
		addStep("Resmî mint adresini proje sitesi ve resmî sosyal hesaplarla çapraz doğrula.")
		addStep("Authority, arz, holder dağılımı ve likidite bilgisini kontrol et.")
	}
	if strings.Contains(intent, "recovery phrase") || strings.Contains(intent, "secret words") || strings.Contains(intent, "kurtarma kelimeleri") {
		resp.Score = 100
		resp.Decision = "blocked"
		resp.RiskLevel = "critical"
		addReason("Cüzdan kurtarma bilgisi isteniyor; bu doğrudan hesap kaybı riski doğurur.")
		addStep("Bu bilgiyi hiçbir siteye veya kişiye verme.")
		resp.HumanMessage = "Bu işlem durdurulmalı. Cüzdan kurtarma bilgisi isteyen akış güvenli değildir."
		return resp
	}
	if strings.Contains(intent, "approve") || strings.Contains(intent, "sign") || strings.Contains(intent, "imza") || strings.Contains(intent, "connect wallet") || strings.Contains(intent, "cüzdan bağla") {
		resp.Score += 18
		addReason("Cüzdan imzası veya onay isteniyor; işlem öncesi doğrulama gerekli.")
		addStep("İmza ekranında harcama yetkisi veya beklenmeyen token hareketi olup olmadığını kontrol et.")
	}
	if strings.Contains(intent, "guaranteed") || strings.Contains(intent, "garanti") || strings.Contains(intent, "kesin kazanç") || strings.Contains(intent, "100x") {
		resp.Score += 25
		addReason("Garanti getiri veya aşırı kazanç vaadi var; risk artıyor.")
	}
	if resp.Score >= 90 {
		resp.Decision = "blocked"
		resp.RiskLevel = "critical"
	} else if resp.Score >= 70 {
		resp.Decision = "avoid"
		resp.RiskLevel = "high"
	} else if resp.Score >= 40 {
		resp.Decision = "review"
		resp.RiskLevel = "medium"
	} else {
		resp.Decision = "caution"
		resp.RiskLevel = "low"
	}
	if len(resp.Reasons) == 0 {
		addReason("Bilinen kritik kırmızı bayrak yakalanmadı; yine de zincir üstü kanıtlarla doğrulama gerekir.")
	}
	addStep("Resmî kaynakları ve zincir üstü veriyi doğrula.")
	addStep("Şüphe varsa cüzdanı bağlama, imzalama veya fon gönderme.")
	resp.HumanMessage = humanPreflightMessage(resp.Decision)
	return resp
}

func humanPreflightMessage(decision string) string {
	switch decision {
	case "blocked":
		return "Bu akış güvenli görünmüyor. İşlemi durdurmanı öneririm."
	case "avoid":
		return "Yüksek risk var. Resmî doğrulama yapılmadan işlem yapma."
	case "caution":
		return "Net bir kırmızı bayrak yok, ama zincir üstü doğrulama yapılmadan güvenli kabul edilmez."
	default:
		return "Orta risk var. İmza atmadan veya fon göndermeden önce ek kanıtları kontrol et."
	}
}
