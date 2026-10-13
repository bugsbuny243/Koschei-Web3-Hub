package openapi

import "testing"

func TestTronRouteDeclaresRequiredAddressAndProviderFailure(t *testing.T) {
	op := operation(Route{Path: "/api/v1/radar/tron-wallet-permissions"}, "get")
	params := op["parameters"].([]any)
	if len(params) != 1 {
		t.Fatalf("parameters=%v", params)
	}
	p := params[0].(map[string]any)
	if p["in"] != "query" || p["name"] != "address" || p["required"] != true {
		t.Fatalf("parameter=%v", p)
	}
	if _, ok := op["responses"].(map[string]any)["503"]; !ok {
		t.Fatal("missing provider unavailable response")
	}
}
