package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/services"
)

// OwnerOperationsStatus is the KOSCH-era owner dashboard contract. It avoids
// legacy checkout, package and entitlement concepts entirely.
func (h *Handler) OwnerOperationsStatus(w http.ResponseWriter, r *http.Request) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	ctx := r.Context()
	summary := map[string]any{
		"total_users": 0, "active_users": 0, "verified_wallets": 0,
		"kosch_holders": 0, "kosch_basic": 0, "kosch_pro": 0, "kosch_enterprise": 0,
		"radar_verdicts_24h": 0, "high_risk_24h": 0, "security_events_24h": 0,
		"open_feedback": 0,
	}
	if db != nil {
		if ownerTableExists(ctx, db, "app_user_profiles") {
			summary["total_users"] = ownerCount(ctx, db, `SELECT count(*) FROM app_user_profiles`)
			summary["active_users"] = ownerCount(ctx, db, `SELECT count(*) FROM app_user_profiles WHERE COALESCE(status,'active')='active'`)
		}
		if ownerTableExists(ctx, db, "verified_wallet_links") {
			summary["verified_wallets"] = ownerCount(ctx, db, `SELECT count(DISTINCT auth_subject) FROM verified_wallet_links WHERE status='active'`)
		}
		if ownerTableExists(ctx, db, "token_access_snapshots") {
			latest := `WITH latest AS (
				SELECT DISTINCT ON (auth_subject) auth_subject, tier, amount_raw
				FROM token_access_snapshots
				WHERE expires_at > now()
				ORDER BY auth_subject, checked_at DESC
			)`
			summary["kosch_holders"] = ownerCount(ctx, db, latest+` SELECT count(*) FROM latest WHERE tier IN ('basic','pro','enterprise')`)
			summary["kosch_basic"] = ownerCount(ctx, db, latest+` SELECT count(*) FROM latest WHERE tier='basic'`)
			summary["kosch_pro"] = ownerCount(ctx, db, latest+` SELECT count(*) FROM latest WHERE tier='pro'`)
			summary["kosch_enterprise"] = ownerCount(ctx, db, latest+` SELECT count(*) FROM latest WHERE tier='enterprise'`)
		}
		if ownerTableExists(ctx, db, "security_radar_verdicts") {
			summary["radar_verdicts_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND created_at >= now()-interval '24 hours'`)
			summary["high_risk_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND created_at >= now()-interval '24 hours' AND lower(COALESCE(risk_level,'')) IN ('high','critical')`)
		}
		if ownerTableExists(ctx, db, "security_audit_events") {
			summary["security_events_24h"] = ownerCount(ctx, db, `SELECT count(*) FROM security_audit_events WHERE created_at >= now()-interval '24 hours'`)
		}
		if ownerTableExists(ctx, db, "customer_feedback") {
			summary["open_feedback"] = ownerCount(ctx, db, `SELECT count(*) FROM customer_feedback WHERE status IN ('new','reviewing','planned')`)
		}
	}

	radar := h.securityRadarStreamStats(ctx)
	radarStatus := firstMapString(radar, "pipeline_status")
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SOLANA_RPC_LIMIT_SAVER_ENABLED")), "true") {
		radar["background_streams_paused"] = true
		radar["manual_scans_available"] = true
		if services.PumpHighVolumeRadarEnabled() {
			radarStatus = "selective_auto_volume"
			radar["pump_volume_auto_enabled"] = true
			radar["pump_volume_threshold_usd"] = services.PumpHighVolumeThresholdUSD()
		} else {
			radarStatus = "manual_rpc_saver"
		}
		radar["pipeline_status"] = radarStatus
	}
	servicesMap := map[string]any{
		"database":        map[string]any{"status": serviceStatus(db != nil, "connected", "unavailable")},
		"neon_auth":       map[string]any{"status": serviceStatus(envSet("NEON_AUTH_JWKS_URL"), "configured", "missing")},
		"solana_rpc":      map[string]any{"status": serviceStatus(envSet("SOLANA_RPC_URL") || envSet("ALCHEMY_SOLANA_RPC_URL") || envSet("HELIUS_SOLANA_RPC_URL") || envSet("QUICKNODE_SOLANA_RPC_URL") || envSet("ALCHEMY_API_KEY"), "configured", "missing")},
		"security_radar":  map[string]any{"status": radarStatus},
		"kosch_access":    map[string]any{"status": serviceStatus(configuredKoscheiTokenGateEnabled() && configuredKoscheiTokenMint() != "", "configured", "missing")},
		"visual_renderer": map[string]any{"status": "ready", "mode": "client_canvas_png"},
		"owner_brain":     map[string]any{"status": serviceStatus(aiProviderConfigured(), "configured", "missing")},
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "generated_at": time.Now().UTC(), "summary": summary,
		"services": servicesMap, "radar": radar,
		"access_model": map[string]any{
			"free_core":         []string{"safe_check", "basic_token_scan"},
			"kosch_premium":     []string{"full_radar", "structural_memory", "graph", "exposure", "visual_reports", "automation"},
			"payment_providers": []string{},
		},
	})
}

