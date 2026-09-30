package radarcursor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const GapSchemaVersion = "koschei.global-radar-ingest-gap.v1"

const (
	GapStateOpen     = "open"
	GapStateResolved = "resolved"
)

type Gap struct {
	GapKey             string     `json:"gap_key"`
	SchemaVersion      string     `json:"schema_version"`
	CursorKey          string     `json:"cursor_key"`
	NetworkID          string     `json:"network_id"`
	StreamKind         string     `json:"stream_kind"`
	InitialStartHeight uint64     `json:"initial_start_height"`
	NextHeight         uint64     `json:"next_height"`
	EndHeight          uint64     `json:"end_height"`
	Reason             string     `json:"reason"`
	State              string     `json:"state"`
	DetectedAt         time.Time  `json:"detected_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`
}

type GapStore interface {
	LoadOpenGlobalRadarIngestGap(context.Context, string) (Gap, bool, error)
	SaveGlobalRadarIngestGap(context.Context, Gap) error
}

func NewGap(cursorKey, networkID, streamKind string, startHeight, endHeight uint64, reason string, detectedAt time.Time) (Gap, error) {
	if endHeight == math.MaxUint64 {
		return Gap{}, fmt.Errorf("gap end height is too large")
	}
	material := strings.Join([]string{
		strings.TrimSpace(cursorKey),
		strconv.FormatUint(startHeight, 10),
		strconv.FormatUint(endHeight, 10),
		detectedAt.UTC().Format(time.RFC3339Nano),
	}, "\n")
	sum := sha256.Sum256([]byte(material))
	return (Gap{
		GapKey:             hex.EncodeToString(sum[:]),
		SchemaVersion:      GapSchemaVersion,
		CursorKey:          cursorKey,
		NetworkID:          networkID,
		StreamKind:         streamKind,
		InitialStartHeight: startHeight,
		NextHeight:         startHeight,
		EndHeight:          endHeight,
		Reason:             reason,
		State:              GapStateOpen,
		DetectedAt:         detectedAt,
		UpdatedAt:          detectedAt,
	}).Canonical()
}

func (g Gap) Canonical() (Gap, error) {
	out := g
	out.GapKey = strings.ToLower(strings.TrimSpace(out.GapKey))
	out.SchemaVersion = strings.TrimSpace(out.SchemaVersion)
	if out.SchemaVersion == "" {
		out.SchemaVersion = GapSchemaVersion
	}
	out.CursorKey = strings.TrimSpace(out.CursorKey)
	out.NetworkID = strings.ToLower(strings.TrimSpace(out.NetworkID))
	out.StreamKind = strings.ToLower(strings.TrimSpace(out.StreamKind))
	out.Reason = strings.ToLower(strings.TrimSpace(out.Reason))
	out.State = strings.ToLower(strings.TrimSpace(out.State))
	out.DetectedAt = out.DetectedAt.UTC()
	out.UpdatedAt = out.UpdatedAt.UTC()
	if out.ResolvedAt != nil {
		resolved := out.ResolvedAt.UTC()
		out.ResolvedAt = &resolved
	}

	if out.SchemaVersion != GapSchemaVersion {
		return Gap{}, fmt.Errorf("unsupported gap schema %q", out.SchemaVersion)
	}
	if !validSHA256(out.GapKey) {
		return Gap{}, fmt.Errorf("gap key must be sha256")
	}
	if out.CursorKey == "" || len(out.CursorKey) > 256 {
		return Gap{}, fmt.Errorf("gap cursor key is required and must be <=256 bytes")
	}
	if out.NetworkID == "" || out.StreamKind == "" {
		return Gap{}, fmt.Errorf("gap network and stream kind are required")
	}
	if out.InitialStartHeight > out.EndHeight || out.EndHeight == math.MaxUint64 {
		return Gap{}, fmt.Errorf("gap height range is invalid")
	}
	if out.NextHeight < out.InitialStartHeight || out.NextHeight > out.EndHeight+1 {
		return Gap{}, fmt.Errorf("gap next height is outside the episode range")
	}
	if out.Reason == "" || len(out.Reason) > 120 {
		return Gap{}, fmt.Errorf("gap reason is required and must be <=120 bytes")
	}
	if out.DetectedAt.IsZero() || out.UpdatedAt.IsZero() {
		return Gap{}, fmt.Errorf("gap detected_at and updated_at are required")
	}
	switch out.State {
	case GapStateOpen:
		if out.NextHeight > out.EndHeight || out.ResolvedAt != nil {
			return Gap{}, fmt.Errorf("open gap must have unresolved heights and no resolved_at")
		}
	case GapStateResolved:
		if out.NextHeight != out.EndHeight+1 || out.ResolvedAt == nil || out.ResolvedAt.IsZero() {
			return Gap{}, fmt.Errorf("resolved gap must advance through end height and include resolved_at")
		}
	default:
		return Gap{}, fmt.Errorf("invalid gap state %q", out.State)
	}
	return out, nil
}
