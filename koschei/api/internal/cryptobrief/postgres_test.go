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
	if e := f.db.QueryRowContext(ctx, `SELECT 1`).Scan(&one); e != nil {
		return SendResult{State: "failed", Reason: "test_database_transaction_held"}
	}
	f.calls++
	return f.result
}
func TestCryptoBriefPostgres17(t *testing.T) {
	connection := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("isolated PostgreSQL 17 test database required")
	}
	db, e := sql.Open("postgres", connection)
	if e != nil {
		t.Fatal(e)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var occupied bool
	if e = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM crypto_brief_subscriptions) OR EXISTS(SELECT 1 FROM crypto_brief_items)`).Scan(&occupied); e != nil || occupied {
		t.Fatalf("empty isolated test database required: %v", e)
	}
	s := &Service{DB: db, Config: Config{Enabled: true, TelegramToken: "12345:fixture", TelegramSecret: strings.Repeat("f", 32), TelegramUsername: "FixtureBot"}}
	customer := "fixture-customer-crypto-brief"
	other := "fixture-other-crypto-brief"
	recipient := "123456789"
	now := time.Now().UTC()
	p := Preferences{Networks: []string{"solana"}, Topics: []string{"network"}, Cadence: "hourly", Timezone: "UTC", QuietStart: 0, QuietEnd: 0}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, _ = db.ExecContext(cleanup, `DELETE FROM crypto_brief_subscriptions WHERE customer_sub IN ($1,$2)`, customer, other)
		_, _ = db.ExecContext(cleanup, `DELETE FROM crypto_brief_pairings WHERE customer_sub IN ($1,$2)`, customer, other)
		_, _ = db.ExecContext(cleanup, `DELETE FROM crypto_brief_inbound WHERE event_id LIKE 'fixture-crypto-%'`)
		_, _ = db.ExecContext(cleanup, `DELETE FROM crypto_brief_items WHERE source_id='fixture-source'`)
	})
	if e = s.SetPreferences(ctx, customer, "telegram", p); e != nil {
		t.Fatal(e)
	}
	pair, e := s.Pair(ctx, customer, "telegram")
	if e != nil {
		t.Fatal(e)
	}
	var stored string
	if e = db.QueryRowContext(ctx, `SELECT token_hash FROM crypto_brief_pairings WHERE customer_sub=$1`, customer).Scan(&stored); e != nil || stored == pair.Code || stored != hash(pair.Code) {
		t.Fatal("pairing persisted in plaintext")
	}
	if _, e = s.Pair(ctx, customer, "telegram"); !errors.Is(e, ErrPairRate) {
		t.Fatalf("pair rate: %v", e)
	}
	if e = s.Inbound(ctx, "telegram", "fixture-crypto-bind", recipient, "/start "+pair.Code); e != nil {
		t.Fatal(e)
	}
	if e = s.Inbound(ctx, "telegram", "fixture-crypto-bind", recipient, "/start "+pair.Code); e != nil {
		t.Fatal("replayed bind lost idempotency")
	}
	if e = s.Inbound(ctx, "telegram", "fixture-crypto-reuse", recipient, "/start "+pair.Code); !errors.Is(e, ErrPairInvalid) {
		t.Fatal("consumed token reused")
	}
	second, e := s.Pair(ctx, other, "telegram")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Inbound(ctx, "telegram", "fixture-crypto-steal", recipient, "/start "+second.Code); !errors.Is(e, ErrPairInvalid) {
		t.Fatal("recipient transferred between customers")
	}
	otherSubs, e := s.Subscriptions(ctx, other)
	if e != nil || otherSubs[0].State != "disconnected" || otherSubs[0].RecipientMask != "" {
		t.Fatal("cross-customer subscription leak")
	}
	var subID int64
	if e = db.QueryRowContext(ctx, `SELECT id FROM crypto_brief_subscriptions WHERE customer_sub=$1 AND channel='telegram'`, customer).Scan(&subID); e != nil {
		t.Fatal(e)
	}
	// Explicit fixtures prove filtering and exact headline dedup across digest windows.
	for _, v := range []struct{ id, network string }{{hash("fixture-solana"), "solana"}, {hash("fixture-bitcoin"), "bitcoin"}} {
		_, e = db.ExecContext(ctx, `INSERT INTO crypto_brief_items(id,source_id,title,url,published_at,networks,topics) VALUES($1,'fixture-source','Explicit test fixture','https://status.solana.com/incidents/fixture',$2,ARRAY[$3]::text[],ARRAY['network'])`, v.id, now.Add(-time.Hour), v.network)
		if e != nil {
			t.Fatal(e)
		}
	}
	all, e := s.Recent(ctx, nil, nil)
	if e != nil || len(all) != 2 {
		t.Fatalf("empty filters must return all news: %d %v", len(all), e)
	}
	if worked, e := s.ScheduleOne(ctx, now); e != nil || !worked {
		t.Fatalf("schedule: %v %v", worked, e)
	}
	var seen int
	if e = db.QueryRowContext(ctx, `SELECT count(*) FROM crypto_brief_seen WHERE subscription_id=$1`, subID).Scan(&seen); e != nil || seen != 1 {
		t.Fatal("network filter or item ledger")
	}
	_, e = db.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET next_digest_at=clock_timestamp() WHERE id=$1`, subID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.ScheduleOne(ctx, now.Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	var digests int
	if e = db.QueryRowContext(ctx, `SELECT count(*) FROM crypto_brief_deliveries WHERE dedup_key LIKE 'digest:%'`).Scan(&digests); e != nil || digests != 1 {
		t.Fatal("same headline repeated in another digest")
	}
	// Pausing cancels queued messages and prevents a provider call.
	if e = s.ChangeState(ctx, customer, "telegram", "paused"); e != nil {
		t.Fatal(e)
	}
	sender := &fixtureSender{db: db, result: SendResult{State: "accepted", ProviderID: "fixture-message"}}
	if worked, e := s.ProcessOne(ctx, sender, now); e != nil || worked || sender.calls != 0 {
		t.Fatal("opted-out recipient was sent a message")
	}
	if e = s.ChangeState(ctx, customer, "telegram", "active"); e != nil {
		t.Fatal(e)
	}
	_, e = db.ExecContext(ctx, `INSERT INTO crypto_brief_deliveries(subscription_id,dedup_key,body) VALUES($1,'fixture-claim','Explicit fixture')`, subID)
	if e != nil {
		t.Fatal(e)
	}
	first, found, e := s.claim(ctx)
	if e != nil || !found {
		t.Fatalf("claim: %v", e)
	}
	if _, found, e = s.claim(ctx); e != nil || found {
		t.Fatal("live send claimed twice")
	}
	_, e = db.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, first.id)
	if e != nil {
		t.Fatal(e)
	}
	if _, found, e = s.claim(ctx); e != nil || found {
		t.Fatal("uncertain crash replayed")
	}
	var state string
	if e = db.QueryRowContext(ctx, `SELECT state FROM crypto_brief_deliveries WHERE id=$1`, first.id).Scan(&state); e != nil || state != "uncertain" {
		t.Fatal("expired send lost uncertainty")
	}
	_, e = db.ExecContext(ctx, `INSERT INTO crypto_brief_deliveries(subscription_id,dedup_key,body) VALUES($1,'fixture-send','Explicit fixture')`, subID)
	if e != nil {
		t.Fatal(e)
	}
	if worked, e := s.ProcessOne(ctx, sender, now); e != nil || !worked || sender.calls != 1 {
		t.Fatalf("provider send/transaction boundary: %v %v calls=%d", worked, e, sender.calls)
	}
	if e = db.QueryRowContext(ctx, `SELECT state FROM crypto_brief_deliveries WHERE dedup_key='fixture-send'`).Scan(&state); e != nil || state != "accepted" {
		t.Fatal("API acceptance misreported")
	}
	if e = s.ChangeState(ctx, other, "telegram", "disconnected"); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = db.QueryRowContext(ctx, `SELECT count(*) FROM crypto_brief_pairings WHERE customer_sub=$1`, other).Scan(&count); e != nil || count != 0 {
		t.Fatal("disconnect retained pairing")
	}
	if e = s.ChangeState(ctx, customer, "telegram", "disconnected"); e != nil {
		t.Fatal(e)
	}
	var hasAddress bool
	if e = db.QueryRowContext(ctx, `SELECT recipient IS NOT NULL FROM crypto_brief_subscriptions WHERE id=$1`, subID).Scan(&hasAddress); e != nil || hasAddress {
		t.Fatal("disconnect retained recipient")
	}
	var hasBody bool
	if e = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM crypto_brief_deliveries WHERE subscription_id=$1 AND body='Explicit fixture')`, subID).Scan(&hasBody); e != nil || hasBody {
		t.Fatal("disconnect retained message text")
	}
}
