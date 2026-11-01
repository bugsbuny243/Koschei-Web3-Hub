package defense

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const ActiveResponseLedgerIntegritySchemaVersion = "koschei-active-response-ledger-integrity-v1"

type ActiveResponseLedgerIntegrity struct {
	SchemaVersion      string `json:"schema_version"`
	TenantID           string `json:"tenant_id"`
	RecordsVerified    int    `json:"records_verified"`
	CheckpointRoot     string `json:"checkpoint_root"`
	CheckpointHead     string `json:"checkpoint_head"`
	Status             string `json:"status"`
	ExecutionPerformed bool   `json:"execution_performed"`
}

// shadowLedgerEntryHash mirrors the persisted v1 chain serialization and
// deliberately excludes ChainHash and ExecutionPerformed from the digest.
// ExecutionPerformed is validated separately and must remain false.
func shadowLedgerEntryHash(entry ActiveResponseLedgerEntry) (string, error) {
	payload := struct {
		SchemaVersion string    `json:"schema_version"`
		TenantID      string    `json:"tenant_id"`
		ActionID      string    `json:"action_id"`
		TargetID      string    `json:"target_id"`
		ProviderID    string    `json:"provider_id"`
		PlanDigest    string    `json:"plan_digest"`
		PreviousHash  string    `json:"previous_hash"`
		RecordedAt    time.Time `json:"recorded_at"`
	}{
		entry.SchemaVersion, entry.TenantID, entry.ActionID, entry.TargetID,
		entry.ProviderID, entry.PlanDigest, entry.PreviousHash, entry.RecordedAt.UTC(),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal shadow ledger entry: %w", err)
	}
	sum := sha256.Sum256(append([]byte("koschei-shadow-ledger-chain-v1\n"), encoded...))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// VerifyShadowLedgerSnapshot checks a complete, ordered, per-tenant range of
// entries against an independently trusted prior root and expected chain head.
// The caller MUST obtain checkpoints from separate, protected storage; hashes
// taken from the same untrusted database do not establish authenticity.
// This is an offline verification function and performs no provider action.
func VerifyShadowLedgerSnapshot(
	tenantID string,
	entries []ActiveResponseLedgerEntry,
	checkpointRoot string,
	checkpointHead string,
) (ActiveResponseLedgerIntegrity, error) {
	result := ActiveResponseLedgerIntegrity{
		SchemaVersion:      ActiveResponseLedgerIntegritySchemaVersion,
		TenantID:           tenantID,
		CheckpointRoot:     checkpointRoot,
		CheckpointHead:     checkpointHead,
		Status:             "rejected",
		ExecutionPerformed: false,
	}
	if !validShadowLedgerID(tenantID) ||
		!validActiveResponseSHA256(checkpointRoot) ||
		!validActiveResponseSHA256(checkpointHead) {
		return result, errors.New("trusted tenant and checkpoint digests are required")
	}
	if len(entries) == 0 {
		return result, errors.New("empty snapshot cannot establish chain continuity")
	}
	if len(entries) > 100000 {
		return result, errors.New("snapshot exceeds offline verification limit")
	}
	previous := checkpointRoot
	seenActions := make(map[string]struct{}, len(entries))
	seenHashes := make(map[string]struct{}, len(entries))
	for index, entry := range entries {
		if entry.SchemaVersion != ActiveResponseLedgerSchemaVersion ||
			entry.TenantID != tenantID ||
			!validShadowLedgerID(entry.ActionID) ||
			!validShadowLedgerID(entry.TargetID) ||
			!validShadowLedgerID(entry.ProviderID) ||
			!validActiveResponseSHA256(entry.PlanDigest) ||
			!validActiveResponseSHA256(entry.ChainHash) ||
			entry.ExecutionPerformed ||
			entry.RecordedAt.IsZero() ||
			entry.PreviousHash != previous {
			return result, fmt.Errorf("shadow ledger boundary or chain link invalid at index %d", index)
		}
		if _, duplicate := seenActions[entry.ActionID]; duplicate {
			return result, fmt.Errorf("duplicate action id at index %d", index)
		}
		if _, duplicate := seenHashes[entry.ChainHash]; duplicate {
			return result, fmt.Errorf("replayed chain hash at index %d", index)
		}
		calculated, err := shadowLedgerEntryHash(entry)
		if err != nil || calculated != entry.ChainHash {
			return result, fmt.Errorf("shadow ledger digest mismatch at index %d", index)
		}
		seenActions[entry.ActionID] = struct{}{}
		seenHashes[entry.ChainHash] = struct{}{}
		previous = entry.ChainHash
		result.RecordsVerified++
	}
	if previous != checkpointHead {
		result.Status = "rejected"
		return result, errors.New("shadow ledger head is not anchored to trusted checkpoint")
	}
	result.Status = "verified"
	return result, nil
}
