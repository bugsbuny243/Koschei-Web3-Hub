package http

import (
	"database/sql"
	"testing"
)

func TestWithEntitlementDBKeepsApplicationPersistenceSeparate(t *testing.T) {
	entitlementDB := &sql.DB{}
	config := serverConfig{}
	WithEntitlementDB(entitlementDB)(&config)
	if config.entitlementDB != entitlementDB {
		t.Fatalf("entitlement DB = %p, want %p", config.entitlementDB, entitlementDB)
	}
	if config.dbRead != nil {
		t.Fatalf("entitlement option unexpectedly set read DB: %p", config.dbRead)
	}
}
