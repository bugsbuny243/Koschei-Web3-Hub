package radaradapter

import "testing"

func TestTronPermissionThresholds(t *testing.T) {
	raw := []byte(`{
		"owner_permission":{"type":0,"id":0,"threshold":2,"keys":[
			{"address":"41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","weight":1},
			{"address":"41bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","weight":1}
		]},
		"active_permission":[{"type":2,"id":2,"threshold":3,"keys":[
			{"address":"41cccccccccccccccccccccccccccccccccccccccc","weight":1}
		]}]
	}`)
	findings, err := ParseTronAccountPermissions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 permissions, got %d", len(findings))
	}
	if !findings[0].ThresholdReachable || findings[0].TotalWeight != 2 {
		t.Fatalf("owner permission threshold: %+v", findings[0])
	}
	if findings[1].ThresholdReachable || findings[1].TotalWeight != 1 {
		t.Fatalf("active permission threshold: %+v", findings[1])
	}
}

func TestTronPermissionParserFailsClosed(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`{"owner_permission":{"id":0,"threshold":0}}`,
		`{"active_permission":[{"id":2,"threshold":1,"keys":[{"address":"abc","weight":-1}]}]}`,
		`{"active_permission":[{"id":2,"threshold":1},{"id":2,"threshold":1}]}`,
		`not json`,
		`{"owner_permission":{"id":0,"threshold":1,"keys":[{"address":"41aaaa","weight":1}]}}`,
		`{"owner_permission":{"id":0,"threshold":1,"keys":[{"address":"41zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz","weight":1}]}}`,
		`{"owner_permission":{"id":0,"threshold":2,"keys":[{"address":"41aaaa","weight":1},{"address":"41aaaa","weight":1}]}}`,
		`{"owner_permission":{"id":0,"threshold":2,"keys":[{"address":"41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","weight":1},{"address":"41AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","weight":1}]}}`,
		`{"owner_permission":{"id":0,"threshold":2,"keys":[{"address":"41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","weight":1},{"address":"TRXcKoEvHr6Y38VMcDYGBEYKznvH3XUX4g","weight":1}]}}`,
	} {
		if _, err := ParseTronAccountPermissions([]byte(raw)); err == nil {
			t.Fatalf("invalid permission accepted: %s", raw)
		}
	}
}

func TestTronPermissionAddressShape(t *testing.T) {
	for _, address := range []string{
		"41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"TRXcKoEvHr6Y38VMcDYGBEYKznvH3XUX4g",
	} {
		if !validTronPermissionAddress(address) {
			t.Fatalf("expected valid address shape: %s", address)
		}
	}
	for _, address := range []string{"", "abc", " 41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "41gggggggggggggggggggggggggggggggggggggggg", "T000000000000000000000000000000000", "TAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		if validTronPermissionAddress(address) {
			t.Fatalf("accepted invalid address shape: %s", address)
		}
	}
}

func TestTronPermissionCanonicalSignerIdentity(t *testing.T) {
	hexSigner := "41aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	base58Signer := "TRXcKoEvHr6Y38VMcDYGBEYKznvH3XUX4g"
	if canonicalTronPermissionSigner(hexSigner) != canonicalTronPermissionSigner(base58Signer) {
		t.Fatal("hex and Base58Check addresses for same signer must canonicalize identically")
	}
}
