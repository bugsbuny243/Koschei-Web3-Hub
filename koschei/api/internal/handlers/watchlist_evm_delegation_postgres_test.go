package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// A watched EVM wallet is re-read through the production probe against an
// explicit local TLS fixture RPC: quiet while the code is empty, one critical
// alert when a delegation appears, rechecked about a minute later.
func TestWatchlistEVMDelegationRefreshPostgres17(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	var code atomic.Value
	code.Store("0x")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		result := "0x1"
		if req.Method == "eth_getCode" {
			result = code.Load().(string)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer server.Close()
	t.Setenv("ETHEREUM_RPC_URL", server.URL)
	previousClient := watchlistEVMHTTPClient
	watchlistEVMHTTPClient = server.Client()
	t.Cleanup(func() { watchlistEVMHTTPClient = previousClient })

	subject := "ci-fixture-evm-watch-" + time.Now().UTC().Format("150405.000000000")
	wallet := "0x28c6c06298d514db089934071355e5743bf21d60"
	var id string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO watchlist_targets (auth_subject,target,target_type,network,next_check_at)
		VALUES ($1,$2,'wallet','ethereum-mainnet',now()) RETURNING id::text`, subject, wallet).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM watchlist_targets WHERE auth_subject=$1`, subject) })
	h := &Handler{DB: db, DBRead: db}

	if _, alerts, err := h.refreshWatchlistTarget(ctx, subject, id); err != nil || alerts != 0 {
		t.Fatalf("empty code baseline: alerts=%d err=%v", alerts, err)
	}
	code.Store("0xef0100" + strings.TrimPrefix(ciFixtureDelegate, "0x"))
	target, alerts, err := h.refreshWatchlistTarget(ctx, subject, id)
	if err != nil || alerts != 1 {
		t.Fatalf("delegation must raise one alert: alerts=%d err=%v", alerts, err)
	}
	if target.LastSnapshot == nil || target.LastSnapshot.EVMDelegation == nil || target.LastSnapshot.EVMDelegation.DelegationTarget != ciFixtureDelegate {
		t.Fatalf("snapshot must keep the observed delegate: %+v", target.LastSnapshot)
	}
	var eventType, severity string
	if err := db.QueryRowContext(ctx, `SELECT event_type,severity FROM watchlist_alerts WHERE watchlist_id=$1`, id).Scan(&eventType, &severity); err != nil {
		t.Fatal(err)
	}
	if eventType != "evm_delegation_added" || severity != "critical" {
		t.Fatalf("stored alert = %s/%s", eventType, severity)
	}
	var nextIn float64
	if err := db.QueryRowContext(ctx, `SELECT extract(epoch FROM next_check_at-now()) FROM watchlist_targets WHERE id=$1`, id).Scan(&nextIn); err != nil {
		t.Fatal(err)
	}
	if nextIn < 30 || nextIn > 90 {
		t.Fatalf("an EVM wallet must be rechecked in about a minute, next check in %.0fs", nextIn)
	}
	if _, alerts, err := h.refreshWatchlistTarget(ctx, subject, id); err != nil || alerts != 0 {
		t.Fatalf("an unchanged delegation must not repeat: alerts=%d err=%v", alerts, err)
	}
}
