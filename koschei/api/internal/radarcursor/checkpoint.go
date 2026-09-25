package radarcursor

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const SchemaVersion = "koschei.global-radar-ingest-checkpoint.v1"

const (
	StateCanonical     = "canonical"
	StateReorgObserved = "reorg_observed"
	StateRewind        = "rewind"
)

type Checkpoint struct {
	CursorKey         string    `json:"cursor_key"`
	SchemaVersion     string    `json:"schema_version"`
	NetworkID         string    `json:"network_id"`
	StreamKind        string    `json:"stream_kind"`
	Height            uint64    `json:"height"`
	BlockHash         string    `json:"block_hash"`
	ParentHash        string    `json:"parent_hash,omitempty"`
	SourceEventSHA256 string    `json:"source_event_sha256"`
	State             string    `json:"state"`
	ObservedAt        time.Time `json:"observed_at"`
}

type Store interface {
	LoadGlobalRadarIngestCheckpoint(context.Context, string) (Checkpoint, bool, error)
	SaveGlobalRadarIngestCheckpoint(context.Context, Checkpoint) error
}

type RecoveryStore interface {
	Store
	LoadGlobalRadarCanonicalCheckpointAtHeight(context.Context, string, uint64) (Checkpoint, bool, error)
}

func (c Checkpoint) Canonical() (Checkpoint, error) {
	out := c
	out.CursorKey = strings.TrimSpace(out.CursorKey)
	out.SchemaVersion = strings.TrimSpace(out.SchemaVersion)
	if out.SchemaVersion == "" {
		out.SchemaVersion = SchemaVersion
	}
	out.NetworkID = strings.ToLower(strings.TrimSpace(out.NetworkID))
	out.StreamKind = strings.ToLower(strings.TrimSpace(out.StreamKind))
	out.BlockHash = strings.ToLower(strings.TrimSpace(out.BlockHash))
	out.ParentHash = strings.ToLower(strings.TrimSpace(out.ParentHash))
	out.SourceEventSHA256 = strings.ToLower(strings.TrimSpace(out.SourceEventSHA256))
	out.State = strings.ToLower(strings.TrimSpace(out.State))
	out.ObservedAt = out.ObservedAt.UTC()

	if out.SchemaVersion != SchemaVersion {
		return Checkpoint{}, fmt.Errorf("unsupported checkpoint schema %q", out.SchemaVersion)
	}
	if out.CursorKey == "" || len(out.CursorKey) > 256 {
		return Checkpoint{}, fmt.Errorf("checkpoint cursor key is required and must be <=256 bytes")
	}
	if out.NetworkID == "" || out.StreamKind == "" {
		return Checkpoint{}, fmt.Errorf("checkpoint network and stream kind are required")
	}
	if out.BlockHash == "" || len(out.BlockHash) > 128 || len(out.ParentHash) > 128 {
		return Checkpoint{}, fmt.Errorf("checkpoint block hash is required and hashes must be <=128 bytes")
	}
	if !validSHA256(out.SourceEventSHA256) {
		return Checkpoint{}, fmt.Errorf("checkpoint source event digest must be sha256")
	}
	switch out.State {
	case StateCanonical, StateReorgObserved, StateRewind:
	default:
		return Checkpoint{}, fmt.Errorf("invalid checkpoint state %q", out.State)
	}
	if out.ObservedAt.IsZero() {
		return Checkpoint{}, fmt.Errorf("checkpoint observed_at is required")
	}
	return out, nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
