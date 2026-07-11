package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const ownerChatSystemPrompt = `You are Koschei Owner Copilot, the private operational assistant inside the Koschei Web3 owner panel.

Rules:
- Answer in Turkish unless the owner explicitly asks for another language.
- Speak naturally, directly and conversationally. Be concise, but explain important risks.
- Use the supplied operational snapshot and deterministic Radar result as the source of truth.
- Koschei uses a free-core + KOSCH-premium access model. Safe Check and basic scans are free; KOSCH unlocks full Radar depth, structural memory, graph, exposure, visual reports and automation.
- There is no live Paddle, Shopier, card-payment or package-sale flow. Never recommend restoring one unless the owner explicitly asks for a new architecture decision.
- Clearly distinguish verified facts, estimates and suggestions.
- Never claim that an action was executed unless the snapshot or deterministic result proves it.
- Never reveal, reconstruct or request API keys, private keys, tokens, passwords, database URLs or service-account secrets.
- This chat is operationally read-only. The separate owner Radar scanner may perform a read-only scan and persist its signed evidence record.
- Treat Koschei as production-grade Solana security and risk intelligence infrastructure, not a demo.
- Auth is frozen and must not be changed unless the owner explicitly removes that restriction.
- A creator/deployer wallet, holder concentration or linked-wallet signal is evidence of an on-chain relation, not proof of fraud or a real-world identity.
- When data is unavailable, say so instead of inventing it.
- Do not output raw JSON unless the owner asks for it. Summarize data in human language.`

type ownerChatSnapshot struct {
	GeneratedAt string         `json:"generated_at"`
	Services    map[string]any `json:"services"`
	Business    map[string]any `json:"business"`
	Access      map[string]any `json:"access"`
	Radar       map[string]any `json:"radar"`
}

func (h *Handler) buildOwnerChatSnapshot(ctx context.Context) ownerChatSnapshot {
	snapshot := ownerChatSnapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Services: map[string]any{
			"database":        ownerDatabaseStatus(ctx, h.DB),
			"ai_provider":     ownerAIProviderStatus(),
			"neon_auth":       configuredStatus("NEON_AUTH_JWKS_URL"),
			"solana_rpc":      configuredStatusAny("SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL", "ALCHEMY_API_KEY"),
			"kosch_access":    serviceStatus(configuredKoscheiTokenGateEnabled() && configuredKoscheiTokenMint() != "", "configured", "missing"),
			"visual_renderer": "client_canvas_png_ready",
		},
		Business: map[string]any{},
		Access: map[string]any{
			"model": "free_core_kosch_premium",
			"free_core": []string{"safe_check", "basic_token_scan"},
			"premium": []string{"full_radar", "structural_memory", "graph", "exposure", "visual_reports", "automation"},
			"payment_providers": []string{},
			"kosch_mint": configuredKoscheiTokenMint(),
		},
		Radar: map[string]any{},
	}

	if ownerTableExists(ctx, h.DB, "app_user_profiles") {
		snapshot.Business["total_users"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM app_user_profiles`)
		snapshot.Business["active_users"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM app_user_profiles WHERE COALESCE(status,'active')='active'`)
	}
	if ownerTableExists(ctx, h.DB, "verified_wallet_links") {
		snapshot.Access["verified_wallets"] = ownerCount(ctx, h.DB, `SELECT count(DISTINCT auth_subject) FROM verified_wallet_links WHERE status='active'`)
	}
	if ownerTableExists(ctx, h.DB, "token_access_snapshots") {
		latest := `WITH latest AS (
			SELECT DISTINCT ON (auth_subject) auth_subject,tier
			FROM token_access_snapshots
			WHERE expires_at > now()
			ORDER BY auth_subject,checked_at DESC
		)`
		snapshot.Access["kosch_holders"] = ownerCount(ctx, h.DB, latest+` SELECT count(*) FROM latest WHERE tier IN ('basic','pro','enterprise')`)
		snapshot.Access["basic"] = ownerCount(ctx, h.DB, latest+` SELECT count(*) FROM latest WHERE tier='basic'`)
		snapshot.Access["pro"] = ownerCount(ctx, h.DB, latest+` SELECT count(*) FROM latest WHERE tier='pro'`)
		snapshot.Access["enterprise"] = ownerCount(ctx, h.DB, latest+` SELECT count(*) FROM latest WHERE tier='enterprise'`)
	}
	if ownerTableExists(ctx, h.DB, "customer_feedback") {
		snapshot.Business["open_feedback"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM customer_feedback WHERE status IN ('new','reviewing','planned')`)
		snapshot.Business["security_feedback"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM customer_feedback WHERE category='security' AND status IN ('new','reviewing')`)
	}
	if ownerTableExists(ctx, h.DB, "security_audit_events") {
		snapshot.Business["security_events_24h"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM security_audit_events WHERE created_at>=now()-interval '24 hours'`)
		snapshot.Business["critical_security_events_24h"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM security_audit_events WHERE created_at>=now()-interval '24 hours' AND lower(COALESCE(severity,'')) IN ('critical','fatal','high','error')`)
	}
	if ownerTableExists(ctx, h.DB, "arvis_stream_processing") {
		snapshot.Radar["completed"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM arvis_stream_processing WHERE status='completed'`)
		snapshot.Radar["processing"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM arvis_stream_processing WHERE status='processing'`)
		snapshot.Radar["retryable"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM arvis_stream_processing WHERE status='failed' AND attempts<3`)
		snapshot.Radar["exhausted"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM arvis_stream_processing WHERE status='exhausted' OR (status='failed' AND attempts>=3)`)
	}
	if ownerTableExists(ctx, h.DB, "security_radar_events") {
		snapshot.Radar["events_total"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM security_radar_events`)
		snapshot.Radar["latest_pump_event"] = ownerTimestamp(ctx, h.DB, `SELECT max(created_at) FROM security_radar_events WHERE module_id='pump_sybil_radar'`)
		snapshot.Radar["latest_raydium_event"] = ownerTimestamp(ctx, h.DB, `SELECT max(created_at) FROM security_radar_events WHERE module_id='raydium_pool_guardian'`)
	}
	if ownerTableExists(ctx, h.DB, "security_radar_verdicts") {
		snapshot.Radar["verdicts_total"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM security_radar_verdicts`)
		snapshot.Radar["final_verdicts_24h"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND created_at>=now()-interval '24 hours'`)
		snapshot.Radar["high_risk_24h"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND signed=true AND created_at>=now()-interval '24 hours' AND lower(COALESCE(risk_level,'')) IN ('high','critical')`)
		snapshot.Radar["latest_final_verdict"] = ownerTimestamp(ctx, h.DB, `SELECT max(created_at) FROM security_radar_verdicts WHERE module_id='final_verdict_engine'`)
	}
	return snapshot
}

