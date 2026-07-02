package handlers

import "testing"

func TestPremiumAccessPrefersPackage(t *testing.T) {
	status := decidePremiumAccess(true, tokenAccessEvaluation{
		GateEnabled:    true,
		Configured:     true,
		WalletVerified: true,
		Tier:           "enterprise",
		Amount:         "100000",
	})
	if !status.Active || status.Source != "package" {
		t.Fatalf("status = %+v, want active package access", status)
	}
}

func TestPremiumAccessAllowsBasicTokenHolder(t *testing.T) {
	status := decidePremiumAccess(false, tokenAccessEvaluation{
		GateEnabled:    true,
		Configured:     true,
		WalletVerified: true,
		Tier:           "basic",
		Amount:         "1000",
	})
	if !status.Active || status.Source != "token" {
		t.Fatalf("status = %+v, want active token access", status)
	}
}

func TestPremiumAccessStaysClosedWhenGateDisabled(t *testing.T) {
	status := decidePremiumAccess(false, tokenAccessEvaluation{
		GateEnabled:    false,
		Configured:     true,
		WalletVerified: true,
		Tier:           "enterprise",
		Amount:         "100000",
	})
	if status.Active || status.Source != "none" {
		t.Fatalf("status = %+v, want closed access", status)
	}
}
