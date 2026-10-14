package evm

import (
	"strings"
	"testing"
)

func TestChecksumAndDelegationAddress(t *testing.T) {
	for _, s := range []string{"0x52908400098527886E0F7030069857D2E4169EE7", "0x5AEDA56215b167893e80B4fE645BA6d5Bab767DE", "0x" + strings.Repeat("a", 40)} {
		if _, e := Address(s); e != nil {
			t.Fatal(s, e)
		}
	}
	if _, e := Address("0x5AEDA56215b167893e80B4fE645BA6d5Bab767De"); e == nil {
		t.Fatal("bad checksum accepted")
	}
	if _, e := addressArray("0x" + strings.Repeat("f", 128)); e == nil {
		t.Fatal("hostile ABI accepted")
	}
	if len(Chains) != 14 {
		t.Fatal("chain catalog drift")
	}
}