func ownerDatabaseStatus(ctx context.Context, db *sql.DB) string {
	if db == nil {
		return "missing"
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := db.PingContext(checkCtx); err != nil {
		return "error"
	}
	return "connected"
}

func ownerAIProviderStatus() map[string]any {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
	if provider == "" {
		provider = "router"
	}
	model := firstNonEmpty(
		strings.TrimSpace(os.Getenv("OWNER_CHAT_MODEL")),
		strings.TrimSpace(os.Getenv("TOGETHER_MODEL")),
		strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
		"router-default",
	)
	return map[string]any{"configured": aiProviderConfigured(), "provider": provider, "model": model}
}

func ownerTimestamp(ctx context.Context, db *sql.DB, query string) string {
	var value sql.NullTime
	if db == nil {
		return ""
	}
	if err := db.QueryRowContext(ctx, query).Scan(&value); err != nil || !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func buildOwnerChatPrompt(snapshot ownerChatSnapshot, messages []ownerChatMessage, deterministic map[string]any) string {
	snapshotJSON, _ := json.Marshal(snapshot)
	var out strings.Builder
	out.WriteString("CURRENT OPERATIONAL SNAPSHOT:\n")
	out.Write(snapshotJSON)
	out.WriteString("\n\n")
	if deterministic != nil {
		resultJSON, _ := json.Marshal(deterministic)
		out.WriteString("DETERMINISTIC READ-ONLY RESULT FOR THE LATEST QUESTION:\n")
		out.Write(resultJSON)
		out.WriteString("\n\n")
	}
	out.WriteString("CONVERSATION HISTORY:\n")
	for _, message := range messages {
		role := strings.ToUpper(strings.TrimSpace(message.Role))
		if role != "USER" && role != "ASSISTANT" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		out.WriteString(role)
		out.WriteString(": ")
		out.WriteString(content)
		out.WriteString("\n")
	}
	out.WriteString("\nRespond to the latest USER message. Do not repeat the snapshot verbatim.")
	return out.String()
}

func ownerChatTitle(message string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	if message == "" {
		return "Yeni sohbet"
	}
	runes := []rune(message)
	if len(runes) > 54 {
		message = string(runes[:54]) + "…"
	}
	return message
}

func ownerChatIdentity() string {
	owner := normalizeWallet(firstEnv("OWNER_WALLET", "KOSCHEI_OWNER_WALLET"))
	if owner == "" {
		owner = "control-center"
	}
	return "owner:" + owner
}

func ownerChatModel() string {
	return firstNonEmpty(strings.TrimSpace(os.Getenv("OWNER_CHAT_MODEL")), strings.TrimSpace(os.Getenv("TOGETHER_MODEL")), strings.TrimSpace(os.Getenv("OPENAI_MODEL")))
}

func ownerChatGenerationError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("Owner AI yanıtı üretilemedi: %s", shortError(err.Error()))
}
