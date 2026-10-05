package cryptobrief

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"koschei/api/internal/workerwake"
)

// QueueARVISResult delivers the result of a customer ARVIS investigation to the
// Telegram account explicitly paired with that customer. The legacy
// crypto_brief tables are reused as a compatibility storage layer while the
// broader persistence migration is in progress; no news/RSS state participates
// in this path.
func (s *Service) QueueARVISResult(ctx context.Context, customerSub, deliveryKey, target, network string, envelope map[string]any) (bool, error) {
	if s == nil || s.DB == nil || strings.TrimSpace(customerSub) == "" {
		return false, nil
	}
	body := FormatARVISResult(target, network, envelope)
	if body == "" {
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
		LIMIT 1`, strings.TrimSpace(customerSub)).Scan(&subscriptionID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	key := strings.TrimSpace(deliveryKey)
	if key == "" {
		key = randomDeliveryKey()
	}
	if !strings.HasPrefix(key, "arvis:") {
		key = "arvis:" + key
	}
	result, err := s.DB.ExecContext(ctx, `
		INSERT INTO crypto_brief_deliveries(subscription_id,dedup_key,body,due_at)
		VALUES($1,$2,$3,clock_timestamp())
		ON CONFLICT(subscription_id,dedup_key) DO NOTHING`, subscriptionID, key, body)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if rows > 0 {
		workerwake.Signal(wakeID)
		return true, nil
	}
	return false, nil
}

func randomDeliveryKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return "scan-result"
}

// FormatARVISResult keeps Telegram secondary to the canonical ARVIS result. It
// never upgrades missing evidence into a safe verdict and deliberately omits
// internal/raw evidence payloads that are better reviewed in the dashboard.
func FormatARVISResult(target, network string, envelope map[string]any) string {
	if envelope == nil {
		return ""
	}
	target = firstNonEmpty(strings.TrimSpace(target), mapText(envelope, "target"))
	network = firstNonEmpty(strings.TrimSpace(network), mapText(envelope, "network"), "solana-mainnet")
	status := strings.ToUpper(strings.TrimSpace(mapText(envelope, "status")))
	if status == "" {
		status = "COMPLETED"
	}

	final := mapValue(envelope, "final_verdict")
	analysis := mapValue(envelope, "analysis_summary")
	decision := mapValue(analysis, "decision")
	verdict := firstNonEmpty(mapText(final, "verdict"), mapText(decision, "verdict"), mapText(decision, "decision"), mapText(envelope, "verdict"))
	grade := firstNonEmpty(mapText(final, "grade"), mapText(decision, "grade"))
	risk := firstNonEmpty(mapText(final, "risk_level"), mapText(decision, "risk_level"), mapText(decision, "risk"))
	recommendation := firstNonEmpty(mapText(final, "recommendation"), mapText(decision, "recommendation"), mapText(analysis, "recommendation"))

	hasEvidence, evidenceKnown := mapBool(envelope, "has_live_evidence")
	if !evidenceKnown {
		if state := strings.TrimSpace(mapText(analysis, "evidence_state")); state != "" {
			hasEvidence = strings.EqualFold(state, "verified") || strings.EqualFold(state, "observed") || strings.EqualFold(state, "ready")
			evidenceKnown = true
		}
	}

	lines := []string{"🛡️ ARVIS taraması tamamlandı"}
	if target != "" {
		lines = append(lines, "Hedef: "+target)
	}
	if network != "" {
		lines = append(lines, "Ağ: "+network)
	}
	lines = append(lines, "Durum: "+status)
	if verdict != "" {
		lines = append(lines, "Sonuç: "+cleanLine(verdict, 420))
	}
	if risk != "" {
		lines = append(lines, "Risk: "+strings.ToUpper(cleanLine(risk, 80)))
	}
	if grade != "" {
		lines = append(lines, "ARVIS derece: "+cleanLine(grade, 40))
	}
	if evidenceKnown {
		if hasEvidence {
			lines = append(lines, "Kanıt: doğrulanmış/canlı kanıt mevcut")
		} else {
			lines = append(lines, "Kanıt: eksik veya doğrulanmamış — güvenli kabul edilmez")
		}
	}
	if recommendation != "" {
		lines = append(lines, "Öneri: "+cleanLine(recommendation, 700))
	}
	lines = append(lines, "Tam rapor: https://tradepigloball.co/dashboard")
	body := strings.Join(lines, "\n")
	if len(body) > 3400 {
		body = body[:3400]
	}
	return body
}

func mapValue(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	value, _ := parent[key].(map[string]any)
	return value
}

func mapText(parent map[string]any, key string) string {
	if parent == nil {
		return ""
	}
	switch value := parent[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}

func mapBool(parent map[string]any, key string) (bool, bool) {
	if parent == nil {
		return false, false
	}
	value, ok := parent[key].(bool)
	return value, ok
}

func cleanLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit > 0 && len(value) > limit {
		return value[:limit] + "…"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
