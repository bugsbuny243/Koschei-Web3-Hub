package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"koschei/api/internal/radaradapter"
	"koschei/api/internal/web3"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RadarProvider struct {
	RequestsPerSecond int               `json:"requests_per_second,omitempty"`
	DailyLimit        int               `json:"daily_limit,omitempty"`
	URL               string            `json:"url"`
	Headers           map[string]string `json:"headers,omitempty"`
}
type RadarNetworkProviders struct {
	DNS         RadarProvider `json:"dns"`
	Certificate RadarProvider `json:"certificate"`
	Capture     RadarProvider `json:"capture"`
	Primary     RadarProvider `json:"primary"`
	Secondary   RadarProvider `json:"secondary"`
	Indexed     RadarProvider `json:"indexed"`
	Explorer    RadarProvider `json:"explorer"`
	Genesis     string        `json:"genesis,omitempty"`
}

// RADAR_PROVIDERS_JSON is server-only. URLs, credentials and upstream errors never enter results.
func RadarProviderConfig() map[string]RadarNetworkProviders {
	out := map[string]RadarNetworkProviders{}
	if json.Unmarshal([]byte(os.Getenv("RADAR_PROVIDERS_JSON")), &out) != nil {
		return map[string]RadarNetworkProviders{}
	}
	return out
}

var radarHTTPClient = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

type radarTransport struct {
	mu        sync.Mutex
	count     int
	providers map[string]RadarNetworkProviders
}

func NewRadarTransport() *radarTransport { return &radarTransport{providers: RadarProviderConfig()} }
func (t *radarTransport) Count() int     { t.mu.Lock(); defer t.mu.Unlock(); return t.count }
func (t *radarTransport) Do(ctx context.Context, q radaradapter.Request) (radaradapter.Response, error) {
	if !radaradapter.ReadOnlyRequest(q) {
		return radaradapter.Response{}, errors.New("request outside read-only evidence contract")
	}
	cfg, ok := t.providers[q.Network]
	if !ok {
		return radaradapter.Response{}, errors.New("provider configuration required")
	}
	p := cfg.Primary
	switch q.Provider {
	case "", "primary":
	case "secondary":
		p = cfg.Secondary
	case "indexed":
		p = cfg.Indexed
	case "explorer":
		p = cfg.Explorer
	case "dns":
		p = cfg.DNS
	case "certificate":
		p = cfg.Certificate
	case "capture":
		p = cfg.Capture
	default:
		return radaradapter.Response{}, errors.New("unknown provider role")
	}
	parsed, e := url.Parse(p.URL)
	if e != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return radaradapter.Response{}, errors.New("HTTPS provider configuration required")
	}
	if q.Origin != "" && (q.Provider == "primary" || q.Provider == "secondary") && parsed.Hostname() != q.Origin {
		return radaradapter.Response{}, errors.New("configured origin mismatch")
	}
	if strings.HasPrefix(q.Path, "//") || strings.Contains(q.Path, "://") {
		return radaradapter.Response{}, errors.New("invalid provider path")
	}
	t.mu.Lock()
	if t.count >= 80 {
		t.mu.Unlock()
		return radaradapter.Response{}, errors.New("investigation request budget exhausted")
	}
	t.count++
	t.mu.Unlock()
	if e = reserveRadarProvider(ctx, parsed.Hostname(), p); e != nil {
		return radaradapter.Response{}, e
	}
	// Share the existing worker budget and provider-host cooldown/pacing chain.
	if e = reserveSolanaRPCBudget(ctx, "radar:"+q.Network); e != nil {
		return radaradapter.Response{}, errors.New("shared request budget exhausted")
	}
	if _, cooling := web3.SolanaRPCProviderCooldown(p.URL); cooling {
		return radaradapter.Response{}, errors.New("provider cooling down")
	}
	if e = web3.WaitForSolanaRPCProviderSlot(ctx, p.URL); e != nil {
		return radaradapter.Response{}, errors.New("provider pacing deadline")
	}
	endpoint := strings.TrimRight(p.URL, "/") + q.Path
	var body []byte
	if q.Body != nil {
		body, e = json.Marshal(q.Body)
		if e != nil {
			return radaradapter.Response{}, errors.New("request encoding failed")
		}
	}
	req, e := http.NewRequestWithContext(ctx, q.Method, endpoint, bytes.NewReader(body))
	if e != nil {
		return radaradapter.Response{}, errors.New("invalid provider request")
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	if (parsed.Hostname() == "api.trongrid.io" || strings.HasSuffix(parsed.Hostname(), ".trongrid.io")) && req.Header.Get("TRON-PRO-API-KEY") == "" {
		return radaradapter.Response{}, errors.New("Tron API key header configuration required")
	}
	res, e := radarHTTPClient.Do(req)
	if e != nil {
		web3.DeferSolanaRPCProvider(p.URL, 30*time.Second)
		return radaradapter.Response{}, errors.New("provider request unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if res.StatusCode == 429 || res.StatusCode == 503 || res.StatusCode == 403 {
			seconds, _ := strconv.Atoi(res.Header.Get("Retry-After"))
			if seconds < 30 {
				seconds = 30
			}
			if seconds > 3600 {
				seconds = 3600
			}
			web3.DeferSolanaRPCProvider(p.URL, time.Duration(seconds)*time.Second)
		}
		return radaradapter.Response{}, errors.New("provider rejected request")
	}
	raw, e := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if e != nil || len(raw) > 2*1024*1024 {
		return radaradapter.Response{}, errors.New("provider response too large or incomplete")
	}
	certificates := []string{}
	if res.TLS != nil {
		for _, cert := range res.TLS.PeerCertificates {
			certificates = append(certificates, radaradapter.Digest(cert.Raw))
		}
	}
	return radaradapter.Response{Raw: raw, Host: parsed.Hostname(), Certificates: certificates}, nil
}

