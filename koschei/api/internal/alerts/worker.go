package alerts

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/webhooks"
)

const (
	alertPollInterval  = 4 * time.Second
	alertBatchSize     = 10
	alertResponseLimit = 16 << 10
)

var telegramTokenPattern = regexp.MustCompile(`^[0-9]{4,20}:[A-Za-z0-9_-]{20,160}$`)

type deliveryRecord struct {
	ID           string
	AlertID      string
	Channel      string
	AttemptCount int
	MaxAttempts  int
	EventType    string
	Severity     string
	Target       string
	Title        string
	Message      string
	Payload      []byte
}

func StartDeliveryWorker(parent context.Context, db *sql.DB) func() {
	if db == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	client := webhooks.NewDeliveryClient()
	var once sync.Once
	go func() {
		if transport, ok := client.Transport.(*http.Transport); ok {
			defer transport.CloseIdleConnections()
		}
		_, _ = db.ExecContext(ctx, `
			UPDATE security_alert_deliveries
			SET status='retry',locked_at=NULL,next_attempt_at=now(),updated_at=now(),
			    last_error=COALESCE(last_error,'alert worker recovered stale lock')
			WHERE status='processing' AND locked_at < now()-interval '2 minutes'`)
		ticker := time.NewTicker(alertPollInterval)
		defer ticker.Stop()
		for {
			if err := processBatch(ctx, db, client); err != nil && ctx.Err() == nil {
				log.Printf("security alert delivery worker: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() { once.Do(cancel) }
}

func processBatch(ctx context.Context, db *sql.DB, client *http.Client) error {
	for i := 0; i < alertBatchSize; i++ {
		item, err := claimDelivery(ctx, db)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		processDelivery(ctx, db, client, item)
	}
	return nil
}

func claimDelivery(ctx context.Context, db *sql.DB) (deliveryRecord, error) {
	var item deliveryRecord
	err := db.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT id
			FROM security_alert_deliveries
			WHERE status IN ('pending','retry') AND next_attempt_at <= now()
			ORDER BY next_attempt_at ASC,created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE security_alert_deliveries d
		SET status='processing',attempt_count=d.attempt_count+1,locked_at=now(),updated_at=now()
		FROM candidate c, security_alert_events e
		WHERE d.id=c.id AND e.id=d.alert_id
		RETURNING d.id::text,d.alert_id::text,d.channel,d.attempt_count,d.max_attempts,
		          e.event_type,e.severity,e.target,e.title,e.message,e.payload`).
		Scan(&item.ID, &item.AlertID, &item.Channel, &item.AttemptCount, &item.MaxAttempts,
			&item.EventType, &item.Severity, &item.Target, &item.Title, &item.Message, &item.Payload)
	return item, err
}

func processDelivery(ctx context.Context, db *sql.DB, client *http.Client, item deliveryRecord) {
	deliveryCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	endpoint, payload, err := channelRequest(item)
	if err != nil {
		markFailure(ctx, db, item, 0, err.Error(), false)
		return
	}
	parsed, err := webhooks.ValidateEndpointURL(deliveryCtx, endpoint)
	if err != nil {
		markFailure(ctx, db, item, 0, "alert endpoint validation failed", false)
		return
	}
	body, err := json.Marshal(payload)
	if err != nil {
		markFailure(ctx, db, item, 0, "alert payload encoding failed", false)
		return
	}
	req, err := http.NewRequestWithContext(deliveryCtx, http.MethodPost, parsed.String(), bytes.NewReader(body))
	if err != nil {
		markFailure(ctx, db, item, 0, "alert request creation failed", false)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Koschei-Security-Alert/1.0")

	resp, err := client.Do(req)
	if err != nil {
		markFailure(ctx, db, item, 0, sanitizeError(err), true)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, alertResponseLimit))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		markSuccess(ctx, db, item, resp.StatusCode)
		return
	}
	retryable := resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooEarly || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
	markFailure(ctx, db, item, resp.StatusCode, fmt.Sprintf("alert endpoint returned HTTP %d", resp.StatusCode), retryable)
}

func channelRequest(item deliveryRecord) (string, map[string]any, error) {
	message := alertMessage(item)
	switch item.Channel {
	case "telegram":
		token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
		chatID := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))
		if !telegramTokenPattern.MatchString(token) || chatID == "" {
			return "", nil, fmt.Errorf("telegram alert configuration is unavailable")
		}
		return "https://api.telegram.org/bot" + token + "/sendMessage", map[string]any{
			"chat_id": chatID,
			"text": message,
			"disable_web_page_preview": true,
		}, nil
	case "discord":
		endpoint := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL"))
		parsed, err := url.Parse(endpoint)
		if err != nil || !allowedDiscordHost(parsed.Hostname()) {
			return "", nil, fmt.Errorf("discord alert configuration is unavailable")
		}
		return endpoint, map[string]any{"content": message}, nil
	default:
		return "", nil, fmt.Errorf("unsupported alert channel")
	}
}

func allowedDiscordHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "discord.com" || host == "www.discord.com" || host == "discordapp.com" || host == "www.discordapp.com"
}

func alertMessage(item deliveryRecord) string {
	severity := strings.ToUpper(normalizeSeverity(item.Severity))
	parts := []string{"🚨 Koschei " + severity, item.Title}
	if strings.TrimSpace(item.Target) != "" {
		parts = append(parts, "Target: "+strings.TrimSpace(item.Target))
	}
	if strings.TrimSpace(item.Message) != "" && strings.TrimSpace(item.Message) != strings.TrimSpace(item.Title) {
		parts = append(parts, item.Message)
	}
	parts = append(parts, "Event: "+item.EventType, "Alert ID: "+item.AlertID)
	return truncate(strings.Join(parts, "\n"), 1900)
}

func markSuccess(ctx context.Context, db *sql.DB, item deliveryRecord, status int) {
	_, _ = db.ExecContext(ctx, `
		UPDATE security_alert_deliveries
		SET status='delivered',locked_at=NULL,last_http_status=$1,last_error=NULL,delivered_at=now(),updated_at=now()
		WHERE id=$2`, status, item.ID)
}

func markFailure(ctx context.Context, db *sql.DB, item deliveryRecord, httpStatus int, message string, retryable bool) {
	status := "dead_letter"
	nextAttempt := time.Now().UTC()
	if retryable && item.AttemptCount < item.MaxAttempts {
		status = "retry"
		nextAttempt = retryAt(time.Now().UTC(), item.AttemptCount)
	}
	var nullableStatus any
	if httpStatus > 0 {
		nullableStatus = httpStatus
	}
	_, _ = db.ExecContext(ctx, `
		UPDATE security_alert_deliveries
		SET status=$1,locked_at=NULL,next_attempt_at=$2,last_http_status=$3,last_error=$4,updated_at=now()
		WHERE id=$5`, status, nextAttempt, nullableStatus, truncate(strings.TrimSpace(message), 1000), item.ID)
}

func retryAt(now time.Time, attempt int) time.Time {
	delays := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 8 * time.Hour, 24 * time.Hour}
	index := attempt - 1
	if index < 0 {
		index = 0
	}
	if index >= len(delays) {
		index = len(delays) - 1
	}
	return now.Add(delays[index])
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.ReplaceAll(err.Error(), "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return truncate(value, 1000)
}
