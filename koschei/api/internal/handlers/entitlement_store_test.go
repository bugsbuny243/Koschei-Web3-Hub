package handlers

import (
	"database/sql"
	"testing"
)

func TestEntitlementStorePrefersDedicatedLedger(t *testing.T) {
	applicationDB := &sql.DB{}
	entitlementDB := &sql.DB{}
	h := &Handler{DB: applicationDB, EntitlementDB: entitlementDB}
	if got := h.entitlementStore(); got != entitlementDB {
		t.Fatalf("entitlementStore() = %p, want dedicated ledger %p", got, entitlementDB)
	}
}

func TestEntitlementStoreFallsBackToApplicationDBForLegacyDeployments(t *testing.T) {
	applicationDB := &sql.DB{}
	h := &Handler{DB: applicationDB}
	if got := h.entitlementStore(); got != applicationDB {
		t.Fatalf("entitlementStore() = %p, want application DB fallback %p", got, applicationDB)
	}
}

func TestEntitlementStoreCanRemainUnavailableInStatelessRuntime(t *testing.T) {
	h := &Handler{}
	if got := h.entitlementStore(); got != nil {
		t.Fatalf("entitlementStore() = %p, want nil", got)
	}
}
