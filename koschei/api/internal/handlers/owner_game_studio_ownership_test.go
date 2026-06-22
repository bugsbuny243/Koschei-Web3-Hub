package handlers

import "testing"

func TestOwnerGameStudioIdentityIsSeparatedFromCustomerIdentity(t *testing.T) {
	t.Setenv("OWNER_WALLET", "OwnerWallet123")
	got := ownerGameStudioUserID()
	if got != "owner:ownerwallet123" {
		t.Fatalf("ownerGameStudioUserID() = %q", got)
	}
}
