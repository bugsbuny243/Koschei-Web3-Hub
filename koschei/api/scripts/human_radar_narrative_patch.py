from pathlib import Path
import re

p = Path('internal/handlers/owner_operations.go')
s = p.read_text()

old_call = 'detail["narrative"] = ownerRadarNarrative(target, final, warning, distribution, sourceContext)'
new_call = '''detail["primary_risk_driver"] = ownerRadarPrimaryRiskDriver(modules)
\tdetail["narrative"] = ownerRadarNarrative(target, final, warning, distribution, sourceContext, modules)'''
if old_call not in s:
    raise SystemExit('narrative call not found')
s = s.replace(old_call, new_call, 1)

pattern = re.compile(r'func ownerRadarNarrative\(target string, final, warning, distribution, source map\[string\]any\) string \{.*?\n\}\n\n// OwnerKOSCHAccess', re.S)
replacement = r'''func ownerRadarNarrative(target string, final, warning, distribution, source map[string]any, modules []map[string]any) string {
	signed, _ := final["signed"].(bool)
	level := strings.ToLower(strings.TrimSpace(fmt.Sprint(final["risk_level"])))
	if !signed || final["risk_index"] == nil || level == "" || level == "unknown" || level == "<nil>" {
		return "Koschei bu hedef için doğrulanmış bir final risk puanı üretmedi. Kanıt eksikliği düşük risk anlamına gelmez; eksik modüller tamamlanana kadar sonuç EVIDENCE PENDING olarak değerlendirilmelidir."
	}

	risk := radarDetailNumber(final["risk_index"])
	parts := []string{
		fmt.Sprintf("Koschei bu tokenı (%s) %.0f/100 ile %s risk seviyesinde değerlendiriyor. %s", ownerRadarShortTarget(target), risk, ownerRadarRiskLabelTR(level), ownerRadarRiskMeaning(level)),
	}

	if available, _ := distribution["available"].(bool); available {
		top1 := radarDetailNumber(distribution["top_1_percentage"])
		top10 := radarDetailNumber(distribution["top_10_percentage"])
		top20 := radarDetailNumber(distribution["top_20_percentage"])
		if adjusted, _ := distribution["role_adjusted"].(bool); adjusted {
			protocolPct := radarDetailNumber(distribution["protocol_controlled_percentage"])
			role := ownerRadarRoleTR(strings.TrimSpace(fmt.Sprint(distribution["dominant_role"])))
			parts = append(parts, fmt.Sprintf("Holder hesabında ham arzın %.2f%%'si doğrulanmış bonding-curve veya protokol envanteri olduğu için normal bir balina gibi sayılmadı. Bu ayrımdan sonra en büyük gerçek holderın payı %.2f%%, ilk 10 holderın toplamı %.2f%% ve ilk 20 holderın toplamı %.2f%% olarak ölçüldü; baskın hesap tipi %s.", protocolPct, top1, top10, top20, role))
		} else {
			parts = append(parts, fmt.Sprintf("Gözlenen holder dağılımında en büyük hesap %.2f%%, ilk 10 hesap %.2f%% ve ilk 20 hesap %.2f%% paya sahip.", top1, top10, top20))
		}
		parts = append(parts, ownerRadarHolderMeaning(top1, top10))
		if blocked, _ := distribution["blocking_evidence_gap"].(bool); blocked {
			parts = append(parts, "Ancak baskın token hesabının ekonomik rolü çözülemediği için holder tarafında kesin sonuç verilmedi; veri yokluğu güvenli kabul edilmedi.")
		}
	}

	positives := ownerRadarStringSlice(warning["positive_signals"])
	if len(positives) > 0 {
		if len(positives) > 3 {
			positives = positives[:3]
		}
		parts = append(parts, "Olumlu sinyaller: "+strings.Join(positives, " "))
	}

	if driver := ownerRadarPrimaryRiskDriver(modules); len(driver) > 0 {
		name := strings.TrimSpace(fmt.Sprint(driver["module"]))
		score := radarDetailNumber(driver["risk_index"])
		verdict := strings.TrimSpace(fmt.Sprint(driver["verdict"]))
		if verdict == "" || verdict == "<nil>" {
			verdict = strings.TrimSpace(fmt.Sprint(driver["recommendation"]))
		}
		if verdict != "" && verdict != "<nil>" {
			parts = append(parts, fmt.Sprintf("Final puanı yukarı taşıyan ana risk sürücüsü %s modülüdür (%.0f/100). Modülün yorumu: %s", name, score, verdict))
		} else {
			parts = append(parts, fmt.Sprintf("Final puanı yukarı taşıyan ana risk sürücüsü %s modülüdür (%.0f/100).", name, score))
		}
	}

	creator := strings.TrimSpace(fmt.Sprint(source["creator_wallet"]))
	if creator != "" && creator != "<nil>" {
		parts = append(parts, "Launch kaynağında creator/deployer ile ilişkili görünen cüzdan "+creator+" olarak gözlendi. Bu yalnızca zincir üstü veya kaynak temelli bir ilişkiyi gösterir; tek başına kötü niyet kanıtı değildir.")
	} else {
		parts = append(parts, "Creator/deployer cüzdanı bu taramada doğrulanamadı. Bu, creator olmadığı anlamına gelmez; yalnızca mevcut kaynakların ilişkiyi çözemediğini gösterir.")
	}

	parts = append(parts, ownerRadarPracticalConclusion(level))
	parts = append(parts, "Bu değerlendirme kanıt kapsamındadır; kötü niyet, dolandırıcılık veya gerçek kişi kimliği iddiası değildir.")
	return strings.Join(parts, " ")
}

func ownerRadarPrimaryRiskDriver(modules []map[string]any) map[string]any {
	var best map[string]any
	bestRisk := -1.0
	for _, module := range modules {
		moduleID := strings.ToLower(strings.TrimSpace(fmt.Sprint(module["module_id"])))
		if moduleID == "" || moduleID == "final_verdict_engine" {
			continue
		}
		verified, _ := module["verified"].(bool)
		signed, _ := module["signed"].(bool)
		if !verified || !signed {
			continue
		}
		risk := radarDetailNumber(module["risk_index"])
		if risk > bestRisk {
			bestRisk = risk
			best = module
		}
	}
	return best
}

func ownerRadarStringSlice(raw any) []string {
	out := []string{}
	switch values := raw.(type) {
	case []string:
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				out = append(out, value)
			}
		}
	case []any:
		for _, rawValue := range values {
			if value := strings.TrimSpace(fmt.Sprint(rawValue)); value != "" && value != "<nil>" {
				out = append(out, value)
			}
		}
	}
	return out
}

func ownerRadarShortTarget(target string) string {
	target = strings.TrimSpace(target)
	if len(target) <= 18 {
		return target
	}
	return target[:9] + "…" + target[len(target)-7:]
}

func ownerRadarRiskLabelTR(level string) string {
	switch level {
	case "critical":
		return "KRİTİK"
	case "high":
		return "YÜKSEK"
	case "medium":
		return "ORTA"
	case "low":
		return "DÜŞÜK"
	default:
		return strings.ToUpper(level)
	}
}

func ownerRadarRiskMeaning(level string) string {
	switch level {
	case "critical", "high":
		return "Bu seviye, doğrulanmış risk sinyallerinin işlem öncesinde ayrıntılı biçimde incelenmesi gerektiğini gösterir; otomatik olarak rug veya dolandırıcılık hükmü değildir."
	case "medium":
		return "Bu seviye doğrudan rug kanıtı değildir; bazı risk kollarının temiz görünürken en az bir doğrulanmış modülün ek inceleme istediğini gösterir."
	case "low":
		return "Bu seviye mevcut kanıtlarda ağır bir risk sürücüsü görülmediğini gösterir; yine de düşük risk, risksiz anlamına gelmez."
	default:
		return "Karar yalnızca mevcut ve doğrulanmış kanıtların kapsamını yansıtır."
	}
}

func ownerRadarRoleTR(role string) string {
	switch role {
	case "externally_owned_wallet":
		return "normal kullanıcı cüzdanı"
	case "pump_bonding_curve_or_protocol_vault":
		return "Pump bonding-curve/protokol kasası"
	case "pump_liquidity_vault":
		return "Pump likidite kasası"
	case "burn_sink":
		return "burn adresi"
	case "program_controlled_unresolved":
		return "rolü henüz çözülememiş program kontrollü hesap"
	default:
		if role == "" || role == "<nil>" {
			return "belirlenemeyen hesap"
		}
		return strings.ReplaceAll(role, "_", " ")
	}
}

func ownerRadarHolderMeaning(top1, top10 float64) string {
	switch {
	case top1 < 5 && top10 < 25:
		return "Bu dağılım tek başına ciddi bir balina veya arz merkezileşmesi göstermiyor; holder tarafı görece dengeli görünüyor."
	case top1 < 15 && top10 < 50:
		return "Holder dağılımı belirgin bir tek-cüzdan hâkimiyeti göstermiyor, ancak büyük hesapların hareketleri izlenmeye devam edilmelidir."
	case top1 >= 50:
		return "Tek bir risk taşıyan cüzdan arzın yarısından fazlasını kontrol ediyor; satış hâlinde ciddi fiyat ve exit-liquidity baskısı oluşabilir."
	case top1 >= 20 || top10 >= 75:
		return "Holder dağılımı merkezileşmiş görünüyor; büyük cüzdanların satış ve bağlantı geçmişi çözülmeden güvenli kabul edilmemelidir."
	default:
		return "Holder yoğunluğu orta seviyede; tek başına nihai karar vermek için creator, likidite, funding cluster ve zamanlama kanıtlarıyla birlikte okunmalıdır."
	}
}

func ownerRadarPracticalConclusion(level string) string {
	switch level {
	case "critical", "high":
		return "Pratik sonuç: işlem yapmadan önce ana risk sürücüsünün kanıtlarını, likidite çıkış yollarını, creator geçmişini ve bağlı cüzdan kümelerini doğrulamak gerekir."
	case "medium":
		return "Pratik sonuç: holder dağılımı rahat görünse bile token otomatik olarak güvenli sayılmaz. Ana risk modülü, likidite davranışı, creator geçmişi ve Sybil/funding-cluster bağlantıları birlikte incelenmelidir."
	case "low":
		return "Pratik sonuç: mevcut taramada ağır bir alarm yok; yine de likidite, creator ve cüzdan kümeleri değişebileceği için karar güncel verilerle yenilenmelidir."
	default:
		return "Pratik sonuç: kanıt kapsamı tamamlanmadan kesin güvenlik yorumu yapılmamalıdır."
	}
}

// OwnerKOSCHAccess'''

