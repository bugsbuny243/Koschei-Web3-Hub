package handlers

import (
	"net/http"
	"regexp"
	"strings"
)

type guardCheckRequest struct {
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Intent string `json:"intent"`
	Note   string `json:"note"`
}

type guardCheckResponse struct {
	OK             bool     `json:"ok"`
	Decision       string   `json:"decision"`
	RiskLevel      string   `json:"risk_level"`
	Score          int      `json:"score"`
	Reasons        []string `json:"reasons"`
	NextSteps      []string `json:"next_steps"`
	HumanMessage   string   `json:"human_message"`
	CreditsCharged bool     `json:"credits_charged"`
}

var solanaAddressLike = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)

func (h *Handler) GuardCheck(w http.ResponseWriter, r *http.Request) {
	if h.Limiter != nil && !h.Limiter.allow("guard-check:"+clientIP(r), 20, 60_000_000_000) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	}
	var req guardCheckRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	writeJSON(w, http.StatusOK, evaluateGuardCheck(req))
}

func evaluateGuardCheck(req guardCheckRequest) guardCheckResponse {
	target := strings.TrimSpace(req.Target)
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	text := strings.ToLower(strings.TrimSpace(req.Intent + " " + req.Note + " " + req.Target))
	resp := guardCheckResponse{OK: true, Decision: "review", RiskLevel: "medium", Score: 45, CreditsCharged: false}
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
		addReason("Bağlantı HTTPS değil; güvenli olmayan yönlendirme riski var.")
	}
	if strings.Contains(text, "claim") || strings.Contains(text, "airdrop") || strings.Contains(text, "xn--") {
		resp.Score += 20
		addReason("Hedefte ödül, claim veya benzer alan adı riski var.")
	}
	if strings.Contains(text, "recovery phrase") || strings.Contains(text, "secret words") || strings.Contains(text, "kurtarma kelimeleri") {
		resp.Score = 100
		resp.Decision = "blocked"
		resp.RiskLevel = "critical"
		addReason("Cüzdan kurtarma bilgisi isteniyor; bu doğrudan hesap kaybı riski doğurur.")
		addStep("Bu bilgiyi hiçbir siteye veya kişiye verme.")
		resp.HumanMessage = "Bu işlem durdurulmalı. Kurtarma bilgisi isteyen akış güvenli değildir."
		return resp
	}
	if strings.Contains(text, "approve") || strings.Contains(text, "sign") || strings.Contains(text, "imza") || strings.Contains(text, "connect wallet") || strings.Contains(text, "cüzdan bağla") {
		resp.Score += 18
		addReason("Cüzdan imzası veya onay isteniyor; işlem öncesi doğrulama gerekli.")
		addStep("İmza ekranında beklenmeyen token hareketi veya harcama yetkisi olup olmadığını kontrol et.")
	}
	if strings.Contains(text, "guaranteed") || strings.Contains(text, "garanti") || strings.Contains(text, "kesin kazanç") || strings.Contains(text, "100x") {
		resp.Score += 25
		addReason("Garanti getiri veya aşırı kazanç vaadi var; risk artıyor.")
	}
	if kind == "token" || kind == "mint" || solanaAddressLike.MatchString(target) {
		addStep("Resmi mint adresini proje sitesi ve resmi sosyal hesaplarla çapraz doğrula.")
		addStep("Authority, arz, holder dağılımı ve likidite bilgisini kontrol et.")
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
	addStep("Resmi kaynakları ve zincir üstü veriyi doğrula.")
	addStep("Şüphe varsa cüzdanı bağlama, imzalama veya fon gönderme.")
	resp.HumanMessage = guardHumanMessage(resp.Decision)
	return resp
}

func guardHumanMessage(decision string) string {
	switch decision {
	case "blocked":
		return "Bu akış güvenli görünmüyor. İşlemi durdurmanı öneririm."
	case "avoid":
		return "Yüksek risk var. Resmi doğrulama yapılmadan işlem yapma."
	case "caution":
		return "Net bir kırmızı bayrak yok, ama zincir üstü doğrulama yapılmadan güvenli kabul edilmez."
	default:
		return "Orta risk var. İmza atmadan veya fon göndermeden önce ek kanıtları kontrol et."
	}
}
