package handlers

import (
	"context"
	"strings"
	"testing"

	"koschei/api/internal/jobs"
)

func TestWeb3JobTypeFromPathExposesCanonicalCustomerRoute(t *testing.T) {
	cases := map[string]string{
		"/api/v1/radar/jobs":     CanonicalInvestigationJobType,
		"/api/jobs/token-scan":   legacyTokenScanJobType,
		"/api/jobs/wallet-score": "wallet_score",
		"/api/jobs/tx-decode":    "tx_decode",
	}
	for path, expected := range cases {
		if got := web3JobTypeFromPath(path); got != expected {
			t.Fatalf("path %s resolved job type %q, want %q", path, got, expected)
		}
	}
	if got := web3JobTypeFromPath("/api/v1/radar/jobs/unknown"); got != "" {
		t.Fatalf("unknown job route resolved as %q", got)
	}
}

func TestCanonicalJobPollPathUsesLastSegment(t *testing.T) {
	cases := map[string]string{
		"/api/owner/radar/jobs/abc-123": "abc-123",
		"/api/v1/radar/jobs/def-456":    "def-456",
		"/api/jobs/ghi-789":             "ghi-789",
		"/api/owner/radar/jobs/":        "",
	}
	for path, expected := range cases {
		if got := lastCanonicalJobPathSegment(path); got != expected {
			t.Fatalf("path %s resolved id %q, want %q", path, got, expected)
		}
	}
}

func TestCanonicalHistoryCollectionPathIsIsolated(t *testing.T) {
	for _, path := range []string{"/api/v1/radar/jobs", "/api/v1/radar/jobs/", " /api/v1/radar/jobs/ "} {
		if !canonicalHistoryCollectionPath(path) {
			t.Fatalf("canonical history collection path rejected: %q", path)
		}
	}
	for _, path := range []string{"/api/jobs/", "/api/jobs", "/api/v1/radar/jobs/abc-123", "/api/owner/radar/jobs/"} {
		if canonicalHistoryCollectionPath(path) {
			t.Fatalf("non-canonical collection path accepted: %q", path)
		}
	}
}

func TestCanonicalPayloadMaxDepthIsBounded(t *testing.T) {
	if got := canonicalPayloadMaxDepth(canonicalInvestigationJobPayload{MaxDepth: 99}); got != 3 {
		t.Fatalf("recursive max depth escaped hard cap: %d", got)
	}
	if got := canonicalPayloadMaxDepth(canonicalInvestigationJobPayload{MaxDepth: 2}); got != 2 {
		t.Fatalf("explicit recursive depth lost: %d", got)
	}
}

func TestCustomerCanonicalWorkerRefusesWrongNetworkBeforeLoadingSolanaEvidence(t *testing.T) {
	// Native EVM and Bitcoin addresses are now supported on their own routes,
	// but no unavailable/mismatched target may fall back to the Solana engine.
	// A failed native collector must return an error, not an empty completed job.
	t.Setenv("ETHEREUM_RPC_URL", "")
	t.Setenv("BASE_RPC_URL", "")
	for _, tc := range []struct {
		network string
		target  string
		wantErr string
	}{
		{"ethereum-mainnet", "0x1111111111111111111111111111111111111111", "native EVM customer collector not configured"},
		{"base-mainnet", "0x1111111111111111111111111111111111111111", "native EVM customer collector not configured"},
		{"bitcoin-mainnet", "0x1111111111111111111111111111111111111111", "address_network_format_mismatch"},
		{"sui-mainnet", "0x1111111111111111111111111111111111111111", "address_network_format_mismatch"},
	} {
		t.Run(tc.network, func(t *testing.T) {
			worker := &canonicalInvestigationJobWorker{}
			report, err := worker.processJob(context.Background(), jobs.Job{
				Type: CanonicalInvestigationJobType, Network: tc.network,
				Target: tc.target,
			})
			if report != nil || err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("mismatched or unavailable native network must fail closed: report=%v err=%v", report, err)
			}
		})
	}
}
