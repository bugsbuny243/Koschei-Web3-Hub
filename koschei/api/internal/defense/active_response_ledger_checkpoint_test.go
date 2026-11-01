package defense

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"
)

func activeResponseSignedCheckpointFixture(t *testing.T) (ActiveResponseSignedCheckpoint, []ActiveResponseLedgerEntry, map[string]ed25519.PublicKey, time.Time) {
	t.Helper()
	root, entry := shadowLedgerIntegrityFixture(t)
	now := entry.RecordedAt.Add(time.Minute)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{41}, ed25519.SeedSize))
	cp := ActiveResponseSignedCheckpoint{
		SchemaVersion: ActiveResponseCheckpointSchemaVersion,
		TenantID: entry.TenantID,
		StartHash: root,
		EndHash: entry.ChainHash,
		RecordCount: 1,
		SignerID: "audit-checkpoint-signer",
		IssuedAt: now,
	}
	data, err := ActiveResponseCheckpointSigningPayload(cp)
	if err != nil {
		t.Fatal(err)
	}
	cp.Signature = ed25519.Sign(private, data)
	return cp, []ActiveResponseLedgerEntry{entry},
		map[string]ed25519.PublicKey{cp.SignerID: private.Public().(ed25519.PublicKey)}, now
}

func TestActiveResponseCheckpointValidOfflineChain(t *testing.T) {
	cp, records, keys, now := activeResponseSignedCheckpointFixture(t)
	result, err := VerifySignedShadowLedgerCheckpoint(cp, records, keys, now)
	if err != nil || result.Status != "verified" || result.RecordsVerified != 1 || result.ExecutionPerformed {
		t.Fatalf("authentic checkpoint rejected or claimed live effect: %+v %v", result, err)
	}
}

func TestActiveResponseCheckpointFailsClosedOnTamperedSignature(t *testing.T) {
	cp, records, keys, now := activeResponseSignedCheckpointFixture(t)
	cp.Signature[0] ^= 1
	result, err := VerifySignedShadowLedgerCheckpoint(cp, records, keys, now)
	if err == nil || result.Status != "rejected" {
		t.Fatalf("tampered signature accepted: %+v %v", result, err)
	}
}

func TestActiveResponseCheckpointRejectsChangedScopeOrCount(t *testing.T) {
	cp, records, keys, now := activeResponseSignedCheckpointFixture(t)
	cp.TenantID = "foreign-tenant"
	if _, err := VerifySignedShadowLedgerCheckpoint(cp, records, keys, now); err == nil {
		t.Fatal("cross-tenant signed checkpoint accepted")
	}
	cp, records, keys, now = activeResponseSignedCheckpointFixture(t)
	cp.RecordCount = 2
	if _, err := VerifySignedShadowLedgerCheckpoint(cp, records, keys, now); err == nil {
		t.Fatal("record count mismatch accepted")
	}
	cp, records, keys, now = activeResponseSignedCheckpointFixture(t)
	cp.EndHash = cp.StartHash
	if _, err := VerifySignedShadowLedgerCheckpoint(cp, records, keys, now); err == nil {
		t.Fatal("unanchored checkpoint head accepted")
	}
}

func TestActiveResponseCheckpointRejectsUnknownSignerAndFutureTime(t *testing.T) {
	cp, records, keys, now := activeResponseSignedCheckpointFixture(t)
	if _, err := VerifySignedShadowLedgerCheckpoint(cp, records, nil, now); err == nil {
		t.Fatal("unknown signer accepted")
	}
	cp, records, keys, now = activeResponseSignedCheckpointFixture(t)
	if _, err := VerifySignedShadowLedgerCheckpoint(cp, records, keys, now.Add(-time.Second)); err == nil {
		t.Fatal("future checkpoint accepted")
	}
	cp, records, keys, now = activeResponseSignedCheckpointFixture(t)
	if _, err := VerifySignedShadowLedgerCheckpoint(cp, records, keys, time.Time{}); err == nil {
		t.Fatal("missing trusted time accepted")
	}
}

func TestActiveResponseCheckpointRejectsMutatedLedgerRow(t *testing.T) {
	cp, records, keys, now := activeResponseSignedCheckpointFixture(t)
	records[0].TargetID = "wrong-target"
	result, err := VerifySignedShadowLedgerCheckpoint(cp, records, keys, now)
	if err == nil || result.Status != "rejected" {
		t.Fatalf("changed row accepted against signed checkpoint: %+v %v", result, err)
	}
}
