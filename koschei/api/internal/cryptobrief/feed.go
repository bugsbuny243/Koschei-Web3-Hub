// Package cryptobrief delivers source-labelled public news to consenting customers.
// Publisher reports are not Koschei security verdicts or investment recommendations.
package cryptobrief

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"koschei/api/internal/outboundhttp"
)

type Source struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	FeedURL string `json:"feed_url"`
	Host    string `json:"-"`
	Network string `json:"-"`
	Topic   string `json:"-"`
}

var Sources = []Source{
	{"cointelegraph", "Cointelegraph", "https://cointelegraph.com/rss", "cointelegraph.com", "", ""},
	{"ethereum-foundation", "Ethereum Foundation", "https://blog.ethereum.org/feed.xml", "blog.ethereum.org", "ethereum", "network"},
	{"solana-status", "Solana Status", "https://status.solana.com/history.atom", "status.solana.com", "solana", "network"},
}

type Item struct {
	ID          string    `json:"id"`
	SourceID    string    `json:"source_id"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	Networks    []string  `json:"networks"`
	Topics      []string  `json:"topics"`
}
type feedItem struct {
	Title  string `xml:"title"`
	Link   string `xml:"link"`
	Date   string `xml:"pubDate"`
	DCDate string `xml:"date"`
}
type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}
type atomItem struct {
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
}
type document struct {
	XMLName xml.Name
	Channel struct {
		Items []feedItem `xml:"item"`
	} `xml:"channel"`
	Entries []atomItem `xml:"entry"`
}

var tags = regexp.MustCompile(`<[^>]*>`)

func cleanText(s string, n int) string {
	s = html.UnescapeString(tags.ReplaceAllString(s, " "))
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || r == 0x202e || r == 0x202d || r == 0x2066 || r == 0x2067 || r == 0x2068 || r == 0x2069
	}), " ")
	r := []rune(s)
	if len(r) > n {
		s = string(r[:n])
	}
	return s
}
func hash(s string) string { d := sha256.Sum256([]byte(s)); return hex.EncodeToString(d[:]) }
func publisherURL(raw string, source Source) (string, bool) {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || len(raw) > 2048 {
		return "", false
	}
	if !strings.EqualFold(u.Hostname(), source.Host) {
		return "", false
	}
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "utm_") {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), true
}
func parseDate(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822, "Mon, 2 Jan 2006 15:04:05 -0700", "2006-01-02T15:04:05Z07:00"} {
		if t, e := time.Parse(layout, strings.TrimSpace(s)); e == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
func classify(title string, source Source) ([]string, []string) {
	low := strings.ToLower(title)
	networks := []string{}
	topics := []string{}
	for _, p := range []struct {
		value string
		words []string
	}{{"bitcoin", []string{"bitcoin", "btc"}}, {"ethereum", []string{"ethereum", "ether", "erc-", "layer 2", "layer-2"}}, {"solana", []string{"solana", "sol "}}} {
		for _, w := range p.words {
			if strings.Contains(low, w) {
				networks = append(networks, p.value)
				break
			}
		}
	}
	if source.Network != "" && !contains(networks, source.Network) {
		networks = append(networks, source.Network)
	}
	if len(networks) == 0 {
		networks = []string{"global"}
	}
	for _, p := range []struct {
		value string
		words []string
	}{{"security", []string{"hack", "exploit", "phishing", "breach", "vulnerability", "scam", "drain"}}, {"defi", []string{"defi", "dex", "lending", "liquidity", "staking"}}, {"regulation", []string{"regulat", "sec ", "legislation", "law ", "court", "ban "}}, {"market", []string{"price", "market", "etf", "trading", "rally", "sell-off"}}, {"network", []string{"upgrade", "outage", "incident", "mainnet", "fork", "validator"}}} {
		for _, w := range p.words {
			if strings.Contains(low, w) {
				topics = append(topics, p.value)
				break
			}
		}
	}
	if source.Topic != "" && !contains(topics, source.Topic) {
		topics = append(topics, source.Topic)
	}
	if len(topics) == 0 {
		topics = []string{"general"}
	}
	return networks, topics
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
func ParseFeed(data []byte, source Source, now time.Time) ([]Item, error) {
	if len(data) > 2<<20 {
		return nil, errors.New("feed_too_large")
	}
	var doc document
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, errors.New("feed_invalid_xml")
	}
	if doc.XMLName.Local != "rss" && doc.XMLName.Local != "feed" {
		return nil, errors.New("feed_unknown_format")
	}
	items := []Item{}
	seen := map[string]bool{}
	add := func(title, raw, date string) {
		t, ok := parseDate(date)
		if !ok || t.After(now.Add(5*time.Minute)) || t.Before(now.Add(-7*24*time.Hour)) {
			return
		}
		u, ok := publisherURL(raw, source)
		title = cleanText(title, 240)
		if !ok || title == "" {
			return
		}
		id := hash(source.ID + "\n" + u)
		if seen[id] {
			return
		}
		seen[id] = true
		n, p := classify(title, source)
		items = append(items, Item{id, source.ID, title, u, t, n, p})
	}
	for _, v := range doc.Channel.Items {
		d := v.Date
		if d == "" {
			d = v.DCDate
		}
		add(v.Title, v.Link, d)
	}
	for _, v := range doc.Entries {
		u := ""
		for _, l := range v.Links {
			if l.Rel == "" || l.Rel == "alternate" {
				u = l.Href
				break
			}
		}
		d := v.Published
		if source.ID == "solana-status" && v.Updated != "" {
			d = v.Updated
		}
		if d == "" {
			d = v.Updated
		}
		before := len(items)
		add(v.Title, u, d)
		// Status incident updates (including resolution) have the same incident URL.
		if source.ID == "solana-status" && len(items) > before {
			items[len(items)-1].ID = hash(source.ID + "\n" + items[len(items)-1].URL + "\n" + items[len(items)-1].PublishedAt.Format(time.RFC3339Nano))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].PublishedAt.After(items[j].PublishedAt) })
	if len(items) > 40 {
		items = items[:40]
	}
	return items, nil
}
func FetchFeed(ctx context.Context, client *http.Client, source Source, now time.Time) ([]Item, error) {
	req, err := outboundhttp.NewRequest(ctx, http.MethodGet, source.FeedURL, nil)
	if err != nil {
		return nil, errors.New("feed_request_invalid")
	}
	req.Header.Set("User-Agent", "KoscheiCryptoBrief/1.0 (+https://tradepigloball.co/crypto-brief)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml")
	res, err := outboundhttp.Do(client, req)
	if err != nil {
		return nil, errors.New("feed_unreachable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("feed_http_rejected")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if err != nil {
		return nil, errors.New("feed_read_failed")
	}
	return ParseFeed(b, source, now)
}
func Matches(item Item, networks, topics []string) bool {
	match := func(wanted, actual []string) bool {
		if len(wanted) == 0 {
			return true
		}
		for _, w := range wanted {
			if contains(actual, w) {
				return true
			}
		}
		return false
	}
	return match(networks, item.Networks) && match(topics, item.Topics)
}
func Digest(items []Item, now time.Time) string {
	if len(items) == 0 {
		return ""
	}
	s := "Koschei · Kripto Gündemi\n" + now.In(istanbul()).Format("02.01.2006 15:04") + " (İstanbul)\n"
	count := 0
	for _, item := range items {
		name := item.SourceID
		for _, source := range Sources {
			if source.ID == item.SourceID {
				name = source.Name
			}
		}
		line := "\n• " + item.Title + "\n" + name + " · " + item.PublishedAt.In(istanbul()).Format("02.01 15:04") + "\n" + item.URL + "\n"
		if len([]rune(s+line)) > 3000 {
			break
		}
		s += line
		count++
		if count == 5 {
			break
		}
	}
	s += "\nKaynak haberidir; Koschei risk kararı değildir. Ayarlar: https://tradepigloball.co/crypto-brief\nDurdur: /dur · Devam: /devam · Bağlantıyı kaldır: /sil"
	return s
}
func istanbul() *time.Location {
	loc, e := time.LoadLocation("Europe/Istanbul")
	if e != nil {
		return time.FixedZone("Europe/Istanbul", 3*60*60)
	}
	return loc
}
