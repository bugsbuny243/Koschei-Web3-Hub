package clickhouse

import "testing"

const validARVISReadIndexMigrationFixture = `
ALTER TABLE koschei_web3.arvis_verdict_snapshots
    ADD INDEX IF NOT EXISTS idx_target target TYPE bloom_filter(0.01) GRANULARITY 1;
ALTER TABLE koschei_web3.security_radar_stream_events
    ADD INDEX IF NOT EXISTS idx_target target TYPE bloom_filter(0.01) GRANULARITY 1;
`

func TestValidateTrustedARVISReadIndexMigrationAllowsOnlyAdditiveIndexes(t *testing.T) {
	if err := validateTrustedARVISReadIndexMigration(validARVISReadIndexMigrationFixture); err != nil {
		t.Fatalf("valid additive read-index migration rejected: %v", err)
	}
}

func TestValidateTrustedARVISReadIndexMigrationRejectsUnauthorizedTable(t *testing.T) {
	migration := `ALTER TABLE koschei_web3.customer_secrets ADD INDEX IF NOT EXISTS idx_x secret TYPE bloom_filter GRANULARITY 1;`
	if err := validateTrustedARVISReadIndexMigration(migration); err == nil {
		t.Fatal("unauthorized table migration was accepted")
	}
}

func TestValidateTrustedARVISReadIndexMigrationRejectsDestructiveOperation(t *testing.T) {
	migration := `ALTER TABLE koschei_web3.arvis_verdict_snapshots DROP COLUMN target;`
	if err := validateTrustedARVISReadIndexMigration(migration); err == nil {
		t.Fatal("destructive migration was accepted")
	}
}
