package cryptobrief

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"
	"koschei/api/internal/workerwake"
)

const wakeID = "crypto-brief-delivery"

var ErrPairRate = errors.New("pairing_rate_limited")
var ErrPairInvalid = errors.New("pairing_invalid_or_expired")
var ErrChannelUnavailable = errors.New("channel_not_configured")

type Config struct {
	Enabled                                                                                                                             bool
	TelegramToken, TelegramSecret, TelegramUsername                                                                                     string
	WhatsAppToken, WhatsAppPhoneID, WhatsAppSecret, WhatsAppVerify, WhatsAppVersion, WhatsAppNumber, WhatsAppTemplate, WhatsAppLanguage string
}

var digits = regexp.MustCompile(`^[0-9]{5,20}$`)
var botUsername = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{4,31}$`)
var telegramToken = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)
var graphVersion = regexp.MustCompile(`^v[0-9]{1,2}\.[0-9]{1,2}$`)
var templateName = regexp.MustCompile(`^[a-z0-9_]{1,128}$`)
var templateLanguage = regexp.MustCompile(`^[a-z]{2,3}(_[A-Z]{2})?$`)

func ConfigFromEnv() Config {
	get := func(k string) string { return strings.TrimSpace(os.Getenv("KOSCHEI_CRYPTO_BRIEF_" + k)) }
	c := Config{Enabled: get("ENABLED") == "1" || strings.EqualFold(get("ENABLED"), "true"), TelegramToken: get("TELEGRAM_BOT_TOKEN"), TelegramSecret: get("TELEGRAM_WEBHOOK_SECRET"), TelegramUsername: get("TELEGRAM_USERNAME"), WhatsAppToken: get("WHATSAPP_ACCESS_TOKEN"), WhatsAppPhoneID: get("WHATSAPP_PHONE_NUMBER_ID"), WhatsAppSecret: get("WHATSAPP_APP_SECRET"), WhatsAppVerify: get("WHATSAPP_VERIFY_TOKEN"), WhatsAppVersion: get("WHATSAPP_GRAPH_VERSION"), WhatsAppNumber: get("WHATSAPP_NUMBER"), WhatsAppTemplate: get("WHATSAPP_TEMPLATE_NAME"), WhatsAppLanguage: get("WHATSAPP_TEMPLATE_LANGUAGE")}
	if c.WhatsAppLanguage == "" {
		c.WhatsAppLanguage = "tr"
	}
	return c
}
func (c Config) Ready(channel string) bool {
	if !c.Enabled {
		return false
	}
	switch channel {
	case "telegram":
		return telegramToken.MatchString(c.TelegramToken) && len(c.TelegramSecret) >= 32 && botUsername.MatchString(c.TelegramUsername)
	case "whatsapp":
		return c.WhatsAppToken != "" && digits.MatchString(c.WhatsAppPhoneID) && len(c.WhatsAppSecret) >= 16 && len(c.WhatsAppVerify) >= 32 && graphVersion.MatchString(c.WhatsAppVersion) && digits.MatchString(c.WhatsAppNumber)
	}
	return false
}
func (c Config) TemplateReady() bool {
	return templateName.MatchString(c.WhatsAppTemplate) && templateLanguage.MatchString(c.WhatsAppLanguage)
}

type Service struct {
	DB     *sql.DB
	Client *http.Client
	Config Config
}

func New(db *sql.DB) *Service {
	return &Service{DB: db, Client: &http.Client{Timeout: 10 * time.Second}, Config: ConfigFromEnv()}
}

type Preferences struct {
	Networks   []string `json:"networks"`
	Topics     []string `json:"topics"`
	Cadence    string   `json:"cadence"`
	Timezone   string   `json:"timezone"`
	QuietStart int      `json:"quiet_start"`
	QuietEnd   int      `json:"quiet_end"`
}
type Subscription struct {
	Channel            string      `json:"channel"`
	State              string      `json:"state"`
	RecipientMask      string      `json:"recipient_mask,omitempty"`
	Configured         bool        `json:"configured"`
	TemplateConfigured bool        `json:"template_configured"`
	Preferences        Preferences `json:"preferences"`
	LastInbound        *time.Time  `json:"last_inbound_at,omitempty"`
}

var allowedNetworks = []string{"bitcoin", "ethereum", "solana", "global"}
var allowedTopics = []string{"security", "network", "defi", "regulation", "market", "general"}

func validatePreferences(p *Preferences) error {
	if p.Networks == nil {
		p.Networks = []string{}
	}
	if p.Topics == nil {
		p.Topics = []string{}
	}
	valid := func(a, allowed []string) bool {
		if len(a) > len(allowed) {
			return false
		}
		seen := map[string]bool{}
		for _, v := range a {
			if !contains(allowed, v) || seen[v] {
				return false
			}
			seen[v] = true
		}
		return true
	}
	if !valid(p.Networks, allowedNetworks) || !valid(p.Topics, allowedTopics) || (p.Cadence != "daily" && p.Cadence != "hourly") || p.QuietStart < 0 || p.QuietStart > 23 || p.QuietEnd < 0 || p.QuietEnd > 23 {
		return errors.New("preferences_invalid")
	}
	if p.Timezone == "" {
		p.Timezone = "Europe/Istanbul"
	}
	if len(p.Timezone) > 64 {
		return errors.New("timezone_invalid")
	}
	if _, e := time.LoadLocation(p.Timezone); e != nil {
		return errors.New("timezone_invalid")
	}
	return nil
}
func Quiet(now time.Time, p Preferences) bool {
	loc, e := time.LoadLocation(p.Timezone)
	if e != nil {
		loc = istanbul()
	}
	h := now.In(loc).Hour()
	if p.QuietStart == p.QuietEnd {
		return false
	}
	if p.QuietStart < p.QuietEnd {
		return h >= p.QuietStart && h < p.QuietEnd
	}
	return h >= p.QuietStart || h < p.QuietEnd
}
func NextDigest(now time.Time, p Preferences) time.Time {
	loc, e := time.LoadLocation(p.Timezone)
	if e != nil {
		loc = istanbul()
	}
	local := now.In(loc)
	next := now.Add(time.Hour)
	if p.Cadence == "daily" {
		next = time.Date(local.Year(), local.Month(), local.Day()+1, 9, 0, 0, 0, loc)
	}
	for i := 0; i < 25 && Quiet(next, p); i++ {
		next = next.Add(time.Hour)
	}
	return next.UTC()
}
func (s *Service) SetPreferences(ctx context.Context, customer, channel string, p Preferences) error {
	if customer == "" || len(customer) > 256 || !validChannel(channel) {
		return errors.New("subscription_invalid")
	}
	if e := validatePreferences(&p); e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `INSERT INTO crypto_brief_subscriptions(customer_sub,channel,networks,topics,cadence,timezone,quiet_start,quiet_end) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(customer_sub,channel) DO UPDATE SET networks=EXCLUDED.networks,topics=EXCLUDED.topics,cadence=EXCLUDED.cadence,timezone=EXCLUDED.timezone,quiet_start=EXCLUDED.quiet_start,quiet_end=EXCLUDED.quiet_end,updated_at=clock_timestamp()`, customer, channel, pq.Array(p.Networks), pq.Array(p.Topics), p.Cadence, p.Timezone, p.QuietStart, p.QuietEnd)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_deliveries d SET state='cancelled',reason='preferences_changed',body='Tercihler değiştirildi.' FROM crypto_brief_subscriptions s WHERE d.subscription_id=s.id AND s.customer_sub=$1 AND s.channel=$2 AND d.state='pending' AND d.dedup_key LIKE 'digest:%'`, customer, channel)
	if e != nil {
		return e
	}
	return tx.Commit()
}
func validChannel(s string) bool { return s == "telegram" || s == "whatsapp" }
func (s *Service) Subscriptions(ctx context.Context, customer string) ([]Subscription, error) {
	result := []Subscription{}
	for _, channel := range []string{"telegram", "whatsapp"} {
		v := Subscription{Channel: channel, State: "disconnected", Configured: s.Config.Ready(channel), TemplateConfigured: channel != "whatsapp" || s.Config.TemplateReady(), Preferences: Preferences{Networks: []string{}, Topics: []string{}, Cadence: "daily", Timezone: "Europe/Istanbul", QuietStart: 23, QuietEnd: 8}}
		var recipient sql.NullString
		var inbound sql.NullTime
		e := s.DB.QueryRowContext(ctx, `SELECT state,recipient,networks,topics,cadence,timezone,quiet_start,quiet_end,last_inbound_at FROM crypto_brief_subscriptions WHERE customer_sub=$1 AND channel=$2`, customer, channel).Scan(&v.State, &recipient, pq.Array(&v.Preferences.Networks), pq.Array(&v.Preferences.Topics), &v.Preferences.Cadence, &v.Preferences.Timezone, &v.Preferences.QuietStart, &v.Preferences.QuietEnd, &inbound)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return nil, e
		}
		if recipient.Valid {
			r := []rune(recipient.String)
			if len(r) > 4 {
				v.RecipientMask = "••••" + string(r[len(r)-4:])
			} else {
				v.RecipientMask = "••••"
			}
		}
		if inbound.Valid {
			v.LastInbound = &inbound.Time
		}
		result = append(result, v)
	}
	return result, nil
}

