package router

import "testing"

func TestDecodeJSONObject(t *testing.T) {
	var out struct {
		Summary string `json:"summary"`
	}
	if err := DecodeJSONObject("{\"summary\":\"ok\"}", &out); err != nil {
		t.Fatal(err)
	}
	if out.Summary != "ok" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if err := DecodeJSONObject("{\"summary\":\"ok\",\"extra\":true}", &out); err == nil {
		t.Fatal("unknown structured field was accepted")
	}
	if err := DecodeJSONObject("{\"summary\":\"one\"}{\"summary\":\"two\"}", &out); err == nil {
		t.Fatal("multiple JSON values were accepted")
	}
}
