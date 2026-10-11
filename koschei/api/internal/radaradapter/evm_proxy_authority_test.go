package radaradapter

import (
	"fmt"
	"strings"
	"testing"
)

func evmSlot(address string) []byte {
	return []byte(fmt.Sprintf(`{"result":"0x%s%s"}`, strings.Repeat("0", 24), address))
}

func TestEVMProxyAuthorityStorage(t *testing.T) {
	impl := strings.Repeat("a", 40)
	admin := strings.Repeat("b", 40)
	result, err := ParseEVMProxyAuthority(evmSlot(impl), evmSlot(admin), evmSlot(strings.Repeat("0", 40)))
	if err != nil {
		t.Fatal(err)
	}
	if result.Implementation != "0x"+impl || result.Admin != "0x"+admin || result.Beacon != "" {
		t.Fatalf("unexpected proxy slots: %+v", result)
	}
}

func TestEVMProxyAuthorityRejectsIncompleteProviderResponses(t *testing.T) {
	for _, input := range [][]byte{
		[]byte(`{"result":"0x1234"}`),
		[]byte(`{"error":{"code":-32000,"message":"provider unavailable"}}`),
		[]byte(`{"result":"0xzzzz"}`),
		[]byte(`{}`),
	} {
		if _, err := ParseEVMStorageAddress(input); err == nil {
			t.Fatalf("invalid storage accepted: %s", input)
		}
	}
}