// OwnerRadarOverview powers the owner ARVIS workspace without requiring a
// customer KOSCH session. The owner cookie remains mandatory at route level.
func (h *Handler) OwnerRadarOverview(w http.ResponseWriter, r *http.Request) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	items := []services.SecurityRadarVerdictRecord{}
	sources := []services.SecurityRadarSource{}
	highVolumePump := []services.PumpHighVolumeOwnerItem{}
	if db != nil {
		store := services.NewSecurityRadarStore(db)
		if loaded, err := store.LatestVerdicts(r.Context(), 100); err == nil {
			items = loaded
		}
		if loaded, err := store.ListSources(r.Context()); err == nil {
			sources = loaded
		}
		if loaded, err := store.LatestPumpHighVolumeReports(r.Context(), 200); err == nil {
			highVolumePump = loaded
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "generated_at": time.Now().UTC(), "items": items,
		"high_volume_pump": highVolumePump,
		"sources":          sources, "pipeline": h.securityRadarStreamStats(r.Context()),
	})
}

// OwnerRadarScan executes the complete evidence pipeline and returns the same
// unabridged detail contract used by premium Radar. It never invents missing
// creator, holder, liquidity or graph evidence.
func (h *Handler) OwnerRadarScan(w http.ResponseWriter, r *http.Request) {
	var input securityRadarInput
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "Invalid request body")
		return
	}
	target := strings.TrimSpace(firstNonEmptyString(input.Target, input.Address))
	if target == "" {
		writeAPIError(w, http.StatusBadRequest, APICodeInvalidInput, "target is required")
		return
	}
	network := strings.TrimSpace(input.Network)
	if network == "" {
		network = "solana-mainnet"
	}
	classification := classifyRadarTarget(r.Context(), target)
	if !radarTargetTokenVerdictAllowed(classification) {
		statusCode := http.StatusUnprocessableEntity
		if classification.Type == radarTargetUnknown {
			statusCode = http.StatusServiceUnavailable
		}
		writeJSON(w, statusCode, map[string]any{
			"ok": false, "error": "token_mint_required", "charged": false,
			"target": target, "network": network, "target_classification": classification,
			"analysis_scope": classification.Type, "report_status": "insufficient_evidence",
			"message": radarTargetRejectionMessage(classification),
			"final_verdict": map[string]any{
				"grade": "-", "risk_index": nil, "risk_level": "unknown", "signed": false,
				"verdict":        "INSUFFICIENT EVIDENCE: token-mint verdict is not applicable to this target type.",
				"recommendation": classification.Type + "_intelligence_required",
			},
		})
		return
	}
	analysis := services.AnalyzeArvisRadars(services.SecurityRadarRequest{Target: target, Network: network, Mode: "owner_full_scan"})
	bundle := services.EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	arms := services.ArvisArmsFromBundle(bundle)
	if len(arms) == 0 {
		arms = analysis.Arms
	}
	if services.SecurityRadarHasLiveEvidence(bundle) {
		_ = h.saveSecurityRadarBundle(r.Context(), ownerChatIdentity(), "owner_full_scan", bundle)
	}
	freshFinal := services.ArvisFinalFromBundle(bundle)
	distribution := radarDetailHolderDistribution(r.Context(), target)
	sourceContext := h.radarDetailSourceContext(r.Context(), target, network)
	structural := h.radarDetailStructuralContext(r.Context(), target, network)
	persisted := h.radarDetailPersistedVerdict(r.Context(), target)
	final := radarDetailFinalMap(freshFinal, persisted)
	modules := radarDetailModules(arms)
	evidence := radarDetailEvidence(arms)
	warning := radarDetailWarning(final, distribution, structural, modules, sourceContext)
	graph := h.radarDetailGraph(r.Context(), target)
	detail := map[string]any{
		"ok": true, "schema_version": "koschei-owner-radar-v2", "target": target,
		"network": network, "generated_at": time.Now().UTC().Format(time.RFC3339),
		"target_classification": classification, "analysis_scope": radarTargetTokenMint,
		"final_verdict": final, "warning": warning, "holder_distribution": distribution,
		"structural_memory": structural, "source_context": sourceContext,
		"modules": modules, "evidence": evidence, "graph": graph,
		"holder_cluster": ownerRadarModuleSignal(modules, services.ModuleFundingClusterDetector, "holder_cluster_analysis"),
		"evidence_policy": map[string]any{
			"hide_verified_details": false, "no_evidence_no_claim": true,
			"creator_wallet_scope": "source-reported or on-chain relation; not proof of wrongdoing or real-world identity",
			"financial_advice":     false,
		},
	}
	detail["primary_risk_driver"] = ownerRadarPrimaryRiskDriver(modules)
	detail["narrative"] = ownerRadarNarrative(target, final, warning, distribution, sourceContext, modules)
	writeJSON(w, http.StatusOK, detail)
}

