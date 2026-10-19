package handlers

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestPlanEntitlementRejectsOwnerBannedAndRemovedProfilesPostgres17(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	// The temporary table shadows the production schema on THIS disposable
	// session. The test neither changes real profiles nor creates entitlements.
	if _, err := db.ExecContext(ctx, `
		CREATE TEMP TABLE app_user_profiles (
			auth_subject text NOT NULL,
			email text NOT NULL,
			status text NOT NULL
		) ON COMMIT PRESERVE ROWS
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO app_user_profiles(auth_subject,email,status)
		VALUES('test-ban-subject','test-ban@example.invalid','banned'),
		      ('test-remove-subject','test-remove@example.invalid','removed')
	`); err != nil {
		t.Fatal(err)
	}
	h := &Handler{DB: db}
	for _, tc := range []struct{subject, email string}{
		{"test-ban-subject", "test-ban@example.invalid"},
		{"test-remove-subject", "test-remove@example.invalid"},
		// Revoke access even if a principal supplies the old email with a
		// different JWT/API-key subject: ownership may not be silently reassigned.
		{"other-jwt-subject", "test-ban@example.invalid"},
		{"test-remove-subject", "other@example.invalid"},
	} {
		access, err := h.evaluatePlanAccess(ctx, tc.subject, tc.email)
		if err != nil {
			t.Fatalf("revocation lookup failed for %s: %v", tc.subject, err)
		}
		if access.Active || access.Plan != "none" || access.Source != "profile_revoked" {
			t.Fatalf("banned/removed customer retained paid access: %+v", access)
		}
	}
}
