package services

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRedactProviderCredentials(t *testing.T) {
	secret := "SWQjVSBiJMcny3J4yry5T"
	input := `Post "https://solana-mainnet.g.alchemy.com/v2/` + secret + `": context deadline exceeded`
	got := safeProviderError(errors.New(input))
	if strings.Contains(got, secret) {
		t.Fatalf("provider secret was not redacted: %s", got)
	}
	if !strings.Contains(got, "/v2/[redacted]") {
		t.Fatalf("redacted provider URL missing marker: %s", got)
	}

	querySecret := "another-secret"
	query := redactProviderCredentials("https://example.test/rpc?api_key=" + querySecret + "&network=mainnet")
	if strings.Contains(query, querySecret) || !strings.Contains(query, "api_key=[redacted]") {
		t.Fatalf("query credential was not redacted: %s", query)
	}
}

func TestRadarReconnectWaitAndRateLimitDetection(t *testing.T) {
	if !isRadarRateLimitError(errors.New("HTTP/1.1 429 Too Many Requests")) {
		t.Fatal("429 response should be detected as a rate-limit error")
	}
	if isRadarRateLimitError(errors.New("connection reset by peer")) {
		t.Fatal("non-rate-limit error was misclassified")
	}

	base := 30 * time.Second
	wait := radarReconnectWait(base)
	if wait < base || wait > base+base/5 {
		t.Fatalf("unexpected jittered reconnect delay: %s", wait)
	}
}

func TestClassifyRadarStreamTextByProgramID(t *testing.T) {
	module, eventType, programID := classifyRadarStreamText("Program " + strings.ToLower(defaultPumpProgramID) + " invoke [1]")
	if module != ModulePumpSybilRadar || eventType != "pump_launch_or_trade" || programID != defaultPumpProgramID {
		t.Fatalf("pump program was not classified correctly: %s %s %s", module, eventType, programID)
	}

	module, eventType, programID = classifyRadarStreamText("Program " + strings.ToLower(defaultRaydiumProgramID) + " invoke [1]")
	if module != ModuleRaydiumPoolGuardian || eventType != "raydium_pool_or_liquidity" || programID != defaultRaydiumProgramID {
		t.Fatalf("raydium program was not classified correctly: %s %s %s", module, eventType, programID)
	}
}
