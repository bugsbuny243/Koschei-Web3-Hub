package services

import (
	"context"
	"testing"
	"time"

	"koschei/api/internal/radarcursor"
	"koschei/api/internal/runtimehealth"
)

type memoryGapStore struct {
	gaps []radarcursor.Gap
}

func (s *memoryGapStore) LoadOpenGlobalRadarIngestGap(_ context.Context, cursorKey string) (radarcursor.Gap, bool, error) {
	for i := len(s.gaps) - 1; i >= 0; i-- {
		if s.gaps[i].CursorKey == cursorKey && s.gaps[i].State == radarcursor.GapStateOpen {
			return s.gaps[i], true, nil
		}
	}
	return radarcursor.Gap{}, false, nil
}

func (s *memoryGapStore) SaveGlobalRadarIngestGap(_ context.Context, gap radarcursor.Gap) error {
	canonical, err := gap.Canonical()
	if err != nil {
		return err
	}
	for i := len(s.gaps) - 1; i >= 0; i-- {
		if s.gaps[i].GapKey == canonical.GapKey {
			s.gaps[i] = canonical
			return nil
		}
	}
	s.gaps = append(s.gaps, canonical)
	return nil
}

func TestGlobalRadarGapOpensAdvancesAndResolves(t *testing.T) {
	store := &memoryGapStore{}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	target := GlobalRadarHeadIngestTarget{Kind: GlobalRadarHeadIngestEVM, NetworkID: "ethereum-mainnet"}
	cursorKey := GlobalRadarHeadIngestCursorKey(target)
	cfg := GlobalRadarHeadIngestConfig{GapStore: store, Health: runtimehealth.New()}

	if err := recordGlobalRadarIngestGap(context.Background(), cfg, target, cursorKey, 101, 104, "block_collect_failed", now); err != nil {
		t.Fatal(err)
	}
	gap, found, err := store.LoadOpenGlobalRadarIngestGap(context.Background(), cursorKey)
	if err != nil || !found {
		t.Fatalf("open gap missing: found=%v err=%v", found, err)
	}
	if gap.NextHeight != 101 || gap.EndHeight != 104 || gap.State != radarcursor.GapStateOpen {
		t.Fatalf("unexpected open gap: %#v", gap)
	}

	if err := advanceGlobalRadarIngestGap(context.Background(), cfg, cursorKey, 101, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	gap, _, _ = store.LoadOpenGlobalRadarIngestGap(context.Background(), cursorKey)
	if gap.NextHeight != 102 {
		t.Fatalf("next_height=%d want 102", gap.NextHeight)
	}

	checkpoint := radarcursor.Checkpoint{
		CursorKey: cursorKey, SchemaVersion: radarcursor.SchemaVersion,
		NetworkID: "ethereum-mainnet", StreamKind: GlobalRadarHeadIngestEVM,
		Height: 103, BlockHash: "0xabc", SourceEventSHA256: string(make([]byte, 64)),
		State: radarcursor.StateCanonical, ObservedAt: now,
	}
	if err := reconcileGlobalRadarIngestGapToCheckpoint(context.Background(), cfg, checkpoint, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	gap, _, _ = store.LoadOpenGlobalRadarIngestGap(context.Background(), cursorKey)
	if gap.NextHeight != 104 {
		t.Fatalf("checkpoint reconciliation next_height=%d want 104", gap.NextHeight)
	}

	if err := advanceGlobalRadarIngestGap(context.Background(), cfg, cursorKey, 104, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.LoadOpenGlobalRadarIngestGap(context.Background(), cursorKey); found {
		t.Fatal("gap should be resolved after final replayed height")
	}
	if got := store.gaps[0]; got.State != radarcursor.GapStateResolved || got.ResolvedAt == nil || got.NextHeight != 105 {
		t.Fatalf("resolved gap: %#v", got)
	}
}

func TestGlobalRadarGapEpisodeExtendsWithoutCallingBacklogDataLoss(t *testing.T) {
	store := &memoryGapStore{}
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	target := GlobalRadarHeadIngestTarget{Kind: GlobalRadarHeadIngestBitcoin, NetworkID: "bitcoin-mainnet"}
	cursorKey := GlobalRadarHeadIngestCursorKey(target)
	cfg := GlobalRadarHeadIngestConfig{GapStore: store}

	if err := recordGlobalRadarIngestGap(context.Background(), cfg, target, cursorKey, 501, 504, "confirmation_failed", now); err != nil {
		t.Fatal(err)
	}
	if err := recordGlobalRadarIngestGap(context.Background(), cfg, target, cursorKey, 501, 508, "confirmation_failed", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	gap, found, _ := store.LoadOpenGlobalRadarIngestGap(context.Background(), cursorKey)
	if !found || gap.InitialStartHeight != 501 || gap.NextHeight != 501 || gap.EndHeight != 508 {
		t.Fatalf("extended gap: %#v", gap)
	}
	if len(store.gaps) != 1 {
		t.Fatalf("repeated failure created %d episodes; want 1", len(store.gaps))
	}
}
