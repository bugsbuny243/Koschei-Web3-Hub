package defense

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
)

// This guard is used only by opt-in database-writing tests. A matching
// database name on a remote host is NOT proof of staging isolation.
func validateIsolatedShadowPostgresDSN(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("invalid staging DSN: %w", err)
	}
	args, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return fmt.Errorf("invalid staging query parameters: %w", err)
	}
	if (u.Scheme != "postgres" && u.Scheme != "postgresql") ||
		u.Hostname() != "127.0.0.1" ||
		u.Port() != "5432" ||
		u.Path != "/koschei_shadow_test" ||
		u.Fragment != "" ||
		u.User == nil ||
		len(args) != 1 || len(args["sslmode"]) != 1 ||
		args.Get("sslmode") != "disable" {
		return errors.New("staging tests require postgres:// credentials at 127.0.0.1:5432/koschei_shadow_test?sslmode=disable with no other connection options")
	}
	return nil
}

func TestActiveResponseLedgerStagingDSNGuard(t *testing.T) {
	if err := validateIsolatedShadowPostgresDSN(
		"postgres://postgres:postgres@127.0.0.1:5432/koschei_shadow_test?sslmode=disable",
	); err != nil {
		t.Fatal(err)
	}
	for _, dsn := range []string{
		"postgres://postgres:password@db.internal:5432/koschei_shadow_test?sslmode=disable",
		"postgres://postgres:password@127.0.0.1:5432/production?sslmode=disable",
		"postgres://postgres:password@127.0.0.1:5433/koschei_shadow_test?sslmode=disable",
		"postgres://postgres:password@localhost:5432/koschei_shadow_test?sslmode=disable",
		"postgres://postgres:password@127.0.0.1:5432/koschei_shadow_test?sslmode=require",
		"postgres://postgres:password@127.0.0.1:5432/koschei_shadow_test?sslmode=disable&host=db.internal",
		"host=127.0.0.1 dbname=koschei_shadow_test sslmode=disable",
	} {
		if err := validateIsolatedShadowPostgresDSN(dsn); err == nil {
			t.Fatalf("unsafe staging DSN accepted: %q", dsn)
		}
	}
}
