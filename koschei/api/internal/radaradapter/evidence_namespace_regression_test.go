package radaradapter

import "testing"

func TestEvidenceIDsAreNamespaced(t *testing.T) {
	a := SubjectIdentity{Network: "solana", ChainIdentity: "mainnet", IdentityKind: "address", IdentityValue: "account-a", NativeID: "account-a"}
	b := a
	b.IdentityValue, b.NativeID = "account-b", "account-b"
	response := Response{Raw: []byte("fixture-a"), Host: "fixture.invalid"}
	first := NewRow(a, "upgrade_authority", response, "123", nil)
	second := NewRow(b, "upgrade_authority", response, "123", nil)
	if first.ID == second.ID {
		t.Fatal("different subjects share an evidence ID")
	}
	changed := NewRow(a, "upgrade_authority", Response{Raw: []byte("fixture-b"), Host: response.Host}, "123", nil)
	if first.ID == changed.ID {
		t.Fatal("different observations share an evidence ID")
	}
}
