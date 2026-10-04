package cryptobrief

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFeedProvenanceAndUntrustedInput(t *testing.T) {
	now := time.Date(2026, 10, 4, 15, 0, 0, 0, time.UTC)
	source := Sources[0]
	xml := `<rss><channel><item><title><![CDATA[<b>Bitcoin exploit</b> &amp; update]]></title><link>https://cointelegraph.com/news/real?utm_source=test</link><pubDate>Sun, 04 Oct 2026 14:00:00 +0000</pubDate></item><item><title>Duplicate</title><link>https://cointelegraph.com/news/real</link><pubDate>Sun, 04 Oct 2026 14:00:00 +0000</pubDate></item><item><title>Foreign publisher</title><link>https://evil.example/</link><pubDate>Sun, 04 Oct 2026 14:00:00 +0000</pubDate></item><item><title>Undated</title><link>https://cointelegraph.com/news/undated</link></item><item><title>Future</title><link>https://cointelegraph.com/news/future</link><pubDate>Mon, 05 Oct 2026 14:00:00 +0000</pubDate></item></channel></rss>`
	items, e := ParseFeed([]byte(xml), source, now)
	if e != nil || len(items) != 1 {
		t.Fatalf("parsed %d %v", len(items), e)
	}
	v := items[0]
	if v.Title != "Bitcoin exploit & update" || v.URL != "https://cointelegraph.com/news/real" || !contains(v.Topics, "security") || !contains(v.Networks, "bitcoin") {
		t.Fatalf("provenance: %+v", v)
	}
	for _, raw := range []string{"https://cointelegraph.com.evil.test/n", "https://user:pass@cointelegraph.com/n", "http://cointelegraph.com/n", "https://cointelegraph.com:444/n"} {
		if _, ok := publisherURL(raw, source); ok {
			t.Fatal("unsafe link accepted")
		}
	}
	if _, e = ParseFeed([]byte(`<rss><broken>`), source, now); e == nil {
		t.Fatal("broken XML accepted")
	}
	if _, e = ParseFeed([]byte(strings.Repeat("a", (2<<20)+1)), source, now); e == nil {
		t.Fatal("unbounded XML accepted")
	}
	atom := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Solana network incident</title><link rel="alternate" href="https://status.solana.com/incidents/fixture"/><updated>2026-10-04T14:00:00Z</updated></entry></feed>`
	items, e = ParseFeed([]byte(atom), Sources[2], now)
	if e != nil || len(items) != 1 || !Matches(items[0], []string{"solana"}, []string{"network"}) || Matches(items[0], []string{"bitcoin"}, nil) {
		t.Fatalf("atom/filter: %+v %v", items, e)
	}
	text := Digest(items, now)
	if !strings.Contains(text, "Solana Status") || !strings.Contains(text, items[0].URL) || !strings.Contains(text, "Koschei risk kararı değildir") {
		t.Fatal("missing provenance")
	}
}
func TestQuietHoursAndCadence(t *testing.T) {
	p := Preferences{Cadence: "daily", Timezone: "Europe/Istanbul", QuietStart: 23, QuietEnd: 8}
	night := time.Date(2026, 10, 4, 21, 0, 0, 0, time.UTC)
	if !Quiet(night, p) || Quiet(night.Add(9*time.Hour), p) {
		t.Fatal("Istanbul quiet hours")
	}
	next := NextDigest(night, p)
	if next.In(istanbul()).Hour() != 9 || Quiet(next, p) {
		t.Fatalf("next daily: %v", next)
	}
	p.Cadence = "hourly"
	if Quiet(NextDigest(night, p), p) {
		t.Fatal("hourly scheduled during quiet hours")
	}
	p.QuietStart = p.QuietEnd
	if Quiet(night, p) {
		t.Fatal("equal boundaries should disable quiet hours")
	}
	p.Networks = []string{"solana", "solana"}
	if validatePreferences(&p) == nil {
		t.Fatal("duplicate preferences accepted")
	}
}
func TestChannelWebhookAuthentication(t *testing.T) {
	s := &Service{Config: Config{Enabled: true, TelegramToken: "12345:fixture", TelegramSecret: strings.Repeat("x", 32), TelegramUsername: "FixtureBot"}}
	r := httptest.NewRequest("POST", "/integrations/crypto-brief/telegram", strings.NewReader(`{"update_id":1}`))
	w := httptest.NewRecorder()
	s.TelegramHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("missing secret: %d", w.Code)
	}
	r = httptest.NewRequest("POST", "/integrations/crypto-brief/telegram", strings.NewReader(`{"update_id":1,"message":{"date":1791126000,"from":{"id":12345},"chat":{"id":99999,"type":"private"},"text":"/dur"}}`))
	r.Header.Set("X-Telegram-Bot-Api-Secret-Token", s.Config.TelegramSecret)
	w = httptest.NewRecorder()
	s.TelegramHTTP(w, r)
	if w.Code != 200 {
		t.Fatal("foreign sender not ignored")
	}
	body := []byte(`{"fixture":true}`)
	mac := hmac.New(sha256.New, []byte("fixture-secret"))
	_, _ = mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !VerifyWhatsAppSignature(body, sig, "fixture-secret") || VerifyWhatsAppSignature([]byte(`{"tampered":true}`), sig, "fixture-secret") || VerifyWhatsAppSignature(body, sig, "") {
		t.Fatal("WhatsApp HMAC validation")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProviderReceiptsAndWhatsAppWindow(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	calls := 0
	var sent string
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		b, _ := io.ReadAll(r.Body)
		sent = string(b)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"messages":[{"id":"fixture-provider-id"}]}`)), Header: make(http.Header)}, nil
	})}
	c := Config{Enabled: true, WhatsAppToken: "fixture", WhatsAppPhoneID: "123456", WhatsAppSecret: strings.Repeat("x", 16), WhatsAppVerify: strings.Repeat("v", 32), WhatsAppVersion: "v23.0", WhatsAppNumber: "905551234567", WhatsAppLanguage: "tr"}
	p := Provider{c, client}
	r := p.Send(context.Background(), "whatsapp", "905551234568", "fixture headline", time.Now().Add(-25*time.Hour))
	if r.Reason != "whatsapp_template_required" || calls != 0 {
		t.Fatal("outside-window text sent")
	}
	r = p.Send(context.Background(), "whatsapp", "905551234568", "fixture headline", time.Now().Add(-time.Hour))
	if r.State != "accepted" || !strings.Contains(sent, `"type":"text"`) {
		t.Fatalf("inside window: %+v", r)
	}
	p.Config.WhatsAppTemplate = "fixture_brief"
	r = p.Send(context.Background(), "whatsapp", "905551234568", "fixture\nheadline", time.Now().Add(-25*time.Hour))
	if r.State != "accepted" || !strings.Contains(sent, `"type":"template"`) || strings.Contains(sent, `fixture\nheadline`) {
		t.Fatalf("template: %+v %s", r, sent)
	}
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 502, Body: io.NopCloser(strings.NewReader(`unavailable`)), Header: make(http.Header)}, nil
	})
	r = p.Send(context.Background(), "whatsapp", "905551234568", "fixture", time.Now())
	if r.State != "uncertain" {
		t.Fatal("ambiguous send auto-retried")
	}
}
