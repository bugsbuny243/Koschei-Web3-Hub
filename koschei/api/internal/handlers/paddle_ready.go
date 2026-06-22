package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
)

var paddleReadyState struct {
	sync.RWMutex
	db *sql.DB
}

func SetPaddleReadyDB(db *sql.DB) {
	paddleReadyState.Lock()
	paddleReadyState.db = db
	paddleReadyState.Unlock()
}

func paddleReadyDB() *sql.DB {
	paddleReadyState.RLock()
	defer paddleReadyState.RUnlock()
	return paddleReadyState.db
}

func PaddleWebhookReadyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/paddle/checkout", "/api/v1/paddle/checkout", "/api/v1/b2b/checkout":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			db := paddleReadyDB()
			if db == nil {
				writeAPIError(w, http.StatusServiceUnavailable, "PAYMENT_SCHEMA_UNAVAILABLE", "Paddle payment database unavailable")
				return
			}
			h := &Handler{DB: db, DBRead: db, Limiter: NewLimiter()}
			RequireAuth(h.CreateCheckoutReady)(w, r)
			return
		case "/api/paddle/webhook", "/api/v1/paddle/webhook":
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			db := paddleReadyDB()
			if db == nil {
				writeAPIError(w, http.StatusServiceUnavailable, "PAYMENT_SCHEMA_UNAVAILABLE", "Paddle payment database unavailable")
				return
			}
			h := &Handler{DB: db, DBRead: db, Limiter: NewLimiter()}
			h.HandlePaddleWebhookReady(w, r)
			return
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (h *Handler) CreateCheckoutReady(w http.ResponseWriter, r *http.Request) {
	if err := ensurePaddleIdempotencySchema(r.Context(), h.DB); err != nil {
		writePaymentAudit(r, h, "paddle_schema_unavailable", "error", map[string]any{"error": err.Error()})
		writeAPIError(w, http.StatusInternalServerError, "PAYMENT_SCHEMA_UNAVAILABLE", "Paddle payment schema unavailable")
		return
	}
	h.CreateCheckoutFinal(w, r)
}

func (h *Handler) HandlePaddleWebhookReady(w http.ResponseWriter, r *http.Request) {
	if err := ensurePaddleIdempotencySchema(r.Context(), h.DB); err != nil {
		writePaymentAudit(r, h, "paddle_schema_unavailable", "error", map[string]any{"error": err.Error()})
		writeAPIError(w, http.StatusInternalServerError, "PAYMENT_SCHEMA_UNAVAILABLE", "Paddle payment schema unavailable")
		return
	}
	h.HandlePaddleWebhookFinal(w, r)
}

func ensurePaddleIdempotencySchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database unavailable")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS paddle_webhook_events (
			event_id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL DEFAULT '',
			occurred_at TIMESTAMPTZ,
			status TEXT NOT NULL DEFAULT 'processing',
			attempts INTEGER NOT NULL DEFAULT 1,
			last_error TEXT NOT NULL DEFAULT '',
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			processed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS paddle_webhook_events_status_updated_idx ON paddle_webhook_events (status, updated_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS orders_provider_order_unique_idx ON orders (provider, provider_order_id) WHERE provider = 'paddle' AND provider_order_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS orders_provider_status_created_idx ON orders (provider, status, created_at DESC)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
