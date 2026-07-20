package requestmeta

import (
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	maxForwardedHeaderBytes = 1024
	maxForwardedHops        = 32
)

// ClientIP returns the direct peer address unless that peer is explicitly
// configured as a trusted reverse proxy. Forwarded headers from untrusted peers
// are ignored so callers cannot forge rate-limit or audit identities.
func ClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}

	peerIP, peerText := remotePeerIP(r.RemoteAddr)
	if peerText == "" {
		peerText = "unknown"
	}
	trusted := configuredTrustedProxyNetworks()
	if peerIP == nil || !ipInNetworks(peerIP, trusted) {
		return peerText
	}

	forwarded, ok := parseForwardedFor(r.Header.Get("X-Forwarded-For"))
	if !ok || len(forwarded) == 0 {
		return peerText
	}

	// Walk from the application back toward the original client. Every trusted
	// proxy is stripped; the first untrusted hop is the client identity.
	chain := append(forwarded, peerIP)
	for i := len(chain) - 1; i >= 0; i-- {
		if ipInNetworks(chain[i], trusted) {
			continue
		}
		return chain[i].String()
	}
	return forwarded[0].String()
}

func remotePeerIP(remoteAddr string) (net.IP, string) {
	value := strings.TrimSpace(remoteAddr)
	if value == "" || len(value) > 128 {
		return nil, ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		host = strings.Trim(strings.TrimSpace(host), "[]")
		if ip := net.ParseIP(host); ip != nil {
			return ip, ip.String()
		}
		return nil, host
	}
	value = strings.Trim(value, "[]")
	if ip := net.ParseIP(value); ip != nil {
		return ip, ip.String()
	}
	return nil, value
}

func parseForwardedFor(raw string) ([]net.IP, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	if len(raw) > maxForwardedHeaderBytes {
		return nil, false
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxForwardedHops {
		return nil, false
	}
	out := make([]net.IP, 0, len(parts))
	for _, part := range parts {
		candidate := strings.Trim(strings.TrimSpace(part), "[]")
		ip := net.ParseIP(candidate)
		if ip == nil {
			return nil, false
		}
		out = append(out, ip)
	}
	return out, true
}

func configuredTrustedProxyNetworks() []*net.IPNet {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("KOSCHEI_TRUSTED_PROXY_CIDRS"))
	}
	if raw == "" {
		return nil
	}
	out := make([]*net.IPNet, 0)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil {
			out = append(out, network)
			continue
		}
		ip := net.ParseIP(strings.Trim(entry, "[]"))
		if ip == nil {
			continue
		}
		bits := 128
		if ip.To4() != nil {
			ip = ip.To4()
			bits = 32
		}
		out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return out
}

func ipInNetworks(ip net.IP, networks []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, network := range networks {
		if network != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}
