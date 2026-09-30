package handlers

import "testing"

func TestIncidentActionStatus(t *testing.T) {
	cases := map[string]string{
		"investigate": "investigating",
		"contain":     "contained",
		"resolve":     "resolved",
		"reopen":      "open",
		"close":       "closed",
		"note":        "",
	}
	for action, want := range cases {
		if got := incidentActionStatus(action); got != want {
			t.Fatalf("incidentActionStatus(%q)=%q want=%q", action, got, want)
		}
	}
}

func TestIncidentStatusTransitionAllowed(t *testing.T) {
	allowed := [][2]string{
		{"open", "investigating"},
		{"open", "contained"},
		{"investigating", "open"},
		{"investigating", "contained"},
		{"investigating", "resolved"},
		{"contained", "open"},
		{"contained", "investigating"},
		{"contained", "resolved"},
		{"resolved", "open"},
		{"resolved", "closed"},
		{"closed", "open"},
	}
	for _, pair := range allowed {
		if !incidentStatusTransitionAllowed(pair[0], pair[1]) {
			t.Fatalf("transition %q -> %q should be allowed", pair[0], pair[1])
		}
	}

	denied := [][2]string{
		{"", "open"},
		{"open", ""},
		{"open", "open"},
		{"open", "resolved"},
		{"open", "closed"},
		{"investigating", "closed"},
		{"contained", "closed"},
		{"resolved", "investigating"},
		{"resolved", "contained"},
		{"closed", "investigating"},
		{"closed", "contained"},
		{"closed", "resolved"},
		{"unknown", "open"},
	}
	for _, pair := range denied {
		if incidentStatusTransitionAllowed(pair[0], pair[1]) {
			t.Fatalf("transition %q -> %q should be denied", pair[0], pair[1])
		}
	}
}