type Pairing struct {
	Code      string    `json:"code"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	Command   string    `json:"command"`
}

func (s *Service) Pair(ctx context.Context, customer, channel string) (Pairing, error) {
	if !validChannel(channel) || customer == "" || len(customer) > 256 {
		return Pairing{}, ErrPairInvalid
	}
	if !s.Config.Ready(channel) {
		return Pairing{}, ErrChannelUnavailable
	}
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		return Pairing{}, errors.New("pairing_generation_failed")
	}
	code := base64.RawURLEncoding.EncodeToString(b)
	expires := time.Now().UTC().Add(10 * time.Minute)
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Pairing{}, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `INSERT INTO crypto_brief_subscriptions(customer_sub,channel) VALUES($1,$2) ON CONFLICT DO NOTHING`, customer, channel)
	if e != nil {
		return Pairing{}, e
	}
	var state string
	e = tx.QueryRowContext(ctx, `SELECT state FROM crypto_brief_subscriptions WHERE customer_sub=$1 AND channel=$2 FOR UPDATE`, customer, channel).Scan(&state)
	if e != nil {
		return Pairing{}, e
	}
	if state != "disconnected" {
		return Pairing{}, errors.New("disconnect_before_pairing")
	}
	var recent bool
	e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM crypto_brief_pairings WHERE customer_sub=$1 AND channel=$2 AND created_at>clock_timestamp()-interval '1 minute')`, customer, channel).Scan(&recent)
	if e != nil {
		return Pairing{}, e
	}
	if recent {
		return Pairing{}, ErrPairRate
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO crypto_brief_pairings(customer_sub,channel,token_hash,expires_at) VALUES($1,$2,$3,$4) ON CONFLICT(customer_sub,channel) DO UPDATE SET token_hash=EXCLUDED.token_hash,expires_at=EXCLUDED.expires_at,created_at=clock_timestamp()`, customer, channel, hash(code), expires)
	if e != nil {
		return Pairing{}, e
	}
	if e = tx.Commit(); e != nil {
		return Pairing{}, e
	}
	p := Pairing{Code: code, ExpiresAt: expires, Command: "/start " + code, URL: "https://t.me/" + s.Config.TelegramUsername + "?start=" + code}
	if channel == "whatsapp" {
		p.Command = "BAGLA " + code
		p.URL = "https://wa.me/" + s.Config.WhatsAppNumber + "?text=" + url.QueryEscape(p.Command)
	}
	return p, nil
}
func (s *Service) ChangeState(ctx context.Context, customer, channel, state string) error {
	if !validChannel(channel) || !contains([]string{"paused", "active", "disconnected"}, state) {
		return errors.New("state_invalid")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var id int64
	var recipient sql.NullString
	e = tx.QueryRowContext(ctx, `SELECT id,recipient FROM crypto_brief_subscriptions WHERE customer_sub=$1 AND channel=$2 FOR UPDATE`, customer, channel).Scan(&id, &recipient)
	if errors.Is(e, sql.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	if state == "active" && !recipient.Valid {
		return errors.New("channel_not_connected")
	}
	if state == "disconnected" {
		_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET state='disconnected',recipient=NULL,consent_at=NULL,last_inbound_at=NULL,updated_at=clock_timestamp() WHERE id=$1`, id)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `DELETE FROM crypto_brief_pairings WHERE customer_sub=$1 AND channel=$2`, customer, channel)
	} else {
		_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET state=$2,updated_at=clock_timestamp() WHERE id=$1`, id, state)
	}
	if e != nil {
		return e
	}
	if state != "active" {
		_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET state='cancelled',body='Bildirim iptal edildi.',reason='customer_opt_out',updated_at=clock_timestamp() WHERE subscription_id=$1 AND state='pending'`, id)
		if e != nil {
			return e
		}
	}
	if state == "disconnected" {
		_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET body='Müşteri bağlantısı kaldırıldı.' WHERE subscription_id=$1`, id)
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func recentItems(ctx context.Context, q queryer, since time.Time, networks, topics []string, limit int) ([]Item, error) {
	if networks == nil {
		networks = []string{}
	}
	if topics == nil {
		topics = []string{}
	}
	if limit < 1 || limit > 40 {
		limit = 40
	}
	rows, e := q.QueryContext(ctx, `SELECT id,source_id,title,url,published_at,networks,topics FROM crypto_brief_items WHERE published_at>$1 AND published_at<=clock_timestamp()+interval '5 minutes' AND (cardinality($2::text[])=0 OR networks && $2::text[]) AND (cardinality($3::text[])=0 OR topics && $3::text[]) ORDER BY published_at DESC,id LIMIT $4`, since, pq.Array(networks), pq.Array(topics), limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []Item{}
	for rows.Next() {
		var v Item
		if e = rows.Scan(&v.ID, &v.SourceID, &v.Title, &v.URL, &v.PublishedAt, pq.Array(&v.Networks), pq.Array(&v.Topics)); e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s *Service) Recent(ctx context.Context, networks, topics []string) ([]Item, error) {
	return recentItems(ctx, s.DB, time.Now().Add(-7*24*time.Hour), networks, topics, 40)
}
func (s *Service) RefreshFeeds(ctx context.Context) (int, error) {
	n := 0
	failed := false
	now := time.Now().UTC()
	for _, source := range Sources {
		var claimed string
		e := s.DB.QueryRowContext(ctx, `INSERT INTO crypto_brief_sources(source_id,checked_at) VALUES($1,clock_timestamp()) ON CONFLICT(source_id) DO UPDATE SET checked_at=clock_timestamp() WHERE crypto_brief_sources.checked_at IS NULL OR crypto_brief_sources.checked_at<clock_timestamp()-interval '30 minutes' RETURNING source_id`, source.ID).Scan(&claimed)
		if errors.Is(e, sql.ErrNoRows) {
			continue
		}
		if e != nil {
			return n, e
		}
		items, e := FetchFeed(ctx, s.Client, source, now)
		if e != nil {
			failed = true
			log.Printf("crypto brief source unavailable source=%s code=%s", source.ID, e.Error())
			_, _ = s.DB.ExecContext(ctx, `UPDATE crypto_brief_sources SET error_code=$2 WHERE source_id=$1`, source.ID, e.Error())
			continue
		}
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return n, e
		}
		for _, item := range items {
			res, e := tx.ExecContext(ctx, `INSERT INTO crypto_brief_items(id,source_id,title,url,published_at,networks,topics) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, item.ID, item.SourceID, item.Title, item.URL, item.PublishedAt, pq.Array(item.Networks), pq.Array(item.Topics))
			if e != nil {
				_ = tx.Rollback()
				return n, e
			}
			rows, e := res.RowsAffected()
			if e != nil {
				_ = tx.Rollback()
				return n, e
			}
			n += int(rows)
		}
		_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_sources SET success_at=clock_timestamp(),error_code='',item_count=$2 WHERE source_id=$1`, source.ID, len(items))
		if e != nil {
			_ = tx.Rollback()
			return n, e
		}
		if e = tx.Commit(); e != nil {
			return n, e
		}
		log.Printf("crypto brief source synchronized source=%s items=%d", source.ID, len(items))
	}
	if failed {
		return n, errors.New("news_source_unavailable")
	}
	var degraded bool
	if e := s.DB.QueryRowContext(ctx, `SELECT count(*)<3 OR coalesce(bool_or(error_code!='' OR success_at IS NULL OR success_at<clock_timestamp()-interval '1 hour'),true) FROM crypto_brief_sources WHERE source_id IN ('cointelegraph','ethereum-foundation','solana-status')`).Scan(&degraded); e != nil {
		return n, e
	}
	if degraded {
		return n, errors.New("news_source_stale")
	}
	return n, nil
}
func (s *Service) ScheduleOne(ctx context.Context, now time.Time) (bool, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	var id int64
	var p Preferences
	var since time.Time
	e = tx.QueryRowContext(ctx, `SELECT id,networks,topics,cadence,timezone,quiet_start,quiet_end,last_digest_at FROM crypto_brief_subscriptions WHERE state='active' AND next_digest_at<=clock_timestamp() ORDER BY next_digest_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, pq.Array(&p.Networks), pq.Array(&p.Topics), &p.Cadence, &p.Timezone, &p.QuietStart, &p.QuietEnd, &since)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if Quiet(now, p) {
		next := now.Add(time.Hour)
		for i := 0; i < 24 && Quiet(next, p); i++ {
			next = next.Add(time.Hour)
		}
		_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET next_digest_at=$2 WHERE id=$1`, id, next)
		if e != nil {
			return true, e
		}
		return true, tx.Commit()
	}
	// A per-customer item ledger prevents repeated headlines and includes late RSS entries.
	rows, e := tx.QueryContext(ctx, `SELECT i.id,i.source_id,i.title,i.url,i.published_at,i.networks,i.topics FROM crypto_brief_items i WHERE i.published_at>$2 AND i.published_at<=clock_timestamp()+interval '5 minutes' AND (cardinality($3::text[])=0 OR i.networks && $3::text[]) AND (cardinality($4::text[])=0 OR i.topics && $4::text[]) AND NOT EXISTS(SELECT 1 FROM crypto_brief_seen z WHERE z.subscription_id=$1 AND z.item_id=i.id) ORDER BY i.published_at DESC,i.id LIMIT 5`, id, now.Add(-48*time.Hour), pq.Array(p.Networks), pq.Array(p.Topics))
	if e != nil {
		return true, e
	}
	items := []Item{}
	for rows.Next() {
		var item Item
		e = rows.Scan(&item.ID, &item.SourceID, &item.Title, &item.URL, &item.PublishedAt, pq.Array(&item.Networks), pq.Array(&item.Topics))
		if e != nil {
			_ = rows.Close()
			return true, e
		}
		items = append(items, item)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return true, e
	}
	body := Digest(items, now)
	if body != "" {
		_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET state='cancelled',reason='digest_superseded',body='Daha güncel özet hazırlandı.' WHERE subscription_id=$1 AND state='pending' AND dedup_key LIKE 'digest:%'`, id)
		if e != nil {
			return true, e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO crypto_brief_deliveries(subscription_id,dedup_key,body) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id, "digest:"+since.UTC().Format(time.RFC3339Nano), body)
		if e != nil {
			return true, e
		}
		for _, item := range items {
			_, e = tx.ExecContext(ctx, `INSERT INTO crypto_brief_seen(subscription_id,item_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, item.ID)
			if e != nil {
				return true, e
			}
		}
	}
	_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET next_digest_at=$2,last_digest_at=$3 WHERE id=$1`, id, NextDigest(now, p), now)
	if e != nil {
		return true, e
	}
	if e = tx.Commit(); e != nil {
		return true, e
	}
	workerwake.Signal(wakeID)
	return true, nil
}
func maxTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}

