package handlers

import "testing"

func TestOwnerGameStudioIdentityIsSeparatedFromCustomerIdentity(t *testing.T) {
	t.Setenv("OWNER_WALLET", "OwnerWallet123")
	got := ownerGameStudioUserID()
	if got != "owner:OwnerWallet123" {
		t.Fatalf("ownerGameStudioUserID() = %q", got)
	}
}
