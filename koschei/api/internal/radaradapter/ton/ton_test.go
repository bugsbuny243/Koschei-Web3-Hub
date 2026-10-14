package ton

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestAddressFormsRemainDistinct(t *testing.T) {
	raw := "0:" + strings.Repeat("0", 64)
	p, e := Address(raw)
	if e != nil || p.Form != "raw" {
		t.Fatal(p, e)
	}
	for _, tag := range []byte{0x11, 0x51, 0x91} {
		b := make([]byte, 36)
		b[0] = tag
		c := CRC16(b[:34])
		b[34] = byte(c >> 8)
		b[35] = byte(c)
		s := base64.RawURLEncoding.EncodeToString(b)
		p, e = Address(s)
		if e != nil || p.Raw != raw || p.Testnet != (tag == 0x91) {
			t.Fatal(p, e)
		}
		b[35] ^= 1
		if _, e = Address(base64.RawURLEncoding.EncodeToString(b)); e == nil {
			t.Fatal("bad CRC accepted")
		}
	}
}
func TestTraceDoesNotTurnOmissionsIntoSettlement(t *testing.T) {
	if TraceState([]byte(`{}`)) != "incomplete" {
		t.Fatal("empty settled")
	}
	good := `{"is_incomplete":false,"trace_info":{"pending_messages":0,"trace_state":"complete","transactions":1},"transactions":{"t":{"description":{"aborted":false},"in_msg":{"bounced":false}}}}`
	if TraceState([]byte(strings.Replace(good, `,"in_msg":{"bounced":false}`, "", 1))) != "incomplete" {
		t.Fatal("missing inbound message settled")
	}
	if TraceState([]byte(good)) != "complete_no_failure_observed" {
		t.Fatal("complete trace rejected")
	}
	if TraceState([]byte(strings.Replace(good, `"aborted":false`, `"aborted":true`, 1))) != "failed" {
		t.Fatal("failed settled")
	}
}
