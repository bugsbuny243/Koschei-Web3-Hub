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
	"time"

	"koschei/api/internal/outboundhttp"
	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/workerwake"
)

type delivery struct {
	id, subscription, token       int64
	channel, recipient, body, key string
	inbound                       sql.NullTime
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

func (p Provider) Send(ctx context.Context, channel, recipient, body string, _ time.Time) SendResult {
	if channel != "telegram" || !p.Config.Ready("telegram") {
		return SendResult{State: "pending", Reason: "channel_not_configured", RetryAfter: time.Hour}
	}
	endpoint := "https://api.telegram.org/bot" + p.Config.TelegramToken + "/sendMessage"
	payload := map[string]any{
		"chat_id": recipient,
		"text":    body,
		"link_preview_options": map[string]any{
			"is_disabled": true,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SendResult{State: "failed", Reason: "provider_payload_invalid"}
	}
	req, err := outboundhttp.NewRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return SendResult{State: "failed", Reason: "provider_request_invalid"}
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := outboundhttp.Do(p.Client, req)
	if err != nil {
		return SendResult{State: "uncertain", Reason: "provider_acknowledgement_unknown"}
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return SendResult{State: "pending", Reason: "provider_rate_limited", RetryAfter: 10 * time.Minute}
	}
	if res.StatusCode >= 400 && res.StatusCode < 500 {
		return SendResult{State: "failed", Reason: "provider_rejected"}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return SendResult{State: "uncertain", Reason: "provider_acknowledgement_unknown"}
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil || len(b) > 65536 {
		return SendResult{State: "uncertain", Reason: "provider_receipt_invalid"}
	}
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
		if v.ErrorCode == http.StatusTooManyRequests {
			return SendResult{State: "pending", Reason: "provider_rate_limited", RetryAfter: 10 * time.Minute}
		}
		return SendResult{State: "failed", Reason: "provider_rejected"}
	}
	if v.Result.ID > 0 {
		return SendResult{State: "accepted", ProviderID: recipient + ":" + strconv.FormatInt(v.Result.ID, 10)}
	}
	return SendResult{State: "uncertain", Reason: "provider_receipt_invalid"}
}

func (s *Service) claim(ctx context.Context) (delivery, bool, error) {
	// Telegram does not provide an idempotency key for sendMessage. A crashed
	// in-flight send is therefore marked uncertain rather than replayed blindly.
	_, err := s.DB.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET state='uncertain',reason='send_lease_expired',updated_at=clock_timestamp() WHERE state='sending' AND lease_until<clock_timestamp()`)
	if err != nil {
		return delivery{}, false, err
	}
	var d delivery
	err = s.DB.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT d.id
			FROM crypto_brief_deliveries d
			JOIN crypto_brief_subscriptions s ON s.id=d.subscription_id
			WHERE d.state='pending'
			  AND d.due_at<=clock_timestamp()
			  AND s.channel='telegram'
			  AND s.state='active'
			  AND s.recipient IS NOT NULL
			  AND $1
			  AND (d.dedup_key LIKE 'arvis:%' OR d.dedup_key LIKE 'reply:%')
			ORDER BY d.due_at,d.id
			FOR UPDATE OF d SKIP LOCKED
			LIMIT 1
		), claimed AS (
			UPDATE crypto_brief_deliveries
			SET state='sending',lease_until=clock_timestamp()+interval '2 minutes',fencing_token=fencing_token+1,
			    attempts=attempts+1,updated_at=clock_timestamp()
			WHERE id IN (SELECT id FROM candidate)
			RETURNING *
		)
		SELECT d.id,d.subscription_id,d.fencing_token,d.body,d.attempts,s.channel,s.recipient,s.last_inbound_at,d.dedup_key
		FROM claimed d
		JOIN crypto_brief_subscriptions s ON s.id=d.subscription_id`, s.telegramDeliveryReady()).Scan(
		&d.id, &d.subscription, &d.token, &d.body, &d.attempts, &d.channel, &d.recipient, &d.inbound, &d.key,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return d, false, nil
	}
	return d, err == nil, err
}

func (s *Service) ProcessOne(ctx context.Context, sender Sender, now time.Time) (bool, error) {
	d, found, err := s.claim(ctx)
	if err != nil || !found {
		return found, err
	}
	result := SendResult{State: "cancelled", Reason: "customer_opt_out"}
	var active bool
	// Re-check consent immediately before external I/O; no transaction is held
	// while Telegram is contacted.
	err = s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM crypto_brief_subscriptions WHERE id=$1 AND channel='telegram' AND state='active' AND recipient=$2)`, d.subscription, d.recipient).Scan(&active)
	if err != nil {
		result = SendResult{State: "pending", Reason: "consent_check_unavailable", RetryAfter: 15 * time.Minute}
	} else if active {
		if !s.telegramDeliveryReady() {
			result = SendResult{State: "pending", Reason: "telegram_webhook_not_verified", RetryAfter: 15 * time.Minute}
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
	res, err := s.DB.ExecContext(finishCtx, `UPDATE crypto_brief_deliveries SET state=$3,reason=$4,provider_id=NULLIF($5,''),lease_until=NULL,due_at=clock_timestamp()+($6::double precision*interval '1 second'),updated_at=clock_timestamp() WHERE id=$1 AND fencing_token=$2 AND state='sending' AND lease_until>clock_timestamp()`, d.id, d.token, result.State, result.Reason, result.ProviderID, delay.Seconds())
	if err != nil {
		return true, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return true, err
	}
	if n != 1 {
		return true, errors.New("delivery_fenced")
	}
	_ = now
	return true, nil
}

// Start now runs only the ARVIS Telegram delivery plane. RSS/news ingestion and
// scheduled news digests are intentionally retired: Koschei is a security
// product, and Telegram mirrors customer ARVIS investigations.
func Start(ctx context.Context, db *sql.DB, health *runtimehealth.Registry) func() {
	s := New(db)
	enabled := s.Config.Enabled && db != nil
	health.RegisterPeriodic("arvis-telegram-delivery", "worker", "", enabled, 30*time.Minute)
	if !enabled {
		health.RegisterPeriodic(telegramWebhookHealthID, "worker", "", false, 2*time.Hour)
		return func() {}
	}
	workerCtx, cancel := context.WithCancel(ctx)
	startTelegramWebhook(workerCtx, s, health)
	go func() {
		gate := workerwake.Get(wakeID)
		lastCleanup := time.Time{}
		for workerCtx.Err() == nil {
			cycleCtx, stop := context.WithTimeout(workerCtx, 90*time.Second)
			if time.Since(lastCleanup) > 24*time.Hour {
				if cleanupErr := s.Cleanup(cycleCtx); cleanupErr != nil {
					log.Print("ARVIS Telegram retention cycle failed")
				} else {
					lastCleanup = time.Now()
				}
			}
			delivered := 0
			var cycleErr error
			for i := 0; i < 50; i++ {
				worked, err := s.ProcessOne(cycleCtx, Provider{s.Config, s.Client}, time.Now().UTC())
				if err != nil {
					cycleErr = err
					break
				}
				if !worked {
					break
				}
				delivered++
			}
			if cycleErr != nil {
				health.Failure("arvis-telegram-delivery", errors.New("arvis_telegram_delivery_cycle_failed"))
				log.Print("ARVIS Telegram delivery cycle failed")
			} else {
				health.Success("arvis-telegram-delivery", delivered)
			}
			stop()
			wait := 15 * time.Minute
			if delivered >= 50 {
				wait = time.Second
			}
			gate.Wait(workerCtx, wait)
		}
	}()
	return cancel
}
