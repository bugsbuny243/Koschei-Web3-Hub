package handlers

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

// SOC integration tests may write and clean up their own fixtures. Refuse
// any DSN that is not explicitly bound to the disposable local CI database.
func socPostgresAcceptanceDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("KOSCHEI_TEST_DATABASE_URL"))
	if dsn == "" || os.Getenv("KOSCHEI_SOC_TEST_DB_CONFIRMATION") != "isolated" {
		t.Skip("SOC PostgreSQL acceptance not enabled; isolated database and explicit confirmation required")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("invalid SOC acceptance DSN: %v", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") ||
		(host != "127.0.0.1" && host != "localhost" && host != "::1") ||
		parsed.Port() != "5432" ||
		parsed.Path != "/koschei_soc_acceptance" ||
		parsed.User == nil || parsed.User.Username() == "" ||
		parsed.RawQuery != "sslmode=disable" || parsed.Fragment != "" {
		t.Fatal("SOC acceptance DSN must target only the disposable loopback koschei_soc_acceptance database")
	}
	return dsn
}
