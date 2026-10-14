package stellar

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"testing"
)

func FixtureAccount() (string, string) {
	address := EncodeAddress(make([]byte, 32))
	var b bytes.Buffer
	put := func(v uint32) { binary.Write(&b, binary.BigEndian, v) }
	put(0)
	put(0)
	b.Write(make([]byte, 32))
	b.Write(make([]byte, 20))
	put(0)
	put(11)
	put(0)
	b.Write([]byte{1, 1, 2, 2})
	put(0)
	put(0)
	return address, base64.StdEncoding.EncodeToString(b.Bytes())
}
func TestStrKeyAndXDR(t *testing.T) {
	address, xdr := FixtureAccount()
	if _, e := Address(address); e != nil {
		t.Fatal(e)
	}
	facts, e := DecodeEntry(xdr, address, "address")
	if e != nil || facts["flags"] != uint32(11) || facts["master_meets_high_threshold"] != false {
		t.Fatal(facts, e)
	}
	if _, e = DecodeEntry(base64.StdEncoding.EncodeToString([]byte{0, 0}), address, "address"); e == nil {
		t.Fatal("truncated XDR accepted")
	}
}
