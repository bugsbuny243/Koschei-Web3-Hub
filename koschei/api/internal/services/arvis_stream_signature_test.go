package services

import "testing"

func TestArvisStreamScopedVerdictSignature(t *testing.T) {
	base := "base-signature"
	module := ModuleFinalVerdictEngine
	first := arvisStreamScopedVerdictSignature(base, module, "event-1")
	second := arvisStreamScopedVerdictSignature(base, module, "event-2")
	repeat := arvisStreamScopedVerdictSignature(base, module, "event-1")

	if first == base {
		t.Fatal("stream signature must be scoped to the event")
	}
	if first == second {
		t.Fatal("different stream events must not share verdict signatures")
	}
	if first != repeat {
		t.Fatal("same stream event must produce a deterministic signature")
	}
	if got := arvisStreamScopedVerdictSignature(base, module, ""); got != base {
		t.Fatalf("missing stream event must preserve base signature, got %s", got)
	}
}
