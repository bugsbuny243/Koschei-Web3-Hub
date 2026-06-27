package services

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type solanaFailoverTransport struct {
	base http.RoundTripper
}

func init() {
	base := solanaRPCClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	solanaRPCClient.Transport = &solanaFailoverTransport{base: base}
}

func (t *solanaFailoverTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if !solanaFailoverEnabled() || !solanaFailoverRequired(resp, err) {
		return resp, err
	}
	if strings.EqualFold(req.URL.Hostname(), "api.mainnet-beta.solana.com") || req.GetBody == nil {
		return resp, err
	}
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 65536))
		_ = resp.Body.Close()
	}
	body, bodyErr := req.GetBody()
	if bodyErr != nil {
		return nil, bodyErr
	}
	fallbackURL, parseErr := url.Parse(defaultSolanaMainnetRPC)
	if parseErr != nil {
		return nil, parseErr
	}
	fallbackReq := req.Clone(req.Context())
	fallbackReq.URL = fallbackURL
	fallbackReq.Host = ""
	fallbackReq.RequestURI = ""
	fallbackReq.Body = body
	fallbackReq.GetBody = req.GetBody
	return t.base.RoundTrip(fallbackReq)
}

func solanaFailoverEnabled() bool {
	if raw := strings.TrimSpace(os.Getenv("SOLANA_RPC_FAILOVER_ENABLED")); raw != "" {
		enabled, parseErr := strconv.ParseBool(raw)
		return parseErr == nil && enabled
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

func solanaFailoverRequired(resp *http.Response, err error) bool {
	if err != nil || resp == nil {
		return true
	}
	switch resp.StatusCode {
	case http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
