package handlers

import (
	"fmt"
	"math"
	"strings"

	"koschei/api/internal/services"
)

type scanExplanationV2 struct {
	CaseClass          string   `json:"case_class"`
	DominantDriver     string   `json:"dominant_driver"`
	Lead               string   `json:"lead"`
	SupportingEvidence []string `json:"supporting_evidence"`
	Limits             string   `json:"limits,omitempty"`
	Disclaimer         string   `json:"disclaimer"`
	Text               string   `json:"text"`
}

type scanExplanationInput struct {
	Target         string
	RiskIndex      float64
	RiskLevel      string
	Signed         bool
	Policy         string
	Distribution   map[string]any
	Warning        map[string]any
	Holder         services.HolderIntelligence
	Cluster        services.HolderClusterAnalysis
	Launch         services.LaunchForensicsAnalysis
	Modules        []map[string]any
	RepeatDominant []services.RepeatDominantHolderEvidence
}

func buildScanExplanationV2(in scanExplanationInput) scanExplanationV2 {
	top1, top10 := explanationConcentration(in)
	repeat, repeatIsDriver := explanationRepeatDriver(in)
	coordinated := explanationCoordinated(in.Cluster)
	sufficientClean := explanationCleanCoverage(in, top1, top10)

	caseClass := "insufficient_evidence"
	driver := explanationModuleDriver(in.Modules)
	switch {
	case !in.Signed || in.Policy == "withhold" || in.Holder.FinalVerdictBlocked:
		caseClass = "insufficient_evidence"
		driver = "evidence_gap"
	case repeatIsDriver:
		caseClass = "critical_concentration"
		driver = "repeat_dominant_holder"
	case top1 >= 50 || (top1 >= 35 && in.RiskIndex >= 65):
		caseClass = "critical_concentration"
		driver = "holder_concentration"
	case coordinated:
		caseClass = "coordinated_cluster"
		driver = "coordinated_cluster"
	case sufficientClean:
		caseClass = "clean_distributed"
		driver = "distributed_holders"
	}

	lead := explanationLead(caseClass, driver, in, top1, top10, repeat)
	supporting := explanationSupporting(caseClass, driver, in, top1, top10, repeat)
	limits := explanationLimits(in)
	disclaimer := "Kanıt kapsamıdır; kimlik/niyet iddiası veya yatırım tavsiyesi değildir."
	parts := []string{lead}
	parts = append(parts, supporting...)
	if limits != "" {
		parts = append(parts, limits)
	}
	parts = append(parts, disclaimer)
	return scanExplanationV2{CaseClass: caseClass, DominantDriver: driver, Lead: lead, SupportingEvidence: supporting, Limits: limits, Disclaimer: disclaimer, Text: strings.Join(parts, "\n")}
}

func explanationConcentration(in scanExplanationInput) (float64, float64) {
	if in.Holder.Available {
		return in.Holder.Top1Percentage, in.Holder.Top10Percentage
	}
	return radarDetailNumber(in.Distribution["top_1_percentage"]), radarDetailNumber(in.Distribution["top_10_percentage"])
}

func explanationRepeatDriver(in scanExplanationInput) (services.RepeatDominantHolderEvidence, bool) {
	var best services.RepeatDominantHolderEvidence
	for _, item := range in.RepeatDominant {
		if item.RiskWeight > best.RiskWeight {
			best = item
		}
	}
	if best.RiskWeight == 0 {
		return best, false
	}
	moduleRisk := 0.0
	for _, module := range in.Modules {
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(module["module_id"])), "final_verdict_engine") {
			continue
		}
		if verified, _ := module["verified"].(bool); verified {
			moduleRisk = math.Max(moduleRisk, radarDetailNumber(module["risk_index"]))
		}
	}
	return best, float64(best.RiskWeight) >= moduleRisk
}

func explanationCoordinated(cluster services.HolderClusterAnalysis) bool {
	return cluster.Available && (cluster.RiskIndex >= 45 || cluster.SharedFundingGroupCount > 0 || cluster.SynchronizedWalletCount >= 2 || cluster.Flow.CommonExitGroupCount > 0)
}

