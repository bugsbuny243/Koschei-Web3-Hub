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

const officialKOSCHMint = "HHPpU9u56Bwxov12nf7DXUCuv6h1q5j1xgGS3yukpump"

const arvisPreflightSystemPrompt = `You are ARVIS, Koschei's defensive Web3 security analyst.
Help users avoid fraud before they buy, sign, connect a wallet, or trust a token.
Be evidence-based, concise and conservative.
Never promise profit. Never give investment advice.
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
	if aiProviderConfigured() && resp.RiskLevel != "low" && !isOfficialKOSCHMint(req.Target) {
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
	intent := strings.ToLower(strings.TrimSpace(req.Intent + " " + req.Note + " " + target))
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
		resp.HumanMessage = "Hedef bilgi eksik olduğu için güvenli karar verilemiyor."
		return resp
	}
	if isOfficialKOSCHMint(target) {
		resp.Decision = "review"
		resp.RiskLevel = "medium"
		resp.Score = 40
		addReason("Bu adres resmi KOSCH mint adresiyle eşleşiyor.")
		addReason("KOSCH, Koschei ARVIS içinde erişim ve ödeme bildirimi utility katmanı olarak konumlandırılır.")
		addStep("İşlem yapmadan önce mint adresini resmi /kosch sayfasındaki adresle tekrar karşılaştır.")
		addStep("KOSCH erişimi için yalnızca resmi /pricing ödeme bildirim akışını kullan.")
		resp.HumanMessage = "ARVIS ön kontrol sonucu: resmi KOSCH mint eşleşti. Bu finansal tavsiye değildir; işlem yapmadan önce zincir üstü bilgileri ve resmi sayfayı doğrula."
		return resp
	}
	if solanaPreflightAddressLike.MatchString(target) {
		addReason("Hedef Solana adres formatına benziyor; zincir üstü kanıtlarla doğrulanmalı.")
		addStep("Token, wallet veya program adresini ARVIS dashboard içinde detaylı tara.")
	} else {
		addReason("Hedef adres formatı doğrulanamadı; sahte site veya hatalı adres riski kontrol edilmeli.")
		addStep("Resmi kaynaklardan adres/site doğrulaması yap.")
		resp.Score = maxInt(resp.Score, 55)
	}
	if strings.Contains(kind, "site") || strings.Contains(kind, "url") || strings.Contains(intent, "connect") || strings.Contains(intent, "sign") {
		addReason("Cüzdan bağlantısı veya imza akışı kullanıcı varlığı için doğrudan risk oluşturabilir.")
		addStep("Bağlanmadan önce domain, izinler ve imzalanacak işlem içeriğini kontrol et.")
		resp.Score = maxInt(resp.Score, 60)
	}
	if strings.Contains(intent, "airdrop") || strings.Contains(intent, "claim") || strings.Contains(intent, "free") || strings.Contains(intent, "urgent") || strings.Contains(intent, "guarantee") {
		addReason("Claim/airdrop/aciliyet dili dolandırıcılık kampanyalarında sık görülür.")
		addStep("Sıradışı izin isteyen akışları durdur ve resmi kaynaklardan doğrula.")
		resp.RiskLevel = "high"
		resp.Score = maxInt(resp.Score, 78)
		resp.Decision = "warn"
	}
	if strings.Contains(intent, "recovery phrase") || strings.Contains(intent, "unlimited approval") || strings.Contains(intent, "full permission") {
		addReason("Gizli kurtarma ifadesi veya sınırsız izin isteyen akış kritik risk taşır.")
		addStep("İşlemi durdur ve yalnızca resmi, doğrulanmış kaynakları kullan.")
		resp.RiskLevel = "high"
		resp.Score = 95
		resp.Decision = "blocked"
	}
	if len(resp.Reasons) == 0 {
		resp.RiskLevel = "low"
		resp.Score = 25
		resp.Decision = "allow"
		addReason("Belirgin yüksek risk sinyali tespit edilmedi.")
		addStep("Yine de işlem detaylarını imzalamadan önce kontrol et.")
	}
	if resp.Decision == "review" && resp.Score >= 70 {
		resp.Decision = "warn"
		resp.RiskLevel = "high"
	}
	resp.HumanMessage = "ARVIS ön kontrol sonucu: " + resp.Decision + " / " + resp.RiskLevel + ". İmzadan önce kanıtları ve izinleri doğrula."
	return resp
}

func isOfficialKOSCHMint(target string) bool {
	return strings.EqualFold(strings.TrimSpace(target), officialKOSCHMint)
}
