package cryptobrief

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		respond(w, 400, map[string]string{"error": "invalid_request"})
		return false
	}
	return true
}
func (s *Service) FeedHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	items, e := s.Recent(r.Context(), nil, nil)
	if e != nil {
		respond(w, 503, map[string]string{"error": "news_feed_unavailable"})
		return
	}
	status, e := s.Status(r.Context())
	if e != nil {
		respond(w, 503, map[string]string{"error": "news_feed_unavailable"})
		return
	}
	// Public source freshness contains no customer/delivery inventory.
	respond(w, 200, map[string]any{"schema_version": "koschei.crypto-brief.v1", "items": items, "sources": Sources, "source_status": status["sources"], "enabled": s.Config.Enabled, "evidence_kind": "publisher_report"})
}
func (s *Service) CustomerHTTP(subject func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		customer := subject(r)
		if customer == "" {
			respond(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		switch r.Method {
		case "GET":
			subscriptions, e := s.Subscriptions(r.Context(), customer)
			if e != nil {
				respond(w, 503, map[string]string{"error": "subscriptions_unavailable"})
				return
			}
			respond(w, 200, map[string]any{"schema_version": "koschei.crypto-brief.v1", "subscriptions": subscriptions})
		case "PUT":
			var body struct {
				Channel     string      `json:"channel"`
				Preferences Preferences `json:"preferences"`
			}
			if !decode(w, r, &body) {
				return
			}
			if !validChannel(body.Channel) || validatePreferences(&body.Preferences) != nil {
				respond(w, 400, map[string]string{"error": "preferences_invalid"})
				return
			}
			if e := s.SetPreferences(r.Context(), customer, body.Channel, body.Preferences); e != nil {
				respond(w, 503, map[string]string{"error": "subscriptions_unavailable"})
				return
			}
			respond(w, 200, map[string]bool{"saved": true})
		case "PATCH", "DELETE":
			var body struct {
				Channel string `json:"channel"`
				State   string `json:"state"`
			}
			if !decode(w, r, &body) {
				return
			}
			if r.Method == "DELETE" {
				body.State = "disconnected"
			}
			if !validChannel(body.Channel) || !contains([]string{"active", "paused", "disconnected"}, body.State) {
				respond(w, 400, map[string]string{"error": "state_invalid"})
				return
			}
			e := s.ChangeState(r.Context(), customer, body.Channel, body.State)
			if e != nil {
				respond(w, 409, map[string]string{"error": "channel_state_change_failed"})
				return
			}
			respond(w, 200, map[string]bool{"saved": true})
		default:
			w.WriteHeader(405)
		}
	}
}
func (s *Service) PairHTTP(subject func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		customer := subject(r)
		if customer == "" {
			respond(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		var body struct {
			Channel string `json:"channel"`
			Consent bool   `json:"consent"`
		}
		if !decode(w, r, &body) {
			return
		}
		if !body.Consent || !validChannel(body.Channel) {
			respond(w, 400, map[string]string{"error": "explicit_consent_required"})
			return
		}
		p, e := s.Pair(r.Context(), customer, body.Channel)
		if e != nil {
			status := 409
			code := "channel_pairing_failed"
			if errors.Is(e, ErrChannelUnavailable) {
				status = 503
				code = "channel_not_configured"
			}
			if errors.Is(e, ErrPairRate) {
				status = 429
				code = "pairing_rate_limited"
			}
			respond(w, status, map[string]string{"error": code})
			return
		}
		respond(w, 200, p)
	}
}
func (s *Service) StatusHTTP(w http.ResponseWriter, r *http.Request) {
	v, e := s.Status(r.Context())
	if e != nil {
		respond(w, 503, map[string]string{"error": "crypto_brief_status_unavailable"})
		return
	}
	respond(w, 200, v)
}
func secretEqual(actual, expected string) bool {
	return expected != "" && len(actual) == len(expected) && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
func (s *Service) TelegramHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if !s.Config.Ready("telegram") {
		w.WriteHeader(503)
		return
	}
	if !secretEqual(r.Header.Get("X-Telegram-Bot-Api-Secret-Token"), s.Config.TelegramSecret) {
		w.WriteHeader(401)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	var v struct {
		UpdateID int64 `json:"update_id"`
		Message  *struct {
			Date int64  `json:"date"`
			Text string `json:"text"`
			From struct {
				ID  int64 `json:"id"`
				Bot bool  `json:"is_bot"`
			} `json:"from"`
			Chat struct {
				ID   int64  `json:"id"`
				Type string `json:"type"`
			} `json:"chat"`
		} `json:"message"`
	}
	d := json.NewDecoder(r.Body)
	if d.Decode(&v) != nil || d.Decode(&struct{}{}) != io.EOF {
		w.WriteHeader(400)
		return
	}
	if v.UpdateID <= 0 || v.Message == nil {
		w.WriteHeader(200)
		return
	}
	m := v.Message
	if m.Chat.Type != "private" || m.Chat.ID <= 0 || m.From.ID != m.Chat.ID || m.From.Bot || time.Since(time.Unix(m.Date, 0)) > 10*time.Minute || time.Unix(m.Date, 0).After(time.Now().Add(time.Minute)) {
		w.WriteHeader(200)
		return
	}
	e := s.Inbound(r.Context(), "telegram", strconv.FormatInt(v.UpdateID, 10), strconv.FormatInt(m.Chat.ID, 10), m.Text)
	if errors.Is(e, ErrPairInvalid) {
		w.WriteHeader(200)
		return
	}
	if e != nil {
		w.WriteHeader(503)
		return
	}
	w.WriteHeader(200)
}
func VerifyWhatsAppSignature(body []byte, signature, secret string) bool {
	if secret == "" || !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	sig, e := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if e != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hmac.Equal(sig, mac.Sum(nil))
}
func (s *Service) WhatsAppHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Ready("whatsapp") {
		w.WriteHeader(503)
		return
	}
	if r.Method == "GET" {
		q := r.URL.Query()
		if q.Get("hub.mode") == "subscribe" && secretEqual(q.Get("hub.verify_token"), s.Config.WhatsAppVerify) && len(q.Get("hub.challenge")) <= 256 {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, q.Get("hub.challenge"))
			return
		}
		w.WriteHeader(403)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	body, e := io.ReadAll(r.Body)
	if e != nil {
		w.WriteHeader(413)
		return
	}
	if !VerifyWhatsAppSignature(body, r.Header.Get("X-Hub-Signature-256"), s.Config.WhatsAppSecret) {
		w.WriteHeader(401)
		return
	}
	var v struct {
		Object  string `json:"object"`
		Entries []struct {
			Changes []struct {
				Value struct {
					Metadata struct {
						PhoneID string `json:"phone_number_id"`
					} `json:"metadata"`
					Messages []struct {
						ID        string `json:"id"`
						From      string `json:"from"`
						Type      string `json:"type"`
						Timestamp string `json:"timestamp"`
						Text      struct {
							Body string `json:"body"`
						} `json:"text"`
					} `json:"messages"`
					Statuses []struct {
						ID        string `json:"id"`
						Recipient string `json:"recipient_id"`
						Status    string `json:"status"`
					} `json:"statuses"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &v) != nil || v.Object != "whatsapp_business_account" || len(v.Entries) > 20 {
		w.WriteHeader(400)
		return
	}
	for _, entry := range v.Entries {
		if len(entry.Changes) > 20 {
			w.WriteHeader(400)
			return
		}
		for _, change := range entry.Changes {
			value := change.Value
			if value.Metadata.PhoneID != s.Config.WhatsAppPhoneID {
				continue
			}
			if len(value.Messages) > 50 || len(value.Statuses) > 50 {
				w.WriteHeader(400)
				return
			}
			for _, m := range value.Messages {
				stamp, e := strconv.ParseInt(m.Timestamp, 10, 64)
				if e != nil || time.Since(time.Unix(stamp, 0)) > 10*time.Minute || time.Unix(stamp, 0).After(time.Now().Add(time.Minute)) || m.Type != "text" {
					continue
				}
				e = s.Inbound(r.Context(), "whatsapp", m.ID, m.From, m.Text.Body)
				if errors.Is(e, ErrPairInvalid) {
					continue
				}
				if e != nil {
					w.WriteHeader(503)
					return
				}
			}
			for _, status := range value.Statuses {
				if status.ID == "" || len(status.ID) > 256 || !digits.MatchString(status.Recipient) {
					continue
				}
				state := ""
				if status.Status == "delivered" || status.Status == "read" {
					state = "delivered"
				} else if status.Status == "failed" {
					state = "failed"
				}
				if state != "" {
					_, e = s.DB.ExecContext(r.Context(), `UPDATE crypto_brief_deliveries d SET state=$3,reason=CASE WHEN $3='failed' THEN 'provider_delivery_failed' ELSE '' END,updated_at=clock_timestamp() FROM crypto_brief_subscriptions s WHERE d.subscription_id=s.id AND s.channel='whatsapp' AND s.recipient=$2 AND d.provider_id=$1 AND d.state IN ('accepted','uncertain')`, status.ID, status.Recipient, state)
					if e != nil {
						w.WriteHeader(503)
						return
					}
				}
			}
		}
	}
	w.WriteHeader(200)
}
