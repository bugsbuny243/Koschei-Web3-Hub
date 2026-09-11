package main

import (
	"strings"
	"testing"
)

func TestBuildApplicationStoresKeepsStatelessDefault(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "")
	t.Setenv("APP_DATABASE_READ_URL", "")
	// Legacy DATABASE_URL must not silently reactivate application persistence.
	t.Setenv("DATABASE_URL", "postgres://legacy.example.invalid/should-not-be-used")

	writeDB, readDB, initError, err := buildApplicationStores()
	if err != nil {
		t.Fatalf("buildApplicationStores returned error in stateless mode: %v", err)
	}
	if writeDB != nil || readDB != nil {
		t.Fatal("stateless mode unexpectedly returned an application database")
	}
	if !strings.Contains(initError, "APP_DATABASE_URL") {
		t.Fatalf("init error = %q; want APP_DATABASE_URL guidance", initError)
	}
}
