package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"koschei/api/internal/services"
)

// ownerPolarBillingStatus tells the owner whether Professional can be sold and
// whether verified Polar webhooks are reaching the entitlement ledger. It
// reads configuration presence and ledger metadata only, never secret values.
func ownerPolarBillingStatus(ctx context.Context, ledger *sql.DB) map[string]any {
	cfg := services.LoadPolarConfigFromEnv()
	configured := cfg.CheckoutConfigured("professional") && cfg.WebhookConfigured()
	checkoutEnabled := commercialCheckoutEnabled()
	out := map[string]any{
		"provider":                 "polar",
		"environment":              cfg.Environment,
		"configured":               configured,
		"checkout_enabled":         checkoutEnabled,
		"webhook_events_total":     nil,
		"webhook_last_event_type":  nil,
		"webhook_last_received_at": nil,
	}

	ledgerReadable := false
	var total int64
	var lastType sql.NullString
	var lastAt sql.NullTime
	if ledger != nil {
		err := ledger.QueryRowContext(ctx, `
			SELECT count(*), (array_agg(event_type ORDER BY processed_at DESC))[1], max(processed_at)
			FROM billing_provider_events
			WHERE provider='polar'`).Scan(&total, &lastType, &lastAt)
		ledgerReadable = err == nil
	}
	if ledgerReadable {
		out["webhook_events_total"] = total
		if lastAt.Valid {
			out["webhook_last_event_type"] = lastType.String
			out["webhook_last_received_at"] = lastAt.Time.UTC()
		}
	}
	out["status"], out["verification"] = polarBillingState(configured, checkoutEnabled, ledgerReadable, total, lastType.String, lastAt.Time)
	return out
}

// polarBillingState orders the blockers the way a purchase meets them:
// configuration, the commercial checkout gate, the ledger, then delivery.
func polarBillingState(configured, checkoutEnabled, ledgerReadable bool, events int64, lastType string, lastAt time.Time) (string, string) {
	switch {
	case !configured:
		return "incomplete", "Polar token, webhook secret or Professional product missing"
	case !checkoutEnabled:
		return "checkout_paused", "commercial checkout gate is off"
	case !ledgerReadable:
		return "ledger_unavailable", "billing event ledger could not be read"
	case events == 0:
		return "awaiting_first_webhook", "no verified Polar webhook recorded yet"
	default:
		return "live", fmt.Sprintf("last verified webhook %s · %s", lastType, lastAt.UTC().Format(time.RFC3339))
	}
}
