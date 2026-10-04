package cryptobrief

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/outboundhttp"
	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/workerwake"
)

type delivery struct {
	id, subscription, token       int64
	channel, recipient, body, key string
	inbound                       sql.NullTime
	preferences                   Preferences
	attempts                      int
}
type SendResult struct {
	State      string
	Reason     string
	ProviderID string
	RetryAfter time.Duration
}
type Sender interface {
	Send(context.Context, string, string, string, time.Time) SendResult
}
type Provider struct {
	Config Config
	Client *http.Client
}

func (p Provider) Send(ctx context.Context, channel, recipient, body string, inbound time.Time) SendResult {
	if !p.Config.Ready(channel) {
		return SendResult{State: "pending", Reason: "channel_not_configured", RetryAfter: time.Hour}
	}
	var endpoint string
	var payload any
	if channel == "telegram" {
		endpoint = "https://api.telegram.org/bot" + p.Config.TelegramToken + "/sendMessage"
		payload = map[string]any{"chat_id": recipient, "text": body, "link_preview_options": map[string]any{"is_disabled": true}}
	} else {
		endpoint = "https://graph.facebook.com/" + p.Config.WhatsAppVersion + "/" + p.Config.WhatsAppPhoneID + "/messages"
		if !inbound.IsZero() && time.Since(inbound) < 24*time.Hour && time.Since(inbound) >= 0 {
			payload = map[string]any{"messaging_product": "whatsapp", "to": recipient, "type": "text", "text": map[string]any{"body": body, "preview_url": false}}
		} else {
			if !p.Config.TemplateReady() {
				return SendResult{State: "pending", Reason: "whatsapp_template_required", RetryAfter: time.Hour}
			}
			payload = map[string]any{"messaging_product": "whatsapp", "to": recipient, "type": "template", "template": map[string]any{"name": p.Config.WhatsAppTemplate, "language": map[string]any{"code": p.Config.WhatsAppLanguage}, "components": []any{map[string]any{"type": "body", "parameters": []any{map[string]any{"type": "text", "text": cleanText(body, 900)}}}}}}
		}
	}
	encoded, e := json.Marshal(payload)
	if e != nil {
		return SendResult{State: "failed", Reason: "provider_payload_invalid"}
	}
	req, e := outboundhttp.NewRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if e != nil {
		return SendResult{State: "failed", Reason: "provider_request_invalid"}
	}
	req.Header.Set("Content-Type", "application/json")
	if channel == "whatsapp" {
		req.Header.Set("Authorization", "Bearer "+p.Config.WhatsAppToken)
	}
	res, e := outboundhttp.Do(p.Client, req)
	if e != nil {
		return SendResult{State: "uncertain", Reason: "provider_acknowledgement_unknown"}
	}
	defer res.Body.Close()
	if res.StatusCode == 429 {
		return SendResult{State: "pending", Reason: "provider_rate_limited", RetryAfter: 10 * time.Minute}
	}
	if res.StatusCode >= 400 && res.StatusCode < 500 {
		return SendResult{State: "failed", Reason: "provider_rejected"}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return SendResult{State: "uncertain", Reason: "provider_acknowledgement_unknown"}
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, 65537))
	if e != nil || len(b) > 65536 {
		return SendResult{State: "uncertain", Reason: "provider_receipt_invalid"}
	}
	if channel == "telegram" {
		var v struct {
			OK        bool `json:"ok"`
			ErrorCode int  `json:"error_code"`
			Result    struct {
				ID int64 `json:"message_id"`
			} `json:"result"`
		}
		if json.Unmarshal(b, &v) != nil {
			return SendResult{State: "uncertain", Reason: "provider_receipt_invalid"}
		}
		if !v.OK {
			if v.ErrorCode == 429 {
				return SendResult{State: "pending", Reason: "provider_rate_limited", RetryAfter: 10 * time.Minute}
			}
			return SendResult{State: "failed", Reason: "provider_rejected"}
		}
		if v.Result.ID > 0 {
			return SendResult{State: "accepted", ProviderID: recipient + ":" + strconv.FormatInt(v.Result.ID, 10)}
		}
	} else {
		var v struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		}
		if json.Unmarshal(b, &v) == nil && len(v.Messages) > 0 && v.Messages[0].ID != "" && len(v.Messages[0].ID) < 256 {
			return SendResult{State: "accepted", ProviderID: v.Messages[0].ID}
		}
	}
	return SendResult{State: "uncertain", Reason: "provider_receipt_invalid"}
}
func (s *Service) claim(ctx context.Context) (delivery, bool, error) {
	// Crashed sends cannot be replayed safely: neither provider accepts an idempotency key.
	_, e := s.DB.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET state='uncertain',reason='send_lease_expired',updated_at=clock_timestamp() WHERE state='sending' AND lease_until<clock_timestamp()`)
	if e != nil {
		return delivery{}, false, e
	}
	var d delivery
	var quietStart, quietEnd int
	var tz string
	e = s.DB.QueryRowContext(ctx, `WITH candidate AS (SELECT d.id FROM crypto_brief_deliveries d JOIN crypto_brief_subscriptions s ON s.id=d.subscription_id WHERE d.state='pending' AND d.due_at<=clock_timestamp() AND s.state='active' AND ((s.channel='telegram' AND $1) OR (s.channel='whatsapp' AND $2)) ORDER BY d.due_at,d.id FOR UPDATE OF d SKIP LOCKED LIMIT 1), claimed AS (UPDATE crypto_brief_deliveries SET state='sending',lease_until=clock_timestamp()+interval '2 minutes',fencing_token=fencing_token+1,attempts=attempts+1,updated_at=clock_timestamp() WHERE id IN (SELECT id FROM candidate) RETURNING *) SELECT d.id,d.subscription_id,d.fencing_token,d.body,d.attempts,s.channel,s.recipient,s.last_inbound_at,s.quiet_start,s.quiet_end,s.timezone,d.dedup_key FROM claimed d JOIN crypto_brief_subscriptions s ON s.id=d.subscription_id`, s.Config.Ready("telegram"), s.Config.Ready("whatsapp")).Scan(&d.id, &d.subscription, &d.token, &d.body, &d.attempts, &d.channel, &d.recipient, &d.inbound, &quietStart, &quietEnd, &tz, &d.key)
	if errors.Is(e, sql.ErrNoRows) {
		return d, false, nil
	}
	d.preferences = Preferences{QuietStart: quietStart, QuietEnd: quietEnd, Timezone: tz}
	return d, e == nil, e
}
func (s *Service) ProcessOne(ctx context.Context, sender Sender, now time.Time) (bool, error) {
	d, found, e := s.claim(ctx)
	if e != nil || !found {
		return found, e
	}
	result := SendResult{State: "cancelled", Reason: "customer_opt_out"}
	var active bool
	// Re-check consent immediately before external I/O; no database transaction is held during a send.
	e = s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM crypto_brief_subscriptions WHERE id=$1 AND state='active' AND recipient=$2)`, d.subscription, d.recipient).Scan(&active)
	if e != nil {
		result = SendResult{State: "pending", Reason: "consent_check_unavailable", RetryAfter: 15 * time.Minute}
	} else if active {
		if Quiet(now, d.preferences) && !strings.HasPrefix(d.key, "reply:") {
			result = SendResult{State: "pending", Reason: "customer_quiet_hours", RetryAfter: time.Hour}
		} else {
			sendCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
			result = sender.Send(sendCtx, d.channel, d.recipient, d.body, d.inbound.Time)
			cancel()
		}
	}
	if result.State == "pending" && d.attempts >= 12 && result.Reason == "provider_rate_limited" {
		result = SendResult{State: "failed", Reason: "provider_rate_limit_exhausted"}
	}
	if !contains([]string{"pending", "accepted", "uncertain", "failed", "cancelled"}, result.State) {
		result = SendResult{State: "uncertain", Reason: "provider_result_invalid"}
	}
	delay := result.RetryAfter
	if delay <= 0 {
		delay = 15 * time.Minute
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	res, e := s.DB.ExecContext(finishCtx, `UPDATE crypto_brief_deliveries SET state=$3,reason=$4,provider_id=NULLIF($5,''),lease_until=NULL,due_at=clock_timestamp()+($6::double precision*interval '1 second'),updated_at=clock_timestamp() WHERE id=$1 AND fencing_token=$2 AND state='sending' AND lease_until>clock_timestamp()`, d.id, d.token, result.State, result.Reason, result.ProviderID, delay.Seconds())
	if e != nil {
		return true, e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return true, e
	}
	if n != 1 {
		return true, errors.New("delivery_fenced")
	}
	return true, nil
}
func Start(ctx context.Context, db *sql.DB, health *runtimehealth.Registry) func() {
	s := New(db)
	enabled := s.Config.Enabled && db != nil
	health.RegisterPeriodic("crypto-brief-feeds", "worker", "", enabled, time.Hour)
	health.RegisterPeriodic("crypto-brief-delivery", "worker", "", enabled, 30*time.Minute)
	if !enabled {
		return func() {}
	}
	workerCtx, cancel := context.WithCancel(ctx)
	go func() {
		gate := workerwake.Get(wakeID)
		lastCleanup := time.Time{}
		for workerCtx.Err() == nil {
			cycleCtx, stop := context.WithTimeout(workerCtx, 90*time.Second)
			n, e := s.RefreshFeeds(cycleCtx)
			if time.Since(lastCleanup) > 24*time.Hour {
				if cleanupErr := s.Cleanup(cycleCtx); cleanupErr != nil {
					log.Print("crypto brief retention cycle failed")
				} else {
					lastCleanup = time.Now()
				}
			}
			if e != nil {
				health.Failure("crypto-brief-feeds", errors.New("crypto_brief_source_sync_failed"))
			} else {
				health.Success("crypto-brief-feeds", n)
			}
			e = nil
			for i := 0; i < 50; i++ {
				worked, err := s.ScheduleOne(cycleCtx, time.Now().UTC())
				if err != nil {
					e = err
					break
				}
				if !worked {
					break
				}
			}
			delivered := 0
			if e == nil {
				for i := 0; i < 50; i++ {
					worked, err := s.ProcessOne(cycleCtx, Provider{s.Config, s.Client}, time.Now().UTC())
					if err != nil {
						e = err
						break
					}
					if !worked {
						break
					}
					delivered++
				}
			}
			// API acceptances, uncertain outcomes and blocked templates remain separately visible in Status.
			if e != nil {
				health.Failure("crypto-brief-delivery", errors.New("crypto_brief_delivery_cycle_failed"))
				log.Print("crypto brief delivery cycle failed")
			} else {
				health.Success("crypto-brief-delivery", delivered)
			}
			stop()
			wait := 15 * time.Minute
			if n > 0 || delivered >= 50 {
				wait = time.Second
			}
			gate.Wait(workerCtx, wait)
		}
	}()
	return cancel
}
