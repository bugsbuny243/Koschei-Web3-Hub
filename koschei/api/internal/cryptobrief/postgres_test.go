package cryptobrief

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

type fixtureSender struct {
	db     *sql.DB
	calls  int
	result SendResult
}

func (f *fixtureSender) Send(ctx context.Context, channel, recipient, body string, inbound time.Time) SendResult {
	var one int
	if err := f.db.QueryRowContext(ctx, `SELECT 1`).Scan(&one); err != nil {
		return SendResult{State: "failed", Reason: "test_database_transaction_held"}
	}
	f.calls++
	return f.result
}

func TestARVISTelegramPostgres17(t *testing.T) {
	connection := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("isolated PostgreSQL 17 test database required")
	}
	db, err := sql.Open("postgres", connection)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var occupied bool
	if err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM crypto_brief_subscriptions) OR EXISTS(SELECT 1 FROM crypto_brief_deliveries)`).Scan(&occupied); err != nil || occupied {
		t.Fatalf("empty isolated test database required: %v", err)
	}

	s := &Service{
		DB: db,
		Config: Config{
			Enabled:          true,
			TelegramToken:    "12345:fixture",
			TelegramSecret:   strings.Repeat("f", 32),
			TelegramUsername: "FixtureBot",
		},
	}
	customer := "fixture-customer-arvis-telegram"
	other := "fixture-other-arvis-telegram"
	recipient := "123456789"
	now := time.Now().UTC()

	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = db.ExecContext(cleanup, `DELETE FROM crypto_brief_inbound WHERE event_id LIKE 'fixture-arvis-%'`)
		_, _ = db.ExecContext(cleanup, `DELETE FROM crypto_brief_pairings WHERE customer_sub IN ($1,$2)`, customer, other)
		_, _ = db.ExecContext(cleanup, `DELETE FROM crypto_brief_subscriptions WHERE customer_sub IN ($1,$2)`, customer, other)
	})

	pair, err := s.Pair(ctx, customer, "telegram")
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err = db.QueryRowContext(ctx, `SELECT token_hash FROM crypto_brief_pairings WHERE customer_sub=$1`, customer).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == pair.Code || stored != hash(pair.Code) {
		t.Fatal("pairing token was not stored as a one-way hash")
	}
	if _, err = s.Pair(ctx, customer, "telegram"); !errors.Is(err, ErrPairRate) {
		t.Fatalf("pair rate: %v", err)
	}

	if err = s.ARVISInbound(ctx, "fixture-arvis-bind", recipient, "/start "+pair.Code); err != nil {
		t.Fatal(err)
	}
	if err = s.ARVISInbound(ctx, "fixture-arvis-bind", recipient, "/start "+pair.Code); err != nil {
		t.Fatal("replayed bind lost idempotency")
	}
	if err = s.ARVISInbound(ctx, "fixture-arvis-reuse", recipient, "/start "+pair.Code); !errors.Is(err, ErrPairInvalid) {
		t.Fatal("consumed pairing token was reusable")
	}

	second, err := s.Pair(ctx, other, "telegram")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ARVISInbound(ctx, "fixture-arvis-steal", recipient, "/start "+second.Code); !errors.Is(err, ErrPairInvalid) {
		t.Fatal("paired Telegram recipient transferred between customers")
	}

	var subID int64
	var state string
	var storedRecipient sql.NullString
	if err = db.QueryRowContext(ctx, `SELECT id,state,recipient FROM crypto_brief_subscriptions WHERE customer_sub=$1 AND channel='telegram'`, customer).Scan(&subID, &state, &storedRecipient); err != nil {
		t.Fatal(err)
	}
	if state != "active" || !storedRecipient.Valid || storedRecipient.String != recipient {
		t.Fatal("paired Telegram subscription is not active for the expected recipient")
	}
	var otherRecipient sql.NullString
	if err = db.QueryRowContext(ctx, `SELECT recipient FROM crypto_brief_subscriptions WHERE customer_sub=$1 AND channel='telegram'`, other).Scan(&otherRecipient); err != nil {
		t.Fatal(err)
	}
	if otherRecipient.Valid {
		t.Fatal("cross-customer Telegram recipient leak")
	}

	// Pairing acknowledgement is tested by the inbound contract separately. Mark
	// it accepted here so queue assertions below exercise ARVIS result delivery only.
	if _, err = db.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET state='accepted' WHERE subscription_id=$1 AND dedup_key LIKE 'reply:%'`, subID); err != nil {
		t.Fatal(err)
	}

	envelope := map[string]any{
		"status":            "ready",
		"has_live_evidence": true,
		"final_verdict": map[string]any{
			"verdict":        "withhold",
			"grade":          "C",
			"risk_level":     "high",
			"recommendation": "Review authority evidence before proceeding.",
		},
	}
	queued, err := s.QueueARVISResult(ctx, customer, "scan-1", "target-fixture", "solana-mainnet", envelope)
	if err != nil || !queued {
		t.Fatalf("queue ARVIS result: queued=%v err=%v", queued, err)
	}
	queuedAgain, err := s.QueueARVISResult(ctx, customer, "scan-1", "target-fixture", "solana-mainnet", envelope)
	if err != nil || queuedAgain {
		t.Fatalf("ARVIS result dedupe failed: queued=%v err=%v", queuedAgain, err)
	}
	var count int
	var body string
	if err = db.QueryRowContext(ctx, `SELECT count(*),max(body) FROM crypto_brief_deliveries WHERE subscription_id=$1 AND dedup_key='arvis:scan-1'`, subID).Scan(&count, &body); err != nil {
		t.Fatal(err)
	}
	if count != 1 || !strings.Contains(body, "ARVIS taraması tamamlandı") || strings.Contains(strings.ToLower(body), "gündem") {
		t.Fatalf("ARVIS delivery queue contains the wrong product content: count=%d body=%q", count, body)
	}

	// Pausing cancels queued ARVIS delivery and prevents a provider call.
	if err = s.ChangeState(ctx, customer, "telegram", "paused"); err != nil {
		t.Fatal(err)
	}
	sender := &fixtureSender{db: db, result: SendResult{State: "accepted", ProviderID: "fixture-message"}}
	if worked, processErr := s.ProcessOne(ctx, sender, now); processErr != nil || worked || sender.calls != 0 {
		t.Fatalf("paused recipient was sent a message: worked=%v err=%v calls=%d", worked, processErr, sender.calls)
	}
	if err = s.ChangeState(ctx, customer, "telegram", "active"); err != nil {
		t.Fatal(err)
	}

	queued, err = s.QueueARVISResult(ctx, customer, "scan-2", "target-fixture", "solana-mainnet", envelope)
	if err != nil || !queued {
		t.Fatalf("queue verification fixture: queued=%v err=%v", queued, err)
	}

	// No result can be sent until runtime webhook verification succeeds.
	s.telegramVerificationRequired = true
	if _, found, claimErr := s.claim(ctx); claimErr != nil || found {
		t.Fatalf("Telegram result claimed before webhook verification: found=%v err=%v", found, claimErr)
	}
	s.telegramVerified.Store(true)
	first, found, claimErr := s.claim(ctx)
	if claimErr != nil || !found {
		t.Fatalf("claim verified ARVIS result: found=%v err=%v", found, claimErr)
	}
	if _, found, claimErr = s.claim(ctx); claimErr != nil || found {
		t.Fatalf("live ARVIS result claimed twice: found=%v err=%v", found, claimErr)
	}
	if _, err = db.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, first.id); err != nil {
		t.Fatal(err)
	}
	if _, found, claimErr = s.claim(ctx); claimErr != nil || found {
		t.Fatalf("uncertain Telegram send was replayed: found=%v err=%v", found, claimErr)
	}
	if err = db.QueryRowContext(ctx, `SELECT state FROM crypto_brief_deliveries WHERE id=$1`, first.id).Scan(&state); err != nil || state != "uncertain" {
		t.Fatalf("expired Telegram send lost uncertainty: state=%q err=%v", state, err)
	}

	queued, err = s.QueueARVISResult(ctx, customer, "scan-3", "target-fixture", "solana-mainnet", envelope)
	if err != nil || !queued {
		t.Fatalf("queue provider-send fixture: queued=%v err=%v", queued, err)
	}
	if worked, processErr := s.ProcessOne(ctx, sender, now); processErr != nil || !worked || sender.calls != 1 {
		t.Fatalf("provider send boundary: worked=%v err=%v calls=%d", worked, processErr, sender.calls)
	}
	if err = db.QueryRowContext(ctx, `SELECT state FROM crypto_brief_deliveries WHERE subscription_id=$1 AND dedup_key='arvis:scan-3'`, subID).Scan(&state); err != nil || state != "accepted" {
		t.Fatalf("Telegram API acceptance misreported: state=%q err=%v", state, err)
	}

	if err = s.ChangeState(ctx, other, "telegram", "disconnected"); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM crypto_brief_pairings WHERE customer_sub=$1`, other).Scan(&count); err != nil || count != 0 {
		t.Fatal("disconnect retained unused pairing")
	}
	if err = s.ChangeState(ctx, customer, "telegram", "disconnected"); err != nil {
		t.Fatal(err)
	}
	var hasAddress bool
	if err = db.QueryRowContext(ctx, `SELECT recipient IS NOT NULL FROM crypto_brief_subscriptions WHERE id=$1`, subID).Scan(&hasAddress); err != nil || hasAddress {
		t.Fatal("disconnect retained Telegram recipient")
	}
	var hasOriginalBody bool
	if err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM crypto_brief_deliveries WHERE subscription_id=$1 AND body LIKE '%ARVIS taraması tamamlandı%')`, subID).Scan(&hasOriginalBody); err != nil || hasOriginalBody {
		t.Fatal("disconnect retained ARVIS Telegram message body")
	}
}