func explanationCleanCoverage(in scanExplanationInput, top1, top10 float64) bool {
	if !in.Signed || in.Policy == "withhold" || !in.Holder.Available || top1 >= 20 || top10 >= 60 || explanationCoordinated(in.Cluster) || len(in.RepeatDominant) > 0 {
		return false
	}
	required := in.Holder.RiskBearingOwnerCount / 2
	if required < 2 {
		required = 2
	}
	return in.Holder.WalletsWithParsedEvidence >= required && in.Launch.OwnersWithTradeHistory >= required
}

func explanationModuleDriver(modules []map[string]any) string {
	best := ownerRadarPrimaryRiskDriver(modules)
	if best == nil {
		return "verified_modules"
	}
	return strings.TrimSpace(fmt.Sprint(best["module_id"]))
}

func explanationLead(caseClass, driver string, in scanExplanationInput, top1, top10 float64, repeat services.RepeatDominantHolderEvidence) string {
	switch caseClass {
	case "critical_concentration":
		if driver == "repeat_dominant_holder" && strings.TrimSpace(repeat.EvidenceLine) != "" {
			position := explanationPositionLiquidity(in.Holder)
			if position != "" {
				return repeat.EvidenceLine + " " + position
			}
			return repeat.EvidenceLine
		}
		owner := "tek bir risk taşıyan owner"
		if top := explanationTopRiskRow(in.Holder); top != nil && strings.TrimSpace(top.OwnerWallet) != "" {
			owner = ownerRadarShortTarget(top.OwnerWallet)
		}
		lead := fmt.Sprintf("%s dolaşımdaki arzın %.2f%%'sini kontrol ediyor; ilk 10 owner toplamı %.2f%%.", owner, top1, top10)
		if position := explanationPositionLiquidity(in.Holder); position != "" {
			lead += " " + position
		}
		return lead
	case "coordinated_cluster":
		linked := in.Cluster.LinkedHolderPercentage
		if in.Cluster.Flow.LinkedHolderPercentage > linked {
			linked = in.Cluster.Flow.LinkedHolderPercentage
		}
		return fmt.Sprintf("Holder geçmişinde koordinasyon baskın risk sürücüsü: %d senkron cüzdan, %d ortak fonlama grubu ve %d ortak çıkış grubu gözlendi; bağlantılı pay %.2f%%.", in.Cluster.SynchronizedWalletCount, in.Cluster.SharedFundingGroupCount, in.Cluster.Flow.CommonExitGroupCount, linked)
	case "clean_distributed":
		return fmt.Sprintf("Dağılım belirgin bir baskın owner göstermiyor: Top 1 %.2f%%, Top 10 %.2f%%; yeterli gözlem penceresinde doğrulanmış koordinasyon bulgusu çıkmadı.", top1, top10)
	default:
		if in.Holder.Available {
			return fmt.Sprintf("Holder bakiyeleri görüldü (Top 1 %.2f%%, Top 10 %.2f%%) ancak final hikâye için kanıt kapsamı yetersiz; eksik gözlem güvenlik sinyali sayılmadı.", top1, top10)
		}
		return "Bu taramada doğrulanmış holder ve davranış kanıtı final risk hikâyesi kurmaya yetmedi; veri yokluğu düşük risk olarak yorumlanmadı."
	}
}

func explanationPositionLiquidity(holder services.HolderIntelligence) string {
	top := explanationTopRiskRow(holder)
	if top == nil || top.ReferenceUSDValue == nil || *top.ReferenceUSDValue <= 0 || holder.Market.LiquidityUSD <= 0 {
		return ""
	}
	ratio := *top.ReferenceUSDValue / holder.Market.LiquidityUSD
	return fmt.Sprintf("Pozisyon yaklaşık $%.1fK; doğrulanmış havuz $%.1fK ve pozisyon havuzun ~%.1f katı.", *top.ReferenceUSDValue/1000, holder.Market.LiquidityUSD/1000, ratio)
}

func explanationTopRiskRow(holder services.HolderIntelligence) *services.HolderIntelligenceRow {
	for i := range holder.Rows {
		if holder.Rows[i].RiskBearing && !holder.Rows[i].ExcludedFromHolderRisk {
			return &holder.Rows[i]
		}
	}
	return nil
}

