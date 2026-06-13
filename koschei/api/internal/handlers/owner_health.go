package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"koschei/api/internal/router"
	"koschei/api/internal/web3"
)

type ownerBrainInput struct {
	Message string `json:"message"`
}

type backendHealthCheck struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	LatencyMS int64     `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
}

func (h *Handler) OwnerBackendHealth(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	checks := []backendHealthCheck{
		h.checkDatabase(ctx),
		h.checkOpenAI(ctx),
		h.checkPaddle(ctx),
		h.checkAlchemy(ctx),
		h.checkGitHub(ctx),
		h.checkNeon(ctx),
	}
	overall := "ok"
	for _, check := range checks {
		if check.Status == "error" {
			overall = "degraded"
			break
		}
		if check.Status == "unavailable" && overall == "ok" {
			overall = "partial"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": overall, "checks": checks, "checked_at": time.Now().UTC()})
}

func (h *Handler) OwnerBrain(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	var req ownerBrainInput
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Message) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message required"})
		return
	}
	health := map[string]any{}
	status := map[string]any{}
	if h.DB != nil {
		_ = h.DB.QueryRowContext(r.Context(), `SELECT json_build_object('users',(SELECT count(*) FROM app_user_profiles),'active_entitlements',(SELECT count(*) FROM entitlements WHERE status='active' AND COALESCE(plan_id,'free') <> 'free'),'pending_payments',(SELECT count(*) FROM payment_requests WHERE status='pending'))::text`).Scan(new(string))
	}
	checks := []backendHealthCheck{h.checkDatabase(r.Context()), h.checkOpenAI(r.Context()), h.checkPaddle(r.Context()), h.checkAlchemy(r.Context()), h.checkGitHub(r.Context()), h.checkNeon(r.Context())}
	health["checks"] = checks
	status["safety"] = "Owner brain is read-only by default. Use explicit emergency endpoints for controls."
	payload, _ := json.Marshal(map[string]any{"health": health, "status": status})
	ai, err := router.Chat(r.Context(), router.ChatRequest{
		System:      "You are Koschei Central Brain for the owner-only /owner panel. Never reveal secrets. Summarize operations, health, payments, database, GitHub, Neon, OpenAI, Paddle, Alchemy, errors, and safe emergency actions. If data is unavailable, say Real data unavailable.",
		Prompt:      fmt.Sprintf("Owner asks: %s\n\nLive backend context JSON: %s", strings.TrimSpace(req.Message), string(payload)),
		MaxTokens:   700,
		Temperature: 0.2,
		Timeout:     25 * time.Second,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "answer": "Real data unavailable.", "provider": "unavailable", "error": shortError(err.Error()), "health": health})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "answer": ai.Content, "provider": ai.Provider, "model": ai.Model, "health": health})
}

func (h *Handler) OwnerEmergencyControl(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	var req struct {
		Control string `json:"control"`
		Enabled bool   `json:"enabled"`
		Reason  string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Control) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "control required"})
		return
	}
	allowed := map[string]string{"maintenance": "KOSCHEI_MAINTENANCE_MODE", "sales_pause": "KOSCHEI_SALES_PAUSED", "premium_pause": "KOSCHEI_PREMIUM_PAUSED"}
	envKey := allowed[strings.TrimSpace(req.Control)]
	if envKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown control"})
		return
	}
	value := strings.EqualFold(strings.TrimSpace(os.Getenv(envKey)), "true")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "control": req.Control, "requested_enabled": req.Enabled, "effective_enabled": value, "message": "Emergency controls are env-backed for production safety. Update deployment environment to change effective state."})
}

func (h *Handler) checkDatabase(ctx context.Context) backendHealthCheck {
	return timedCheck("Database", func(ctx context.Context) (string, string) {
		if err := h.dbAvailable(ctx); err != nil {
			return "error", shortError(err.Error())
		}
		return "ok", "Database ping succeeded."
	})
}

func (h *Handler) checkOpenAI(ctx context.Context) backendHealthCheck {
	return timedCheck("OpenAI", func(ctx context.Context) (string, string) {
		if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
			return "unavailable", "OPENAI_API_KEY is not configured."
		}
		return httpProviderCheck(ctx, http.MethodGet, "https://api.openai.com/v1/models", "Bearer "+os.Getenv("OPENAI_API_KEY"), nil)
	})
}

func (h *Handler) checkPaddle(ctx context.Context) backendHealthCheck {
	return timedCheck("Paddle", func(ctx context.Context) (string, string) {
		key := firstEnv("PADDLE_API_KEY", "PADDLE_VENDOR_AUTH_CODE")
		if key == "" {
			return "unavailable", "PADDLE_API_KEY is not configured."
		}
		base := "https://api.paddle.com"
		if strings.EqualFold(os.Getenv("PADDLE_ENV"), "sandbox") || strings.Contains(strings.ToLower(os.Getenv("PADDLE_API_BASE_URL")), "sandbox") {
			base = "https://sandbox-api.paddle.com"
		}
		if configured := strings.TrimRight(os.Getenv("PADDLE_API_BASE_URL"), "/"); configured != "" {
			base = configured
		}
		return httpProviderCheck(ctx, http.MethodGet, base+"/transactions?per_page=1", "Bearer "+key, nil)
	})
}

func (h *Handler) checkAlchemy(ctx context.Context) backendHealthCheck {
	return timedCheck("Alchemy/RPC", func(ctx context.Context) (string, string) {
		if strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")) == "" && strings.TrimSpace(os.Getenv("SOLANA_RPC_URL")) == "" {
			return "unavailable", "ALCHEMY_API_KEY or SOLANA_RPC_URL is not configured."
		}
		var out string
		rpc := h.SolanaRPC
		if rpc == nil {
			rpc = web3.NewSolanaRPC(nil)
		}
		if err := rpc.Call(ctx, "solana-mainnet", "getHealth", []any{}, &out, 5*time.Second); err != nil {
			return "error", shortError(err.Error())
		}
		return "ok", "Solana RPC getHealth returned " + out
	})
}

func (h *Handler) checkGitHub(ctx context.Context) backendHealthCheck {
	return timedCheck("GitHub", func(ctx context.Context) (string, string) {
		repo := strings.TrimSpace(os.Getenv("GITHUB_REPO"))
		if repo == "" {
			return "unavailable", "GITHUB_REPO is not configured."
		}
		url := "https://api.github.com/repos/" + strings.Trim(repo, "/")
		auth := ""
		if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
			auth = "Bearer " + token
		}
		return httpProviderCheck(ctx, http.MethodGet, url, auth, nil)
	})
}

func (h *Handler) checkNeon(ctx context.Context) backendHealthCheck {
	return timedCheck("Neon", func(ctx context.Context) (string, string) {
		if strings.TrimSpace(os.Getenv("NEON_API_KEY")) != "" {
			return httpProviderCheck(ctx, http.MethodGet, "https://console.neon.tech/api/v2/projects?limit=1", "Bearer "+os.Getenv("NEON_API_KEY"), nil)
		}
		if strings.TrimSpace(ConfiguredNeonAuthJWKSURL()) != "" {
			return httpProviderCheck(ctx, http.MethodGet, ConfiguredNeonAuthJWKSURL(), "", nil)
		}
		return "unavailable", "NEON_API_KEY or NEON_AUTH_JWKS_URL is not configured."
	})
}

func timedCheck(name string, fn func(context.Context) (string, string)) backendHealthCheck {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	status, message := fn(ctx)
	return backendHealthCheck{Name: name, Status: status, Message: message, LatencyMS: time.Since(started).Milliseconds(), CheckedAt: time.Now().UTC()}
}

func httpProviderCheck(ctx context.Context, method, url, auth string, body any) (string, string) {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return "error", shortError(err.Error())
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error", shortError(err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "ok", fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return "error", fmt.Sprintf("HTTP %d", resp.StatusCode)
}
