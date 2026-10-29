package openapi

import (
	"strings"
	"testing"
)

func TestSOCOperationContract(t *testing.T) {
	for _, method := range []string{"get", "post"} {
		op := operation(Route{Path: "/api/owner/soc/case"}, method)
		p := op["parameters"].([]any)[0].(map[string]any)
		if p["name"] != "id" || p["required"] != true {
			t.Fatal(p)
		}
		if _, ok := op["responses"].(map[string]any)["503"]; !ok {
			t.Fatal("missing unavailable state")
		}
		if method == "post" {
			if op["requestBody"].(map[string]any)["required"] != true {
				t.Fatal("command not required")
			}
			if _, ok := op["responses"].(map[string]any)["409"]; !ok {
				t.Fatal("missing concurrency conflict")
			}
		}
	}
}

func TestSOCRuntimeReadinessOpenAPIHasNoCaseOrSLAClaims(t *testing.T) {
	op := operation(Route{
		Path:            "/api/owner/soc/readiness",
		InventoryAuth:   "owner_session",
		InventorySource: "route_inventory.go",
	}, "get")
	if len(op["parameters"].([]any)) != 0 {
		t.Fatal("read-only readiness probe inherited case filters")
	}
	if _, ok := op["requestBody"]; ok {
		t.Fatal("read-only readiness probe unexpectedly accepts mutations")
	}
	if !strings.Contains(op["description"].(string), "No staffing") ||
		strings.Contains(op["description"].(string), "First-response SLA") {
		t.Fatal("readiness contract fabricates human response or SLA authority")
	}
	if op["x-koschei-auth-tier"] != "owner_session" {
		t.Fatal("readiness endpoint lost owner authentication")
	}
	responses := op["responses"].(map[string]any)
	if _, ok := responses["503"]; !ok {
		t.Fatal("missing fail-closed unavailable state")
	}
	for code, wantOK := range map[string]bool{"200": true, "503": false} {
		response := responses[code].(map[string]any)
		media := response["content"].(map[string]any)["application/json"].(map[string]any)
		schema := media["schema"].(map[string]any)
		properties := schema["properties"].(map[string]any)
		if properties["ok"].(map[string]any)["const"] != wantOK {
			t.Fatalf("response %s claimed wrong ok state", code)
		}
	}
}
