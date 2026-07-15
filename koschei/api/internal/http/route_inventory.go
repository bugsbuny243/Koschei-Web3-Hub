package http

import (
	"encoding/json"
	"net/http"
	"time"
)

type routeInventoryGroup struct {
	Name   string   `json:"name"`
	Auth   string   `json:"auth"`
	Routes []string `json:"routes"`
}

func ownerRouteMap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": true, "generated_at": time.Now().UTC().Format(time.RFC3339),
		"source": "server_boot_chain", "access_model": "public_free_core_plus_kosch_tiers_and_daily_quota",
		"groups": productionRouteInventory(),
		"rules": []string{
			"A handler is live only when registered in the server boot chain.",
			"Public Safe Check and basic token fundamentals remain outside auth, KOSCH tier and quota middleware.",
			"KOSCH holdings grant eligibility; Basic, Pro and Enterprise determine route access and daily UTC quota.",
			"Basic defaults to 25,000 KOSCH / 5 daily calls, Pro to 250,000 / 100, Enterprise to 2,000,000 / 1000; every value is env-overridable.",
			"Developer API keys remain identity credentials and require live Enterprise KOSCH verification.",
			"Failed metered requests refund their transactional quota reservation.",
			"Legacy Shopier, Paddle, package purchase and owner payment routes are not registered.",
			"Evidence-backed verdicts use deterministic letter rules and never a numeric risk score.",
			"Recipient fate investigation is mint-specific ATA-only and never queries recipient-wide signature history.",
			"The owner unified Radar is manual-only and owner-session protected; KOSCH tiers do not gate owner routes.",
		},
	})
}

func productionRouteInventory() []routeInventoryGroup {
	return []routeInventoryGroup{
		{Name: "free_core", Auth: "public_unmetered", Routes: []string{
			"POST /api/arvis/preflight", "POST /api/token/scan", "GET /api/v1/risk/badge",
			"GET /api/public/impact", "GET /api/public/metrics", "GET /api/web3/health",
		}},
		{Name: "identity", Auth: "mixed", Routes: []string{
			"GET /health", "GET /api/config", "POST /api/auth/register", "POST /api/auth/login", "GET /api/me",
			"GET /api/web3/health/logs", "POST /api/analytics/event",
		}},
		{Name: "enterprise_account", Auth: "customer_session_plus_enterprise_kosch", Routes: []string{
			"GET /api/account/api-keys", "POST /api/account/api-keys", "POST /api/account/api-keys/{id}/revoke",
		}},
		{Name: "wallet_and_access_status", Auth: "customer_session", Routes: []string{
			"POST /api/auth/wallet/challenge", "POST /api/auth/wallet/verify", "GET /api/auth/wallet/status",
			"POST /api/auth/wallet/unlink", "GET /api/auth/token-access", "GET /api/auth/premium-access",
		}},
		{Name: "owner", Auth: "owner_session_no_kosch_gate", Routes: []string{
			"POST /api/owner/login", "POST /api/owner/logout", "GET /api/owner/command-center", "GET /api/owner/route-map",
			"POST /api/owner/radar/unified", "GET /api/owner/defense/tracks", "POST /api/owner/defense/investigate", "POST /api/owner/defense/distribution",
			"GET /api/owner/users", "POST /api/owner/users/ban", "POST /api/owner/users/remove", "POST /api/owner/command",
			"POST /api/owner/brain", "/api/owner/chat", "GET /api/owner/health", "GET /api/owner/status",
		}},
		{Name: "basic_metered", Auth: "customer_session_plus_basic_kosch_daily_quota", Routes: []string{
			"POST /api/v1/token/extensions", "POST /api/v1/address-poisoning/check", "POST /api/v1/radar/check", "GET /api/v1/radar/detail",
		}},
		{Name: "pro_metered", Auth: "customer_session_plus_pro_kosch_daily_quota", Routes: []string{
			"GET /api/v1/radar/feed", "GET /api/v1/radar/creator-intelligence", "GET /api/v1/radar/actor-intelligence",
			"GET /api/v1/radar/graph", "GET /api/v1/radar/exposure", "/api/watchlist", "POST /api/watchlist/refresh", "/api/watchlist/alerts", "/api/watchlist/{id}",
		}},
		{Name: "enterprise_developer_api", Auth: "api_key_plus_enterprise_kosch_daily_quota", Routes: []string{
			"POST /api/v1/scan/token", "GET /api/v1/usage", "POST /api/v1/shield/preflight",
			"POST /api/v1/shield/transaction", "POST /api/v1/shield/address-poisoning",
		}},
		{Name: "enterprise_webhooks", Auth: "customer_session_plus_enterprise_kosch", Routes: []string{
			"/api/webhooks", "/api/webhooks/{id}", "/api/webhooks/deliveries", "/api/webhooks/deliveries/{id}",
		}},
	}
}
