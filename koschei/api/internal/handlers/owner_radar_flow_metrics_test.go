package handlers

import (
	"testing"
	"time"
)

func TestRadarFlowBasisPoints(t *testing.T) {
	cases := []struct {
		name        string
		numerator   int64
		denominator int64
		want        int64
	}{
		{name: "half", numerator: 50, denominator: 100, want: 5000},
		{name: "full", numerator: 100, denominator: 100, want: 10000},
		{name: "backlog can exceed window denominator", numerator: 150, denominator: 100, want: 15000},
		{name: "zero numerator", numerator: 0, denominator: 100, want: 0},
		{name: "zero denominator", numerator: 100, denominator: 0, want: 0},
		{name: "negative values fail closed", numerator: -1, denominator: 100, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := radarFlowBasisPoints(tc.numerator, tc.denominator); got != tc.want {
				t.Fatalf("radarFlowBasisPoints(%d,%d)=%d want=%d", tc.numerator, tc.denominator, got, tc.want)
			}
		})
	}
}

func TestRadarFlowAgeSeconds(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	if got := radarFlowAgeSeconds(now, "2026-09-30T11:58:00Z"); got != 120 {
		t.Fatalf("age=%d want=120", got)
	}
	for _, value := range []string{"", "not-a-time", "2026-09-30T12:01:00Z"} {
		if got := radarFlowAgeSeconds(now, value); got != 0 {
			t.Fatalf("radarFlowAgeSeconds(%q)=%d want=0", value, got)
		}
	}
}
