package handlers

import "testing"

func TestDefaultKOSCHTierThresholds(t *testing.T) {
	thresholds, _, err := configuredTokenThresholds(6)
	if err != nil {
		t.Fatal(err)
	}
	if thresholds["basic"] != "25000" {
		t.Fatalf("basic threshold=%q", thresholds["basic"])
	}
	if thresholds["pro"] != "250000" || thresholds["enterprise"] != "2000000" {
		t.Fatalf("tier thresholds=%#v", thresholds)
	}
}
