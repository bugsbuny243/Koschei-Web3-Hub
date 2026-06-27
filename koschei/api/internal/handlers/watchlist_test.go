package handlers

import "testing"

func TestCompareWatchlistSnapshotsSkipsInitialBaseline(t *testing.T) {
	current := watchlistTokenSnapshot{Score: 40, RiskLevel: "medium", Supply: "1000"}
	alerts := compareWatchlistSnapshots(nil, current, 50)
	if len(alerts) != 0 {
		t.Fatalf("initial snapshot should not create alerts: %#v", alerts)
	}
}

func TestCompareWatchlistSnapshotsDetectsRiskIncreaseAndThreshold(t *testing.T) {
	previous := &watchlistTokenSnapshot{Score: 72, RiskLevel: "low", Supply: "1000"}
	current := watchlistTokenSnapshot{Score: 35, RiskLevel: "high", Supply: "1000", Findings: []string{"mint authority active"}}
	alerts := compareWatchlistSnapshots(previous, current, 50)
	if !hasWatchlistEvent(alerts, "risk_increased") {
		t.Fatalf("expected risk increase alert: %#v", alerts)
	}
	if !hasWatchlistEvent(alerts, "threshold_crossed") {
		t.Fatalf("expected threshold alert: %#v", alerts)
	}
}

func TestCompareWatchlistSnapshotsDetectsAuthorityAndSupplyChanges(t *testing.T) {
	previous := &watchlistTokenSnapshot{
		Score: 80, RiskLevel: "low", Supply: "1000", MintAuthority: "", FreezeAuthority: "FreezeA",
		LargestHolderPercent: 12,
	}
	current := watchlistTokenSnapshot{
		Score: 65, RiskLevel: "medium", Supply: "1500", MintAuthority: "MintA", FreezeAuthority: "",
		LargestHolderPercent: 24,
	}
	alerts := compareWatchlistSnapshots(previous, current, 50)
	for _, event := range []string{"risk_increased", "mint_authority_changed", "freeze_authority_changed", "holder_concentration_increased", "supply_changed"} {
		if !hasWatchlistEvent(alerts, event) {
			t.Fatalf("expected %s alert: %#v", event, alerts)
		}
	}
	mintAlert := findWatchlistEvent(alerts, "mint_authority_changed")
	if mintAlert == nil || mintAlert.Severity != "critical" {
		t.Fatalf("reactivated mint authority must be critical: %#v", mintAlert)
	}
}

func TestCompareWatchlistSnapshotsIgnoresSmallMovements(t *testing.T) {
	previous := &watchlistTokenSnapshot{Score: 80, RiskLevel: "low", Supply: "1000", LargestHolderPercent: 10}
	current := watchlistTokenSnapshot{Score: 75, RiskLevel: "low", Supply: "1000", LargestHolderPercent: 15}
	alerts := compareWatchlistSnapshots(previous, current, 50)
	if len(alerts) != 0 {
		t.Fatalf("small movement should not create alert: %#v", alerts)
	}
}

func TestDeduplicateWatchlistAlerts(t *testing.T) {
	alerts := []watchlistAlertCandidate{
		{EventType: "risk_increased", Severity: "medium", CurrentValue: 30},
		{EventType: "risk_increased", Severity: "critical", CurrentValue: 30},
		{EventType: "supply_changed", Severity: "high", CurrentValue: "2000"},
	}
	result := deduplicateWatchlistAlerts(alerts)
	if len(result) != 2 {
		t.Fatalf("expected two unique alerts, got %#v", result)
	}
	if result[0].Severity != "high" {
		t.Fatalf("alerts should be severity sorted: %#v", result)
	}
}

func hasWatchlistEvent(alerts []watchlistAlertCandidate, event string) bool {
	return findWatchlistEvent(alerts, event) != nil
}

func findWatchlistEvent(alerts []watchlistAlertCandidate, event string) *watchlistAlertCandidate {
	for index := range alerts {
		if alerts[index].EventType == event {
			return &alerts[index]
		}
	}
	return nil
}
