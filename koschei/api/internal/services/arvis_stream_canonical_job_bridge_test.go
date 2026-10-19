package services

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestArvisStreamCanonicalJobBridgeFailsClosedWithoutSourceAndTrust(t *testing.T) {
	actor := ActorDefenseRuleVerdict{}
	behavior := UnifiedRadarBehaviorReport{}
	if _, err := PersistArvisStreamCanonicalJobFinal(context.Background(), nil, "event", "mint", "solana-mainnet", actor, behavior, "rpc"); err == nil {
		t.Fatal("nil database was accepted")
	}
	db, err := sql.Open("postgres", "postgres://postgres:postgres@127.0.0.1:5432/unused?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Setenv(arvisStreamTrustedKeysEnv, "")
	if _, err := PersistArvisStreamCanonicalJobFinal(context.Background(), db, "test", "mint", "solana-mainnet", actor, behavior, "rpc"); err == nil || !strings.Contains(err.Error(), "trusted operator public keys") {
		t.Fatalf("untrusted canonical job source was accepted: %v", err)
	}
	for _, tc := range []struct{ event, target, network string }{
		{"", "mint", "solana-mainnet"},
		{"source", "", "solana-mainnet"},
		{"source", "mint", "ethereum"},
	} {
		if _, err := PersistArvisStreamCanonicalJobFinal(context.Background(), db, tc.event, tc.target, tc.network, actor, behavior, "rpc"); err == nil {
			t.Fatalf("missing/cross-chain source identity was accepted: %#v", tc)
		}
	}
}