func (t *radarTransport) Baseline(host, path string) string {
	var pinned map[string]string
	if json.Unmarshal([]byte(os.Getenv("RADAR_FRONTEND_BASELINES_JSON")), &pinned) != nil {
		return ""
	}
	digest := pinned["https://"+host+path]
	if len(digest) != 64 {
		return ""
	}
	return digest
}

// RadarCapabilityRuntime separates compiled/fixture-tested capability from configuration.
func RadarCapabilityRuntime() []map[string]any {
	cfg := RadarProviderConfig()
	out := []map[string]any{}
	for _, c := range RadarCapabilities() {
		if c.Family == "offchain" {
			origins := 0
			for key, p := range cfg {
				u, err := url.Parse(p.Primary.URL)
				if err == nil && u.Scheme == "https" && key == "offchain:"+u.Hostname() && u.Hostname() != "" {
					origins++
				}
			}
			out = append(out, map[string]any{"capability": c, "runtime_status": "per_origin_configuration", "configured_origin_count": origins, "live_verified": false})
			continue
		}
		p := cfg[c.Network]
		primary, _ := url.Parse(p.Primary.URL)
		secondary, _ := url.Parse(p.Secondary.URL)
		configured := primary != nil && primary.Scheme == "https" && primary.Hostname() != ""
		dual := configured && secondary != nil && secondary.Scheme == "https" && secondary.Hostname() != "" && primary.Hostname() != secondary.Hostname()
		status := "provider_configuration_required"
		if configured {
			status = "primary_configured"
		}
		if dual {
			status = "two_provider_hosts_configured"
		}
		out = append(out, map[string]any{"capability": c, "runtime_status": status, "live_verified": false})
	}
	return out
}

type radarProviderWindow struct {
	Next  time.Time
	Day   string
	Count int
}

var radarProviderWindows = struct {
	sync.Mutex
	Hosts map[string]radarProviderWindow
}{Hosts: map[string]radarProviderWindow{}}

func reserveRadarProvider(ctx context.Context, host string, p RadarProvider) error {
	rps := p.RequestsPerSecond
	if rps <= 0 {
		rps = 1
	}
	if rps > 100 {
		rps = 100
	}
	daily := p.DailyLimit
	if daily <= 0 {
		daily = 10000
	}
	for {
		now := time.Now().UTC()
		radarProviderWindows.Lock()
		s := radarProviderWindows.Hosts[host]
		if s.Day != now.Format("2006-01-02") {
			s.Day = now.Format("2006-01-02")
			s.Count = 0
		}
		if s.Count >= daily {
			radarProviderWindows.Unlock()
			return errors.New("provider daily request budget exhausted")
		}
		if !s.Next.After(now) {
			s.Next = now.Add(time.Second / time.Duration(rps))
			s.Count++
			radarProviderWindows.Hosts[host] = s
			radarProviderWindows.Unlock()
			return nil
		}
		delay := time.Until(s.Next)
		radarProviderWindows.Unlock()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.New("provider pacing deadline")
		case <-timer.C:
		}
	}
}
func (t *radarTransport) KnownSweeper(hash string) (string, bool) {
	var known map[string]string
	if json.Unmarshal([]byte(os.Getenv("RADAR_EVM_SWEEPER_HASHES_JSON")), &known) != nil {
		return "", false
	}
	source, ok := known[strings.ToLower(hash)]
	return source, ok && strings.HasPrefix(source, "https://")
}

func (t *radarTransport) CanonicalJetton(master string) bool {
	var registry map[string]string
	if json.Unmarshal([]byte(os.Getenv("RADAR_TON_JETTON_MASTERS_JSON")), &registry) != nil {
		return false
	}
	source := registry[master]
	return strings.HasPrefix(source, "https://")
}
