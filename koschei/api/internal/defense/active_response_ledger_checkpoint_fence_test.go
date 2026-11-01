package defense

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestActiveResponseLedgerHighWatermarkSameHeight(t *testing.T) {
	cp, _, keys, now := activeResponseSignedCheckpointFixture(t)
	fence := TrustedShadowLedgerHighWatermark{
		TenantID:        cp.TenantID,
		RecordsVerified: cp.RecordCount,
		ChainHash:       cp.EndHash,
		IssuedAt:        cp.IssuedAt,
	}
	if err := validateShadowLedgerHighWatermark(cp, fence, now); err != nil {
		t.Fatalf("identical independently verified checkpoint rejected: %v", err)
	}
	got, err := VerifyCurrentShadowLedgerCheckpointWithHighWatermark(
		context.Background(), nil, cp, keys, fence, now,
	)
	if err == nil || got.Status != "rejected" || got.ExecutionPerformed {
		t.Fatalf("fence must not permit checking without PostgreSQL: %+v %v", got, err)
	}
}

func TestActiveResponseLedgerHighWatermarkRejectsRollbackAndFork(t *testing.T) {
	cp, _, _, now := activeResponseSignedCheckpointFixture(t)
	initial := TrustedShadowLedgerHighWatermark{
		TenantID: cp.TenantID, RecordsVerified: cp.RecordCount,
		ChainHash: cp.EndHash, IssuedAt: cp.IssuedAt,
	}
	type testCase struct {
		name   string
		change func(*ActiveResponseSignedCheckpoint, *TrustedShadowLedgerHighWatermark)
	}
	for _, test := range []testCase{
		{name: "wrong tenant", change: func(_ *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			f.TenantID = "different-tenant"
		}},
		{name: "zero count", change: func(_ *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			f.RecordsVerified = 0
		}},
		{name: "invalid trusted head", change: func(_ *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			f.ChainHash = "sha256:not-a-real-hash"
		}},
		{name: "missing trusted time", change: func(_ *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			f.IssuedAt = time.Time{}
		}},
		{name: "future trusted time", change: func(_ *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			f.IssuedAt = now.Add(time.Minute)
		}},
		{name: "decreased count", change: func(_ *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			f.RecordsVerified = cp.RecordCount + 1
		}},
		{name: "same count different head", change: func(_ *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			f.ChainHash = "sha256:" + strings.Repeat("e", 64)
		}},
		{name: "older issued time", change: func(cp *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			cp.IssuedAt = f.IssuedAt.Add(-time.Second)
		}},
		{name: "growth without later issuance", change: func(cp *ActiveResponseSignedCheckpoint, f *TrustedShadowLedgerHighWatermark) {
			cp.RecordCount++
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			newCP, fence := cp, initial
			test.change(&newCP, &fence)
			if err := validateShadowLedgerHighWatermark(newCP, fence, now); err == nil {
				t.Fatalf("unsafe high-water checkpoint transition accepted: %+v %+v", newCP, fence)
			}
		})
	}
}

func TestActiveResponseLedgerHighWatermarkAllowsMonotonicGrowth(t *testing.T) {
	cp, _, _, now := activeResponseSignedCheckpointFixture(t)
	fence := TrustedShadowLedgerHighWatermark{
		TenantID: cp.TenantID, RecordsVerified: cp.RecordCount,
		ChainHash: cp.EndHash, IssuedAt: cp.IssuedAt,
	}
	cp.RecordCount++
	cp.IssuedAt = cp.IssuedAt.Add(time.Second)
	if err := validateShadowLedgerHighWatermark(cp, fence, cp.IssuedAt.Add(time.Second)); err != nil {
		t.Fatalf("valid checkpoint growth rejected: %v", err)
	}
	_ = now
}