func explanationSupporting(caseClass, driver string, in scanExplanationInput, top1, top10 float64, repeat services.RepeatDominantHolderEvidence) []string {
	out := []string{}
	if driver != "repeat_dominant_holder" && strings.TrimSpace(repeat.EvidenceLine) != "" {
		out = append(out, repeat.EvidenceLine)
	}
	protocol := radarDetailNumber(in.Distribution["protocol_controlled_percentage"]) + radarDetailNumber(in.Distribution["burn_percentage"])
	if protocol > 0 && caseClass != "clean_distributed" {
		out = append(out, fmt.Sprintf("LP/protokol/burn envanterinin %.2f%%'si ordinary-holder yoğunlaşmasından ayrı hesaplandı; yukarıdaki yüzdeler owner-bazlı dolaşım payıdır.", protocol))
	}
	if explanationCoordinated(in.Cluster) {
		for _, finding := range in.Cluster.Findings {
			finding = strings.TrimSpace(finding)
			if finding != "" {
				out = appendUniqueExplanation(out, finding)
			}
			if len(out) >= 3 {
				break
			}
		}
	}
	if in.Launch.OwnersWithTradeHistory > 0 && (in.Launch.SniperCount > 0 || in.Launch.RhythmBotCount > 0 || in.Launch.CreatorLinkedCount > 0) {
		out = append(out, fmt.Sprintf("Lansman geçmişinde %d sniper, %d ritim-botu ve %d creator-bağlantılı owner kanıtlandı (%d/%d owner geçmişi çözüldü).", in.Launch.SniperCount, in.Launch.RhythmBotCount, in.Launch.CreatorLinkedCount, in.Launch.OwnersWithTradeHistory, in.Launch.OwnersRequested))
	}
	mintAuth, _ := explanationModuleBool(in.Modules, "mint_authority_present")
	freezeAuth, _ := explanationModuleBool(in.Modules, "freeze_authority_present")
	if mintAuth || freezeAuth {
		active := []string{}
		if mintAuth {
			active = append(active, "mint")
		}
		if freezeAuth {
			active = append(active, "freeze")
		}
		out = append(out, "Aktif yetki riski: "+strings.Join(active, " + ")+" authority açık.")
	}
	if caseClass == "critical_concentration" && top1 >= 50 {
		out = append(out, fmt.Sprintf("Yoğunlaşma tek başına doğrulanmış ana bulgu: Top 1 %.2f%%, Top 10 %.2f%%.", top1, top10))
	}
	return out
}

func explanationModuleBool(modules []map[string]any, key string) (bool, bool) {
	for _, module := range modules {
		signals, _ := module["signals"].(map[string]any)
		if value, ok := signals[key].(bool); ok {
			return value, true
		}
	}
	return false, false
}

func explanationLimits(in scanExplanationInput) string {
	limits := []string{}
	if in.Launch.OwnersRequested > 0 && in.Launch.OwnersWithTradeHistory < in.Launch.OwnersRequested {
		limits = append(limits, fmt.Sprintf("lansman/işlem geçmişi %d owner'dan yalnızca %d'i için çözüldü; %d sniper sonucu bu kapsamda temiz sinyal değildir", in.Launch.OwnersRequested, in.Launch.OwnersWithTradeHistory, in.Launch.SniperCount))
	}
	limitedOwners := 0
	for _, row := range in.Holder.Rows {
		if row.ObservationBudgetDegraded || row.ObservationStatus == "rpc_budget_exhausted" || row.ObservationStatus == "no_observed_signatures" || row.ObservationStatus == "signature_only_observation" {
			limitedOwners++
		}
	}
	if limitedOwners > 0 {
		limits = append(limits, fmt.Sprintf("%d owner yalnızca bounded/shallow veya RPC-bütçe-sınırlı pencerede gözlendi; bu boşluk güvenli veya organik olarak sınıflandırılmadı", limitedOwners))
	}
	if in.Holder.FinalVerdictBlocked || in.Policy == "withhold" {
		limits = append(limits, "çözülemeyen baskın rol nedeniyle final sonuç bekletildi")
	}
	if len(limits) == 0 {
		return ""
	}
	return "Sınırlar: " + strings.Join(limits, "; ") + "."
}

func appendUniqueExplanation(dst []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return dst
	}
	for _, existing := range dst {
		if strings.EqualFold(strings.TrimSpace(existing), value) {
			return dst
		}
	}
	return append(dst, value)
}
