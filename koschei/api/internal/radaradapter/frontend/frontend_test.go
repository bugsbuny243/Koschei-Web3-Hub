package frontend

import (
	r "koschei/api/internal/radaradapter"
	"testing"
)

func TestSignatureSurfaceIsShapeOnly(t *testing.T) {
	if len(SignatureSurfaces([]byte(`{"method":"eth_signTypedData_v4","params":["0x0",{"primaryType":"PermitSingle"}]}`))) != 1 {
		t.Fatal("permit missed")
	}
	if len(SignatureSurfaces([]byte(`{"method":"eth_chainId"}`))) != 0 {
		t.Fatal("benign request flagged")
	}
}
func TestRejectAmbiguousOrigin(t *testing.T) {
	for _, s := range []string{"http://example.com", "https://user:secret@example.com", "https://example.com:8443", "https://example.com/?token=secret"} {
		if _, e := New(r.Target{Network: "offchain", Kind: "frontend", Raw: s}, nil); e == nil {
			t.Fatal("invalid origin accepted")
		}
	}
}