if not pattern.search(s):
    raise SystemExit('ownerRadarNarrative block not found')
s = pattern.sub(replacement, s, count=1)
p.write_text(s)

Path('internal/handlers/owner_operations_narrative_test.go').write_text(r'''package handlers

import (
	"strings"
	"testing"
)

func TestOwnerRadarNarrativeExplainsMeaning(t *testing.T) {
	final := map[string]any{"risk_index": 53, "risk_level": "medium", "signed": true}
	warning := map[string]any{"positive_signals": []string{"Mint authority kapalı/revoked olarak gözlendi.", "Freeze authority kapalı/revoked olarak gözlendi."}}
	distribution := map[string]any{
		"available": true, "role_adjusted": true, "blocking_evidence_gap": false,
		"top_1_percentage": 2.4774, "top_10_percentage": 14.6475, "top_20_percentage": 22.108,
		"protocol_controlled_percentage": 2.0999, "dominant_role": "externally_owned_wallet",
	}
	modules := []map[string]any{{
		"module": "Sniper Timing Detector", "module_id": "sniper_timing_detector",
		"risk_index": 53, "risk_level": "medium", "verified": true, "signed": true,
		"verdict": "Ardışık slotlarda kümelenen alımlar ek inceleme gerektiriyor.",
	}}
	text := ownerRadarNarrative("4ko5tSr5o3H4v1sFtjTSd9MPUW7yx5AFCpkNPoL6pump", final, warning, distribution, map[string]any{}, modules)
	for _, expected := range []string{
		"53/100 ile ORTA risk seviyesinde", "tek başına ciddi bir balina", "Olumlu sinyaller:",
		"ana risk sürücüsü Sniper Timing Detector", "Creator/deployer cüzdanı bu taramada doğrulanamadı",
		"Pratik sonuç:",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in narrative: %s", expected, text)
		}
	}
	if strings.Contains(text, "ARVIS kararı: MEDIUM, risk") {
		t.Fatalf("machine-like legacy summary returned: %s", text)
	}
}

func TestOwnerRadarNarrativeEvidencePending(t *testing.T) {
	text := ownerRadarNarrative("target", map[string]any{"risk_index": nil, "risk_level": "unknown", "signed": false}, map[string]any{}, map[string]any{}, map[string]any{}, nil)
	if !strings.Contains(text, "EVIDENCE PENDING") || strings.Contains(text, "/100") {
		t.Fatalf("unexpected pending narrative: %s", text)
	}
}
''')
