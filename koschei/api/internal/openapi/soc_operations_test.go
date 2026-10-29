package openapi

import "testing"

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
