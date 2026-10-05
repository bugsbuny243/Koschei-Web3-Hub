package cryptobrief

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"

	"koschei/api/internal/workerwake"
)

// QueueARVISResult reuses the customer's existing Telegram pairing to deliver
// an ARVIS scan result. Delivery is opt-in only: no active, consented Telegram
// subscription means no message is queued. The existing delivery worker keeps
// provider I/O, retries and webhook verification outside the scan request.
func (s *Service) QueueARVISResult(ctx context.Context, customer, resultID, body string) (bool, error) {
	if s == nil || s.DB == nil {
		return false, nil
	}
	customer = strings.TrimSpace(customer)
	body = trimCustomerMessage(body, 3400)
	if customer == "" || body == "" {
		return false, nil
	}

	var subscriptionID int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT id
		FROM crypto_brief_subscriptions
		WHERE customer_sub=$1
		  AND channel='telegram'
		  AND state='active'
		  AND recipient IS NOT NULL
		  AND consent_at IS NOT NULL
		LIMIT 1`, customer).Scan(&subscriptionID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	resultID = strings.TrimSpace(resultID)
	if resultID == "" {
		sum := sha256.Sum256([]byte(body))
		resultID = hex.EncodeToString(sum[:])
	}
	if len(resultID) > 220 {
		sum := sha256.Sum256([]byte(resultID))
		resultID = hex.EncodeToString(sum[:])
	}
	// reply:* is intentionally immediate in the existing delivery worker, which
	// is appropriate for a customer-triggered scan response rather than a digest.
	dedupKey := "reply:arvis:" + resultID

	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO crypto_brief_deliveries(subscription_id,dedup_key,body)
		VALUES($1,$2,$3)
		ON CONFLICT(subscription_id,dedup_key) DO NOTHING`, subscriptionID, dedupKey, body)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if rows > 0 {
		workerwake.Signal(wakeID)
		return true, nil
	}
	return false, nil
}

func trimCustomerMessage(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
