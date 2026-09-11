package main

import (
	"os"
	"strings"
	"testing"
)

func TestMetadataStudioContractIsMountedAndSaaSAuthorized(t *testing.T) {
	routes, err := os.ReadFile("internal/http/metadata_routes.go")
	if err != nil {
		t.Fatal(err)
	}
	js, err := os.ReadFile("public/js/koschei-premium-modules.js")
	if err != nil {
		t.Fatal(err)
	}
	routeText := string(routes)
	jsText := string(js)
	for _, required := range []string{
		`/api/metadata/generate`,
		`RequirePlanTier("professional"`,
		`h.GenerateMetadata`,
	} {
		if !strings.Contains(routeText, required) {
			t.Fatalf("metadata route contract missing %q", required)
		}
	}
	for _, required := range []string{
		`endpoint:'/api/metadata/generate'`,
		`asset_type:'project'`,
		`/api/auth/premium-access`,
		`Professional SaaS entitlement`,
	} {
		if !strings.Contains(jsText, required) {
			t.Fatalf("metadata frontend contract missing %q", required)
		}
	}
	if strings.Contains(jsText, "KOSCH HOLDER") || strings.Contains(jsText, "KOSCH holder access") {
		t.Fatal("metadata premium module still advertises token-holder product authorization")
	}
}
