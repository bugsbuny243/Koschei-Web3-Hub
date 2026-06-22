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
- Use the supplied operational snapshot as the source of truth for project status.
- Clearly distinguish verified facts, estimates and suggestions.
- Never claim that an action was executed unless the snapshot or deterministic tool result explicitly proves it.
- Never reveal, reconstruct or request API keys, private keys, tokens, passwords, webhook secrets, database URLs or service-account private keys.
- This chat is read-only. Do not perform user bans, payments, entitlement changes, deploys, database writes or production releases.
- When the owner asks for a write action, explain the intended action and tell them to use the relevant owner control or explicitly request an approved implementation workflow.
- Treat Koschei as production-grade Solana security and risk intelligence infrastructure, not a demo.
- Auth is frozen and must not be changed unless the owner explicitly removes that restriction.
- When data is unavailable, say so instead of inventing it.
- Do not output raw JSON unless the owner asks for it. Summarize data in human language.`

type ownerChatSnapshot struct {
	GeneratedAt string         `json:"generated_at"`
	Services    map[string]any `json:"services"`
	Business    map[string]any `json:"business"`
	Radar       map[string]any `json:"radar"`
	GooglePlay  map[string]any `json:"google_play"`
}

func (h *Handler) buildOwnerChatSnapshot(ctx context.Context) ownerChatSnapshot {
	snapshot := ownerChatSnapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Services: map[string]any{
			"database":     ownerDatabaseStatus(ctx, h.DB),
			"ai_provider":  ownerAIProviderStatus(),
			"paddle":       configuredStatus("PADDLE_API_KEY", "PADDLE_WEBHOOK_SECRET", "PADDLE_ENV"),
			"alchemy_rpc":  configuredStatusAny("ALCHEMY_API_KEY", "SOLANA_RPC_URL"),
			"neon":         configuredStatus("DATABASE_URL"),
			"google_play":  configuredStatusAny("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON", "GOOGLE_APPLICATION_CREDENTIALS_JSON", "GOOGLE_APPLICATION_CREDENTIALS"),
		},
		Business: map[string]any{},
		Radar:    map[string]any{},
		GooglePlay: googlePlayReadiness(),
	}

	if ownerTableExists(ctx, h.DB, "app_user_profiles") {
		snapshot.Business["total_users"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM app_user_profiles`)
	}
	if ownerTableExists(ctx, h.DB, "entitlements") {
		snapshot.Business["active_entitlements"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM entitlements WHERE status='active'`)
	}
	if ownerTableExists(ctx, h.DB, "payment_requests") {
		snapshot.Business["pending_payments"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM payment_requests WHERE status='pending'`)
		snapshot.Business["approved_payments_30d"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM payment_requests WHERE status='approved' AND COALESCE(reviewed_at,created_at) >= now()-interval '30 days'`)
	}
	if ownerTableExists(ctx, h.DB, "orders") {
		snapshot.Business["paddle_orders_total"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM orders WHERE provider='paddle'`)
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
		snapshot.Radar["final_verdicts_24h"] = ownerCount(ctx, h.DB, `SELECT count(*) FROM security_radar_verdicts WHERE module_id='final_verdict_engine' AND created_at>=now()-interval '24 hours'`)
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
	return map[string]any{
		"configured": aiProviderConfigured(),
		"provider":   provider,
		"model":      model,
	}
}

func ownerCount(ctx context.Context, db *sql.DB, query string) int64 {
	var value int64
	if db == nil {
		return 0
	}
	_ = db.QueryRowContext(ctx, query).Scan(&value)
	return value
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
	return firstNonEmpty(
		strings.TrimSpace(os.Getenv("OWNER_CHAT_MODEL")),
		strings.TrimSpace(os.Getenv("TOGETHER_MODEL")),
		strings.TrimSpace(os.Getenv("OPENAI_MODEL")),
	)
}

func ownerChatGenerationError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("Owner AI yanıtı üretilemedi: %s", shortError(err.Error()))
}
