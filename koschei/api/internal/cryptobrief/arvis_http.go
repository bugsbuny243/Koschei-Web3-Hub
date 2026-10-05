package cryptobrief

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

// ARVISTelegramHTTP exposes only the account-to-Telegram delivery state used by
// ARVIS. It deliberately has no news topics, cadence or RSS preferences.
func (s *Service) ARVISTelegramHTTP(subject func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customer := strings.TrimSpace(subject(r))
		if customer == "" {
			respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			state, mask, err := s.telegramSubscriptionState(r, customer)
			if err != nil {
				respond(w, http.StatusServiceUnavailable, map[string]string{"error": "telegram_state_unavailable"})
				return
			}
			respond(w, http.StatusOK, map[string]any{
				"schema_version": "koschei.arvis-telegram.v1",
				"configured":     s.Config.Ready("telegram"),
				"state":          state,
				"recipient_mask": mask,
				"purpose":        "arvis_scan_result_delivery",
			})
		case http.MethodPatch, http.MethodDelete:
			state := "disconnected"
			if r.Method == http.MethodPatch {
				var body struct {
					State string `json:"state"`
				}
				if !decode(w, r, &body) {
					return
				}
				state = strings.ToLower(strings.TrimSpace(body.State))
				if state != "active" && state != "paused" {
					respond(w, http.StatusBadRequest, map[string]string{"error": "state_invalid"})
					return
				}
			}
			if err := s.ChangeState(r.Context(), customer, "telegram", state); err != nil {
				respond(w, http.StatusConflict, map[string]string{"error": "telegram_state_change_failed"})
				return
			}
			respond(w, http.StatusOK, map[string]any{"saved": true, "state": state})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func (s *Service) ARVISTelegramPairHTTP(subject func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		customer := strings.TrimSpace(subject(r))
		if customer == "" {
			respond(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		var body struct {
			Consent bool `json:"consent"`
		}
		if !decode(w, r, &body) {
			return
		}
		if !body.Consent {
			respond(w, http.StatusBadRequest, map[string]string{"error": "explicit_consent_required"})
			return
		}
		pairing, err := s.Pair(r.Context(), customer, "telegram")
		if err != nil {
			status := http.StatusConflict
			code := "telegram_pairing_failed"
			if errors.Is(err, ErrChannelUnavailable) {
				status = http.StatusServiceUnavailable
				code = "telegram_not_configured"
			} else if errors.Is(err, ErrPairRate) {
				status = http.StatusTooManyRequests
				code = "pairing_rate_limited"
			}
			respond(w, status, map[string]string{"error": code})
			return
		}
		respond(w, http.StatusOK, map[string]any{
			"schema_version": "koschei.arvis-telegram.v1",
			"url":            pairing.URL,
			"command":        pairing.Command,
			"expires_at":     pairing.ExpiresAt,
		})
	}
}

func (s *Service) ARVISTelegramStatusHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var active, paused, pending, accepted, uncertain, failed int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT
		count(*) FILTER (WHERE state='active'),
		count(*) FILTER (WHERE state='paused')
		FROM crypto_brief_subscriptions WHERE channel='telegram'`).Scan(&active, &paused); err != nil {
		respond(w, http.StatusServiceUnavailable, map[string]string{"error": "telegram_status_unavailable"})
		return
	}
	if err := s.DB.QueryRowContext(r.Context(), `SELECT
		count(*) FILTER (WHERE d.state='pending'),
		count(*) FILTER (WHERE d.state='accepted'),
		count(*) FILTER (WHERE d.state='uncertain'),
		count(*) FILTER (WHERE d.state='failed')
		FROM crypto_brief_deliveries d
		JOIN crypto_brief_subscriptions s ON s.id=d.subscription_id
		WHERE s.channel='telegram' AND d.dedup_key LIKE 'arvis:%'`).Scan(&pending, &accepted, &uncertain, &failed); err != nil {
		respond(w, http.StatusServiceUnavailable, map[string]string{"error": "telegram_status_unavailable"})
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"schema_version":      "koschei.arvis-telegram.v1",
		"configured":          s.Config.Ready("telegram"),
		"webhook_verified":    s.telegramVerified.Load(),
		"active_customers":    active,
		"paused_customers":    paused,
		"pending_deliveries":  pending,
		"accepted_deliveries": accepted,
		"uncertain_deliveries": uncertain,
		"failed_deliveries":   failed,
		"purpose":             "arvis_scan_result_delivery",
	})
}

func (s *Service) telegramSubscriptionState(r *http.Request, customer string) (string, string, error) {
	state := "disconnected"
	var recipient sql.NullString
	err := s.DB.QueryRowContext(r.Context(), `SELECT state,recipient FROM crypto_brief_subscriptions WHERE customer_sub=$1 AND channel='telegram'`, customer).Scan(&state, &recipient)
	if errors.Is(err, sql.ErrNoRows) {
		return state, "", nil
	}
	if err != nil {
		return "", "", err
	}
	mask := ""
	if recipient.Valid {
		runes := []rune(recipient.String)
		if len(runes) > 4 {
			mask = "••••" + string(runes[len(runes)-4:])
		} else {
			mask = "••••"
		}
	}
	return state, mask, nil
}
