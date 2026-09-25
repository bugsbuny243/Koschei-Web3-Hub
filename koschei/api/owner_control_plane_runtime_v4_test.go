package main

import (
	"os"
	"strings"
	"testing"
)

func TestOwnerCanonicalRuntimeSeparatesMissingFromZero(t *testing.T) {
	body, err := os.ReadFile("public/js/owner-control-center.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(body)
	for _, required := range []string{
		"if(v===null||v===undefined||v==='')return'—'",
		"data_state",
		"application_database",
		"entitlement_store",
		"global_radar_graph",
		"global_radar_events",
		"owner_assistant",
	} {
		if !strings.Contains(js, required) {
			t.Fatalf("owner canonical runtime missing %q", required)
		}
	}
	if strings.Contains(js, "Number(v||0)") {
		t.Fatal("owner runtime still coerces missing operational values to zero")
	}
}
