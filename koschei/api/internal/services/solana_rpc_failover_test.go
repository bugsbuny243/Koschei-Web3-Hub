package services

import (
	"errors"
	"net/http"
	"testing"
)

func TestSolanaFailoverDefaultsOnInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_FAILOVER_ENABLED", "")
	if !solanaFailoverEnabled() {
		t.Fatal("production failover should be enabled by default")
	}
}

func TestSolanaFailoverCanBeDisabled(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SOLANA_RPC_FAILOVER_ENABLED", "false")
	if solanaFailoverEnabled() {
		t.Fatal("explicit false should disable failover")
	}
}

func TestSolanaFailoverRequiredForCapacityAndProviderErrors(t *testing.T) {
	for _, status := range []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		if !solanaFailoverRequired(&http.Response{StatusCode: status}, nil) {
			t.Fatalf("status %d should trigger failover", status)
		}
	}
	if !solanaFailoverRequired(nil, errors.New("provider unavailable")) {
		t.Fatal("transport errors should trigger failover")
	}
	if solanaFailoverRequired(&http.Response{StatusCode: http.StatusOK}, nil) {
		t.Fatal("successful response should not trigger failover")
	}
}
