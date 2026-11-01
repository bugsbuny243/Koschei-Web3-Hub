package defense

import (
	"strings"
	"testing"
	"time"
)

func shadowLedgerIntegrityFixture(t *testing.T) (string, ActiveResponseLedgerEntry) {
	t.Helper()
	plan, review, recovery, now := activeResponseLedgerFixture(t)
	root := "sha256:" + strings.Repeat("0", 64)
	entry, err := BuildShadowLedgerEntry(plan, review, recovery, root, now)
	if err != nil {
		t.Fatal(err)
	}
	return root, entry
}

func TestActiveResponseLedgerIntegrityValidAnchoredSnapshot(t *testing.T) {
	root, entry := shadowLedgerIntegrityFixture(t)
	result, err := VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{entry}, root, entry.ChainHash)
	if err != nil || result.Status != "verified" || result.RecordsVerified != 1 || result.ExecutionPerformed {
		t.Fatalf("valid offline chain rejected or claimed execution: %+v %v", result, err)
	}
}

func TestActiveResponseLedgerIntegrityRejectsAlteredPayload(t *testing.T) {
	root, entry := shadowLedgerIntegrityFixture(t)
	tampered := entry
	tampered.TargetID = "other-target"
	result, err := VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{tampered}, root, entry.ChainHash)
	if err == nil || result.Status != "rejected" {
		t.Fatalf("tampered chain accepted: %+v %v", result, err)
	}
	tampered = entry
	tampered.RecordedAt = tampered.RecordedAt.Add(time.Second)
	_, err = VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{tampered}, root, entry.ChainHash)
	if err == nil {
		t.Fatal("changed timestamp accepted")
	}
	tampered = entry
	tampered.ExecutionPerformed = true
	_, err = VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{tampered}, root, entry.ChainHash)
	if err == nil {
		t.Fatal("ledger must not assert real action execution")
	}
}

func TestActiveResponseLedgerIntegrityRejectsWrongRootHeadAndScope(t *testing.T) {
	root, entry := shadowLedgerIntegrityFixture(t)
	wrongRoot := "sha256:" + strings.Repeat("1", 64)
	if _, err := VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{entry}, wrongRoot, entry.ChainHash); err == nil {
		t.Fatal("wrong trusted root accepted")
	}
	if _, err := VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{entry}, root, wrongRoot); err == nil {
		t.Fatal("wrong trusted head accepted")
	}
	if _, err := VerifyShadowLedgerSnapshot("foreign-tenant", []ActiveResponseLedgerEntry{entry}, root, entry.ChainHash); err == nil {
		t.Fatal("cross-tenant audit entry accepted")
	}
	if _, err := VerifyShadowLedgerSnapshot(entry.TenantID, nil, root, root); err == nil {
		t.Fatal("empty range accepted")
	}
}

func TestActiveResponseLedgerIntegrityRejectsDuplicateOrDisconnectedRange(t *testing.T) {
	root, entry := shadowLedgerIntegrityFixture(t)
	if _, err := VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{entry, entry}, root, entry.ChainHash); err == nil {
		t.Fatal("duplicated audit event accepted")
	}
	second := entry
	second.ActionID = "second-action"
	second.PreviousHash = entry.ChainHash
	second.RecordedAt = second.RecordedAt.Add(time.Second)
	computed, err := shadowLedgerEntryHash(second)
	if err != nil {
		t.Fatal(err)
	}
	second.ChainHash = computed
	got, err := VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{entry, second}, root, second.ChainHash)
	if err != nil || got.RecordsVerified != 2 {
		t.Fatalf("valid two-link chain failed: %+v %v", got, err)
	}
	second.PreviousHash = root
	if _, err := VerifyShadowLedgerSnapshot(entry.TenantID, []ActiveResponseLedgerEntry{entry, second}, root, second.ChainHash); err == nil {
		t.Fatal("disconnected second link accepted")
	}
}