// Inbound is called only after channel authentication and payload validation.
func (s *Service) Inbound(ctx context.Context, channel, event, recipient, text string) error {
	if !validChannel(channel) || event == "" || len(event) > 256 || !digits.MatchString(recipient) || len(text) > 512 {
		return errors.New("inbound_invalid")
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return nil
	}
	command := strings.ToLower(strings.Split(fields[0], "@")[0])
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	res, e := tx.ExecContext(ctx, `INSERT INTO crypto_brief_inbound(channel,event_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, channel, event)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return nil
	}
	var id int64
	var state string
	var customer string
	var p Preferences
	reply := ""
	if (command == "/start" || command == "bagla" || command == "bağla") && len(fields) == 2 {
		if len(fields[1]) != 32 {
			return ErrPairInvalid
		}
		e = tx.QueryRowContext(ctx, `SELECT customer_sub FROM crypto_brief_pairings WHERE channel=$1 AND token_hash=$2 AND expires_at>clock_timestamp() FOR UPDATE`, channel, hash(fields[1])).Scan(&customer)
		if errors.Is(e, sql.ErrNoRows) {
			return ErrPairInvalid
		}
		if e != nil {
			return e
		}
		e = tx.QueryRowContext(ctx, `UPDATE crypto_brief_subscriptions SET recipient=$3,state='active',consent_at=clock_timestamp(),last_inbound_at=clock_timestamp(),updated_at=clock_timestamp() WHERE customer_sub=$1 AND channel=$2 AND state='disconnected' RETURNING id,state`, customer, channel, recipient).Scan(&id, &state)
		if e != nil {
			return ErrPairInvalid
		}
		_, e = tx.ExecContext(ctx, `DELETE FROM crypto_brief_pairings WHERE customer_sub=$1 AND channel=$2`, customer, channel)
		if e != nil {
			return e
		}
		reply = "Koschei Kripto Gündemi bağlantınız hazır. Tercihlerinize göre özet alacaksınız.\n/gundem: son haberler\n/dur: durdur · /devam: aç · /sil: bağlantıyı kaldır\nAyarlar: https://tradepigloball.co/crypto-brief"
	} else {
		e = tx.QueryRowContext(ctx, `SELECT id,state,networks,topics,cadence,timezone,quiet_start,quiet_end FROM crypto_brief_subscriptions WHERE channel=$1 AND recipient=$2 FOR UPDATE`, channel, recipient).Scan(&id, &state, pq.Array(&p.Networks), pq.Array(&p.Topics), &p.Cadence, &p.Timezone, &p.QuietStart, &p.QuietEnd)
		if errors.Is(e, sql.ErrNoRows) {
			return tx.Commit()
		}
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET last_inbound_at=clock_timestamp() WHERE id=$1`, id)
		if e != nil {
			return e
		}
		switch command {
		case "/dur", "dur", "stop", "/stop":
			state = "paused"
		case "/devam", "devam", "start":
			state = "active"
			reply = "Bildirimler açıldı."
		case "/sil", "sil", "delete", "/delete":
			state = "disconnected"
		case "/gundem", "gundem", "gündem":
			if state == "active" {
				items, e := recentItems(ctx, tx, time.Now().Add(-48*time.Hour), p.Networks, p.Topics, 5)
				if e != nil {
					return e
				}
				reply = Digest(items, time.Now())
				if reply == "" {
					reply = "Seçtiğiniz konular için son 48 saatte kayıtlı haber yok. https://tradepigloball.co/crypto-brief"
				}
			}
		default:
			if state == "active" {
				reply = "Komutlar: /gundem · /dur · /devam · /sil\nTercihler: https://tradepigloball.co/crypto-brief"
			}
		}
		if state == "disconnected" {
			_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET state='disconnected',recipient=NULL,consent_at=NULL,last_inbound_at=NULL,updated_at=clock_timestamp() WHERE id=$1`, id)
		} else {
			_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_subscriptions SET state=$2,updated_at=clock_timestamp() WHERE id=$1`, id, state)
		}
		if e != nil {
			return e
		}
		if state != "active" {
			_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET state='cancelled',body='Bildirim iptal edildi.',reason='customer_opt_out' WHERE subscription_id=$1 AND state='pending'`, id)
			if e != nil {
				return e
			}
		}
		if state == "disconnected" {
			_, e = tx.ExecContext(ctx, `UPDATE crypto_brief_deliveries SET body='Müşteri bağlantısı kaldırıldı.' WHERE subscription_id=$1`, id)
			if e != nil {
				return e
			}
		}
	}
	if reply != "" && state == "active" {
		// Cap command replies to one per minute per connected customer (digest cadence is separate).
		_, e = tx.ExecContext(ctx, `INSERT INTO crypto_brief_deliveries(subscription_id,dedup_key,body) SELECT $1,$2,$3 WHERE NOT EXISTS(SELECT 1 FROM crypto_brief_deliveries WHERE subscription_id=$1 AND dedup_key LIKE 'reply:%' AND created_at>clock_timestamp()-interval '1 minute') ON CONFLICT DO NOTHING`, id, "reply:"+event, reply)
		if e != nil {
			return e
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	workerwake.Signal(wakeID)
	return nil
}
func (s *Service) Status(ctx context.Context) (map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT source_id,checked_at,success_at,error_code,item_count FROM crypto_brief_sources ORDER BY source_id`)
	if e != nil {
		return nil, e
	}
	sources := []map[string]any{}
	for rows.Next() {
		var id, code string
		var checked, success sql.NullTime
		var n int
		if e = rows.Scan(&id, &checked, &success, &code, &n); e != nil {
			_ = rows.Close()
			return nil, e
		}
		v := map[string]any{"id": id, "error_code": code, "item_count": n, "fresh": success.Valid && time.Since(success.Time) < time.Hour}
		if checked.Valid {
			v["checked_at"] = checked.Time
		}
		if success.Valid {
			v["success_at"] = success.Time
		}
		sources = append(sources, v)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return nil, e
	}
	counts := map[string]int{}
	rows, e = s.DB.QueryContext(ctx, `SELECT state,count(*) FROM crypto_brief_deliveries GROUP BY state`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var n int
		if e = rows.Scan(&state, &n); e != nil {
			return nil, e
		}
		counts[state] = n
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	return map[string]any{"schema_version": "koschei.crypto-brief.v1", "enabled": s.Config.Enabled, "sources": sources, "deliveries": counts, "telegram_configured": s.Config.Ready("telegram"), "whatsapp_configured": s.Config.Ready("whatsapp"), "whatsapp_template_configured": s.Config.TemplateReady(), "evidence_kind": "publisher_report", "telegram_delivery_receipts": false}, nil
}

// Bound metadata retention. Recipient data is erased immediately on disconnect.
func (s *Service) Cleanup(ctx context.Context) error {
	for _, q := range []string{
		`DELETE FROM crypto_brief_deliveries WHERE id IN (SELECT id FROM crypto_brief_deliveries WHERE created_at<clock_timestamp()-interval '30 days' AND state!='sending' LIMIT 500)`,
		`DELETE FROM crypto_brief_inbound WHERE (channel,event_id) IN (SELECT channel,event_id FROM crypto_brief_inbound WHERE received_at<clock_timestamp()-interval '30 days' LIMIT 500)`,
		`DELETE FROM crypto_brief_pairings WHERE (customer_sub,channel) IN (SELECT customer_sub,channel FROM crypto_brief_pairings WHERE expires_at<clock_timestamp()-interval '1 day' LIMIT 500)`,
		`DELETE FROM crypto_brief_items WHERE id IN (SELECT id FROM crypto_brief_items WHERE published_at<clock_timestamp()-interval '14 days' LIMIT 500)`,
	} {
		if _, e := s.DB.ExecContext(ctx, q); e != nil {
			return e
		}
	}
	return nil
}
