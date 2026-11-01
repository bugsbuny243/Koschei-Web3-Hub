package defense

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestActiveResponseReceiptRejectsMissingDatabase(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	got, err := InspectShadowLedgerReceipt(context.Background(), nil,
		"tenant", "action", "target", "provider", digest)
	if err == nil || got.Status != ActiveResponseReceiptUnknown || got.ExecutionPerformed {
		t.Fatalf("missing receipt database should fail closed: %+v %v", got, err)
	}
}

func TestActiveResponseReceiptRejectsMalformedScope(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		name   string
		tenant string
		digest string
	}{
		{name: "empty", tenant: "", digest: digest},
		{name: "untrusted newline", tenant: "tenant\nother", digest: digest},
		{name: "excessive tenant", tenant: strings.Repeat("x", 129), digest: digest},
		{name: "bad digest", tenant: "tenant", digest: "sha256:invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := InspectShadowLedgerReceipt(context.Background(), nil,
				tc.tenant, "action", "target", "provider", tc.digest)
			if err == nil || got.Status != ActiveResponseReceiptUnknown {
				t.Fatalf("invalid trusted scope accepted: %+v %v", got, err)
			}
		})
	}
}

func TestActiveResponseReceiptRejectsCanceledOrClosedDatabase(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://invalid:invalid@127.0.0.1:5432/koschei_shadow_test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	digest := "sha256:" + strings.Repeat("a", 64)
	got, err := InspectShadowLedgerReceipt(ctx, db, "tenant", "action", "target", "provider", digest)
	if !errors.Is(err, context.Canceled) || got.Status != ActiveResponseReceiptUnknown {
		t.Fatalf("canceled read must fail closed: %+v %v", got, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = InspectShadowLedgerReceipt(context.Background(), db, "tenant", "action", "target", "provider", digest)
	if err == nil || got.Status != ActiveResponseReceiptUnknown || got.ExecutionPerformed {
		t.Fatalf("closed database cannot verify receipt: %+v %v", got, err)
	}
}
