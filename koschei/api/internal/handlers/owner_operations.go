package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
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
	servicesMap := map[string]any{
		"database":        map[string]any{"status": serviceStatus(db != nil, "connected", "unavailable")},
		"neon_auth":       map[string]any{"status": serviceStatus(envSet("NEON_AUTH_JWKS_URL"), "configured", "missing")},
		"solana_rpc":      map[string]any{"status": serviceStatus(envSet("SOLANA_RPC_URL") || envSet("ALCHEMY_SOLANA_RPC_URL") || envSet("HELIUS_SOLANA_RPC_URL") || envSet("QUICKNODE_SOLANA_RPC_URL") || envSet("ALCHEMY_API_KEY"), "configured", "missing")},
		"security_radar":  map[string]any{"status": firstMapString(radar, "pipeline_status")},
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
	if db != nil {
		store := services.NewSecurityRadarStore(db)
		if loaded, err := store.LatestVerdicts(r.Context(), 100); err == nil {
			items = loaded
		}
		if loaded, err := store.ListSources(r.Context()); err == nil {
			sources = loaded
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "generated_at": time.Now().UTC(), "items": items,
		"sources": sources, "pipeline": h.securityRadarStreamStats(r.Context()),
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
		"evidence_policy": map[string]any{
			"hide_verified_details": false, "no_evidence_no_claim": true,
			"creator_wallet_scope": "source-reported or on-chain relation; not proof of wrongdoing or real-world identity",
			"financial_advice":     false,
		},
	}
	detail["narrative"] = ownerRadarNarrative(target, final, warning, distribution, sourceContext)
	writeJSON(w, http.StatusOK, detail)
}

func ownerRadarNarrative(target string, final, warning, distribution, source map[string]any) string {
	risk := fmt.Sprint(final["risk_index"])
	level := strings.ToUpper(strings.TrimSpace(fmt.Sprint(final["risk_level"])))
	if level == "" || level == "<NIL>" {
		level = "UNKNOWN"
	}
	parts := []string{fmt.Sprintf("%s için ARVIS kararı: %s, risk %s/100.", target, level, risk)}
	if available, _ := distribution["available"].(bool); available {
		parts = append(parts, fmt.Sprintf("Holder yoğunluğu: Top 1 %v%%, Top 10 %v%%, Top 20 %v%%.", distribution["top_1_percentage"], distribution["top_10_percentage"], distribution["top_20_percentage"]))
		if adjusted, _ := distribution["role_adjusted"].(bool); adjusted {
			parts = append(parts, fmt.Sprintf("Ham arz yoğunluğu rol sınıflandırmasıyla düzeltildi: protokol/bonding-curve envanteri %v%%; baskın rol %v.", distribution["protocol_controlled_percentage"], distribution["dominant_role"]))
		}
		if blocked, _ := distribution["blocking_evidence_gap"].(bool); blocked {
			parts = append(parts, "Baskın holder rolü çözülmediği için final yoğunlaşma kararı bekletildi; veri yokluğu düşük risk sayılmadı.")
		}
	}
	if creator := strings.TrimSpace(fmt.Sprint(source["creator_wallet"])); creator != "" && creator != "<nil>" {
		parts = append(parts, "Kaynakta creator/deployer ilişkisi görülen cüzdan: "+creator+".")
	}
	if headline := strings.TrimSpace(fmt.Sprint(warning["headline"])); headline != "" && headline != "<nil>" {
		parts = append(parts, headline)
	}
	parts = append(parts, "Bu değerlendirme kanıt kapsamındadır; kötü niyet veya gerçek kişi kimliği iddiası değildir.")
	return strings.Join(parts, " ")
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
