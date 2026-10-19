package clickhouse

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func replayHeadsJSON(count int) string {
	var lines strings.Builder
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&lines, "{\"snapshot_sha256\":\"%064x\",\"recorded_at_ns\":%d}\n", i, 1000+i)
	}
	return lines.String()
}

func TestGlobalRadarReplayPageSentinelDoesNotStallOver25Snapshots(t *testing.T) {
	all := replayHeadsJSON(26)
	first, err := decodeGlobalRadarReplayHeadPage(json.NewDecoder(strings.NewReader(all)), 25)
	if err != nil {
		t.Fatalf("26 queued snapshots stalled the 25-item replay page: %v", err)
	}
	if len(first) != 25 || first[0].RecordedAtNanos != 1001 || first[24].RecordedAtNanos != 1025 {
		t.Fatalf("unexpected first replay page: len=%d", len(first))
	}
	// Durable cursor is the LAST returned snapshot, not the lookahead row.
	if first[24].SnapshotSHA256 != fmt.Sprintf("%064x", 25) {
		t.Fatal("page cursor included the sentinel snapshot")
	}
	second, err := decodeGlobalRadarReplayHeadPage(json.NewDecoder(strings.NewReader(replayHeadsJSON(1))), 25)
	if err != nil || len(second) != 1 {
		t.Fatalf("last replay page failed: len=%d err=%v", len(second), err)
	}
}

func TestGlobalRadarReplayRejectsCorruptPageIdentity(t *testing.T) {
	if _, err := decodeGlobalRadarReplayHeadPage(json.NewDecoder(strings.NewReader("{\"snapshot_sha256\":\"invalid\",\"recorded_at_ns\":42}\n")), 25); err == nil {
		t.Fatal("invalid replay head accepted")
	}
	if _, err := decodeGlobalRadarReplayHeadPage(json.NewDecoder(strings.NewReader(replayHeadsJSON(1))), 0); err == nil {
		t.Fatal("unbounded replay page accepted")
	}
}
