package web3

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var rpcURLPattern = regexp.MustCompile(`(?i)(?:https?|wss?)://[^\s\"']+`)
var rpcSecretPattern = regexp.MustCompile(`(?i)([?&](?:api[_-]?key|apikey|key|token)=)[^&\s\"']+`)

// RPCProviderHost returns only the provider hostname. Paths, credentials and
// query strings are deliberately discarded so startup and failure logs cannot
// expose API keys.
func RPCProviderHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "unconfigured"
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return "invalid-host"
	}
	return strings.ToLower(parsed.Hostname())
}

func RPCHTTPStatusClass(statusCode int) string {
	if statusCode <= 0 {
		return "none"
	}
	return strconv.Itoa(statusCode/100) + "xx"
}

// RPCFailureKind separates caller cancellation from upstream throttling and
// timeouts. A cancellation is not evidence that the provider failed.
func RPCFailureKind(statusCode int, err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if statusCode == 429 {
		return "throttled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if statusCode >= 500 {
		return "upstream_5xx"
	}
	if statusCode >= 400 {
		return "upstream_4xx"
	}
	return "rpc_error"
}

// LogRPCFailure emits provider-safe diagnostics. The raw endpoint is never
// logged; error strings are scrubbed of URLs and query credentials.
func LogRPCFailure(method, endpoint string, statusCode int, err error) {
	method = strings.TrimSpace(method)
	if method == "" {
		method = "unknown"
	}
	message := safeRPCLogError(err)
	if message == "" {
		message = "rpc request failed"
	}
	prefix := "solana rpc failure"
	if RPCFailureKind(statusCode, err) == "canceled" {
		prefix = "solana rpc canceled"
	}
	log.Printf(
		"%s kind=%s method=%s provider=%s http_class=%s status=%d error=%s",
		prefix,
		RPCFailureKind(statusCode, err),
		method,
		RPCProviderHost(endpoint),
		RPCHTTPStatusClass(statusCode),
		statusCode,
		message,
	)
}

func safeRPCLogError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	message = rpcURLPattern.ReplaceAllString(message, "[redacted-url]")
	message = rpcSecretPattern.ReplaceAllString(message, `${1}[redacted]`)
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 180 {
		message = message[:180]
	}
	return fmt.Sprintf("%q", message)
}