func ownerRadarModuleSignal(modules []map[string]any, moduleID, key string) any {
	for _, module := range modules {
		if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(module["module_id"])), moduleID) {
			continue
		}
		signals, _ := module["signals"].(map[string]any)
		if signals != nil {
			return signals[key]
		}
	}
	return nil
}

func ownerRadarNarrative(target string, final, warning, distribution, source map[string]any, modules []map[string]any) string {
	signed, _ := final["signed"].(bool)
	level := strings.ToLower(strings.TrimSpace(fmt.Sprint(final["risk_level"])))
	if !signed || final["risk_index"] == nil || level == "" || level == "unknown" || level == "<nil>" {
		return "Koschei bu hedef için doğrulanmış bir final risk puanı üretmedi. Kanıt eksikliği düşük risk anlamına gelmez; eksik modüller tamamlanana kadar sonuç EVIDENCE PENDING olarak değerlendirilmelidir."
	}

	risk := radarDetailNumber(final["risk_index"])
	parts := []string{
		fmt.Sprintf("Koschei bu tokenı (%s) %.0f/100 ile %s risk seviyesinde değerlendiriyor. %s", ownerRadarShortTarget(target), risk, ownerRadarRiskLabelTR(level), ownerRadarRiskMeaning(level)),
	}
	if sourceSignals, ok := source["signals"].(map[string]any); ok {
		volume := radarDetailNumber(sourceSignals["volume_24h_usd"])
		threshold := radarDetailNumber(sourceSignals["volume_threshold_usd"])
		if volume > 0 && threshold > 0 {
			parts = append(parts, fmt.Sprintf("Bu rapor otomatik Pump hacim radarı tarafından açıldı: toplam 24 saatlik işlem hacmi $%.0f ve eşik $%.0f. Hacim tek başına güvenlik veya dolandırıcılık hükmü değildir; yalnızca derin inceleme tetikleyicisidir.", volume, threshold))
		}
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

	parts = append(parts, ownerRadarPracticalConclusion(level, distribution))
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

func ownerRadarPracticalConclusion(level string, distribution map[string]any) string {
	available, _ := distribution["available"].(bool)
	if available {
		top1 := radarDetailNumber(distribution["top_1_percentage"])
		top10 := radarDetailNumber(distribution["top_10_percentage"])
		switch {
		case top1 >= 50:
			return "Pratik sonuç: tek bir risk taşıyan cüzdan dolaşımdaki arzın yarısından fazlasını kontrol ediyor. Bu cüzdanın satış, transfer, funding ve ortak-exit geçmişi çözülmeden token güvenli kabul edilmemelidir."
		case top1 >= 20 || top10 >= 75:
			return "Pratik sonuç: holder dağılımı merkezileşmiş durumda. Büyük cüzdanların satış kapasitesi, likidite çıkış yolları ve cluster bağlantıları işlem öncesinde doğrulanmalıdır."
		}
	}
	switch level {
	case "critical", "high":
		return "Pratik sonuç: işlem yapmadan önce ana risk sürücüsünün kanıtlarını, likidite çıkış yollarını, creator geçmişini ve bağlı cüzdan kümelerini doğrulamak gerekir."
	case "medium":
		return "Pratik sonuç: token otomatik olarak güvenli sayılmaz. Ana risk modülü, likidite davranışı, creator geçmişi ve Sybil/funding-cluster bağlantıları birlikte incelenmelidir."
	case "low":
		return "Pratik sonuç: mevcut taramada ağır bir alarm yok; yine de likidite, creator ve cüzdan kümeleri değişebileceği için karar güncel verilerle yenilenmelidir."
	default:
		return "Pratik sonuç: kanıt kapsamı tamamlanmadan kesin güvenlik yorumu yapılmamalıdır."
	}
}

// OwnerKOSCHAccess exposes current wallet verification and the latest cached
// KOSCH tier per account. Historical package/credit fields are intentionally
// absent from this contract.
func (h *Handler) OwnerKOSCHAccess(w http.ResponseWriter, r *http.Request) {
	db := h.DBRead
	if db == nil {
		db = h.DB
	}
	if db == nil || !ownerTableExists(r.Context(), db, "app_user_profiles") {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "users": []any{}, "summary": map[string]any{}})
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	walletJoin := `LEFT JOIN LATERAL (SELECT wallet_address, verified_at FROM verified_wallet_links v WHERE v.auth_subject=p.auth_subject AND v.status='active' ORDER BY verified_at DESC LIMIT 1) vw ON true`
	if !ownerTableExists(r.Context(), db, "verified_wallet_links") {
		walletJoin = `LEFT JOIN LATERAL (SELECT NULL::text wallet_address, NULL::timestamptz verified_at) vw ON true`
	}
	snapshotJoin := `LEFT JOIN LATERAL (SELECT tier, amount, amount_raw, checked_at, expires_at FROM token_access_snapshots s WHERE s.auth_subject=p.auth_subject ORDER BY checked_at DESC LIMIT 1) ts ON true`
	if !ownerTableExists(r.Context(), db, "token_access_snapshots") {
		snapshotJoin = `LEFT JOIN LATERAL (SELECT NULL::text tier, NULL::text amount, NULL::text amount_raw, NULL::timestamptz checked_at, NULL::timestamptz expires_at) ts ON true`
	}
	rows, err := db.QueryContext(r.Context(), `
		SELECT p.id::text, COALESCE(p.auth_subject,''), lower(p.email),
		       COALESCE(NULLIF(vw.wallet_address,''),NULLIF(p.wallet_address,''),''),
		       COALESCE(p.status,'active'), p.created_at,
		       vw.verified_at, COALESCE(ts.tier,'none'), COALESCE(ts.amount,'0'),
		       ts.checked_at, ts.expires_at
		FROM app_user_profiles p
		`+walletJoin+`
		`+snapshotJoin+`
		WHERE ($1='' OR lower(p.email) LIKE $2 OR lower(COALESCE(p.auth_subject,'')) LIKE $2 OR lower(COALESCE(vw.wallet_address,p.wallet_address,'')) LIKE $2)
		ORDER BY p.created_at DESC LIMIT 500`, q, "%"+q+"%")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "kosch_access_query_failed"})
		return
	}
	defer rows.Close()
	users := []map[string]any{}
	counts := map[string]int64{"total": 0, "verified": 0, "holders": 0, "basic": 0, "pro": 0, "enterprise": 0}
	for rows.Next() {
		var id, subject, email, wallet, status, tier, amount string
		var created time.Time
		var verifiedAt, checkedAt, expiresAt sql.NullTime
		if err := rows.Scan(&id, &subject, &email, &wallet, &status, &created, &verifiedAt, &tier, &amount, &checkedAt, &expiresAt); err != nil {
			continue
		}
		counts["total"]++
		if verifiedAt.Valid {
			counts["verified"]++
		}
		if tier == "basic" || tier == "pro" || tier == "enterprise" {
			counts["holders"]++
			counts[tier]++
		}
		item := map[string]any{"id": id, "auth_subject": subject, "email": email, "wallet_address": wallet, "status": status, "created_at": created, "wallet_verified": verifiedAt.Valid, "tier": tier, "amount": amount}
		if verifiedAt.Valid {
			item["verified_at"] = verifiedAt.Time
		}
		if checkedAt.Valid {
			item["checked_at"] = checkedAt.Time
		}
		if expiresAt.Valid {
			item["snapshot_expires_at"] = expiresAt.Time
		}
		users = append(users, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "users": users, "summary": counts,
		"mint_address": configuredKoscheiTokenMint(),
		"thresholds":   map[string]string{"basic": tokenTierThresholdEnv("KOSCHEI_TOKEN_TIER_BASIC", "0.000001"), "pro": tokenTierThresholdEnv("KOSCHEI_TOKEN_TIER_PRO", "250000"), "enterprise": tokenTierThresholdEnv("KOSCHEI_TOKEN_TIER_ENTERPRISE", "2000000")},
	})
}
