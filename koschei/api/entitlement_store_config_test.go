package main

import "testing"

func TestBuildEntitlementStoreWithoutConfigReturnsNil(t *testing.T) {
	t.Setenv("ENTITLEMENT_DATABASE_URL", "")
	store, err := buildEntitlementStore()
	if err != nil {
		t.Fatalf("buildEntitlementStore() error = %v", err)
	}
	if store != nil {
		t.Fatalf("buildEntitlementStore() = %p, want nil without configuration", store)
	}
}
