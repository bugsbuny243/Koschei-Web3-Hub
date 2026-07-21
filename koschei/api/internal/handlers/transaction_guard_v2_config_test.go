package handlers

import "testing"

func TestValidateGuardOperatorBlocklist(t *testing.T) {
	valid := "11111111111111111111111111111111,ComputeBudget111111111111111111111111111111"
	if err := validateGuardOperatorBlocklist(valid); err != nil {
		t.Fatalf("valid operator blocklist was rejected: %v", err)
	}
	for _, raw := range []string{
		"not-a-program",
		"11111111111111111111111111111111,",
		"11111111111111111111111111111111,not-a-program",
	} {
		if err := validateGuardOperatorBlocklist(raw); err == nil {
			t.Fatalf("invalid operator blocklist was accepted: %q", raw)
		}
	}
}
