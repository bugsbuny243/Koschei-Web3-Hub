package handlers

import (
	"net/http"
	"net/url"
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
	raise := func(score int, decision, level string) {
		resp.Score = maxInt(resp.Score, score)
		if decision != "" {
			resp.Decision = decision
		}
		if level != "" {
			resp.RiskLevel = level
		}
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
		addStep("İşlem yapmadan önce mint adresini resmi /token sayfasındaki adresle tekrar karşılaştır.")
		addStep("KOSCH erişimi için yalnızca resmi /kosch-access wallet doğrulama akışını kullan.")
		resp.HumanMessage = "ARVIS ön kontrol sonucu: resmi KOSCH mint eşleşti. Bu finansal tavsiye değildir; işlem yapmadan önce zincir üstü bilgileri ve resmi sayfayı doğrula."
		return resp
	}
	if host := arvisPreflightHost(target); host != "" {
		addReason("Hedef bir web/domain yüzeyi içeriyor; bağlantı ve imza riski domain seviyesinde kontrol edilmeli.")
		addStep("Domaini resmi sosyal hesaplar ve dokümantasyonla birebir karşılaştır.")
		if arvisIsTrustedKoscheiHost(host) {
			addReason("Domain resmi Koschei alan adıyla eşleşiyor.")
		} else {
			raise(58, "review", "medium")
		}
		if arvisIsURLShortener(host) {
			addReason("Kısaltılmış link gerçek hedefi gizleyebilir; Web3 imza akışlarında yüksek dikkat gerekir.")
			addStep("Kısaltılmış linki açma; gerçek domaini resmi kaynaklardan doğrula.")
			raise(76, "warn", "high")
		}
		if strings.Contains(host, "xn--") {
			addReason("Domain punycode/idn biçimi içeriyor; görsel olarak resmi siteye benzeyen sahte domain riski olabilir.")
			addStep("Domaini kopyalayıp harf harf kontrol et; şüphede işlemi durdur.")
			raise(82, "warn", "high")
		}
		if strings.Contains(target, "@") && strings.Contains(target, "://") {
			addReason("URL içinde @ karakteri var; bu yapı kullanıcıyı farklı bir hosta yönlendirmek için kötüye kullanılabilir.")
			addStep("Bu link üzerinden cüzdan bağlama veya imza verme.")
			raise(85, "warn", "high")
		}
		if arvisLooksLikeBrandImpersonation(host) {
			addReason("Domain popüler cüzdan/DEX/marketplace markalarına benzer ifade içeriyor ancak bilinen resmi hostla eşleşmiyor.")
			addStep("Resmi uygulamayı doğrudan yazılmış URL veya doğrulanmış sosyal bağlantıdan aç.")
			raise(78, "warn", "high")
		}
	}
	if solanaPreflightAddressLike.MatchString(target) {
		addReason("Hedef Solana adres formatına benziyor; zincir üstü kanıtlarla doğrulanmalı.")
		addStep("Token, wallet veya program adresini ARVIS dashboard içinde detaylı tara.")
	} else if arvisPreflightHost(target) == "" {
		addReason("Hedef adres formatı doğrulanamadı; sahte site veya hatalı adres riski kontrol edilmeli.")
		addStep("Resmi kaynaklardan adres/site doğrulaması yap.")
		resp.Score = maxInt(resp.Score, 55)
	}
	if strings.Contains(kind, "site") || strings.Contains(kind, "url") || strings.Contains(intent, "connect") || strings.Contains(intent, "sign") || strings.Contains(intent, "imza") {
		addReason("Cüzdan bağlantısı veya imza akışı kullanıcı varlığı için doğrudan risk oluşturabilir.")
		addStep("Bağlanmadan önce domain, izinler ve imzalanacak işlem içeriğini kontrol et.")
		resp.Score = maxInt(resp.Score, 60)
	}
	if arvisContainsAny(intent, []string{"airdrop", "claim", "free", "urgent", "guarantee", "bonus", "reward", "hediye", "acil", "ücretsiz", "odul", "ödül"}) {
		addReason("Claim/airdrop/aciliyet dili dolandırıcılık kampanyalarında sık görülür.")
		addStep("Sıradışı izin isteyen akışları durdur ve resmi kaynaklardan doğrula.")
		raise(78, "warn", "high")
	}
	if arvisContainsAny(intent, []string{"recovery phrase", "seed phrase", "mnemonic", "private key", "secret phrase", "12 words", "24 words", "gizli anahtar", "kurtarma ifadesi", "seed", "özel anahtar"}) {
		addReason("Gizli kurtarma ifadesi, seed phrase veya private key isteyen akış kritik risk taşır.")
		addStep("İşlemi durdur. Seed phrase/private key hiçbir sitede paylaşılmaz.")
		resp.RiskLevel = "high"
		resp.Score = 98
		resp.Decision = "blocked"
	}
	if arvisContainsAny(intent, []string{"unlimited approval", "full permission", "setauthority", "delegate", "approve all", "sweep", "drain", "drainer", "sınırsız izin", "tam yetki"}) {
		addReason("Sınırsız izin, delegate/setAuthority veya varlık boşaltma benzeri ifade kritik işlem riski taşır.")
		addStep("İmzayı durdur; transaction instruction detayını ARVIS veya wallet simülasyonuyla doğrula.")
		raise(94, "blocked", "high")
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

func arvisPreflightHost(target string) string {
	value := strings.TrimSpace(target)
	if value == "" || strings.ContainsAny(value, " \t\n\r") {
		return ""
	}
	parseValue := value
	if !strings.Contains(parseValue, "://") {
		if !strings.Contains(parseValue, ".") {
			return ""
		}
		parseValue = "https://" + parseValue
	}
	parsed, err := url.Parse(parseValue)
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" || !strings.Contains(host, ".") {
		return ""
	}
	return strings.Trim(host, ".")
}

func arvisIsTrustedKoscheiHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "tradepigloball.co" || host == "www.tradepigloball.co"
}

func arvisIsURLShortener(host string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(host), "www."))
	shorteners := []string{"bit.ly", "tinyurl.com", "t.co", "shorturl.at", "cutt.ly", "is.gd", "buff.ly", "rebrand.ly", "ow.ly", "lnkd.in"}
	for _, shortener := range shorteners {
		if host == shortener {
			return true
		}
	}
	return false
}

func arvisLooksLikeBrandImpersonation(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	knownHosts := []string{"phantom.app", "solflare.com", "jup.ag", "raydium.io", "pump.fun", "dexscreener.com", "solscan.io", "tradepigloball.co", "www.tradepigloball.co"}
	for _, known := range knownHosts {
		if host == known || strings.HasSuffix(host, "."+known) {
			return false
		}
	}
	brandTerms := []string{"phantom", "solflare", "jupiter", "raydium", "pumpfun", "pump-fun", "dexscreener", "solscan", "walletconnect", "airdrop", "claim"}
	compact := strings.ReplaceAll(strings.ReplaceAll(host, "-", ""), ".", "")
	for _, term := range brandTerms {
		if strings.Contains(compact, strings.ReplaceAll(term, "-", "")) {
			return true
		}
	}
	return false
}

func arvisContainsAny(value string, terms []string) bool {
	value = strings.ToLower(value)
	for _, term := range terms {
		if strings.Contains(value, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
