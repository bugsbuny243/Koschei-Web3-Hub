package clickhouse

import (
	"context"
	"fmt"
	"strings"
)

var allowedGlobalRadarMigrationTables = map[string]struct{}{
	"koschei_web3.global_radar_graph_records":      {},
	"koschei_web3.global_radar_events":             {},
	"koschei_web3.global_radar_ingest_checkpoints": {},
	"koschei_web3.global_radar_ingest_gaps":        {},
}

// ApplyTrustedGlobalRadarMigration executes one reviewed Global Radar migration
// as individually validated statements. ClickHouse Cloud 26.6 no longer accepts
// the legacy multiquery HTTP setting, so each additive statement is sent alone.
func (c *Client) ApplyTrustedGlobalRadarMigration(ctx context.Context, migrationSQL string) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	if err := validateTrustedGlobalRadarMigration(migrationSQL); err != nil {
		return err
	}
	statements := trustedMigrationStatements(migrationSQL)
	for index, statement := range statements {
		if err := c.applyTrustedMigrationStatement(ctx, statement); err != nil {
			return fmt.Errorf("ClickHouse Global Radar migration statement %d/%d failed: %w", index+1, len(statements), err)
		}
	}
	return nil
}

func validateTrustedGlobalRadarMigration(migrationSQL string) error {
	trimmed := strings.TrimSpace(migrationSQL)
	if trimmed == "" {
		return fmt.Errorf("ClickHouse Global Radar migration is empty")
	}
	if len(trimmed) > maxTrustedMigrationBytes {
		return fmt.Errorf("ClickHouse Global Radar migration exceeds %d byte safety limit", maxTrustedMigrationBytes)
	}

	seenDatabase := false
	seenTable := ""
	for _, statement := range trustedMigrationStatements(trimmed) {
		fields := strings.Fields(statement)
		if len(fields) < 6 || strings.ToUpper(fields[0]) != "CREATE" {
			return fmt.Errorf("ClickHouse Global Radar migration contains malformed or non-additive statement")
		}
		if strings.ToUpper(fields[2]) != "IF" || strings.ToUpper(fields[3]) != "NOT" || strings.ToUpper(fields[4]) != "EXISTS" {
			return fmt.Errorf("ClickHouse Global Radar migration must use IF NOT EXISTS")
		}
		switch strings.ToUpper(fields[1]) {
		case "DATABASE":
			if len(fields) != 6 || fields[5] != "koschei_web3" || seenDatabase {
				return fmt.Errorf("ClickHouse Global Radar migration may create koschei_web3 database at most once")
			}
			seenDatabase = true
		case "TABLE":
			if _, ok := allowedGlobalRadarMigrationTables[fields[5]]; !ok {
				return fmt.Errorf("ClickHouse Global Radar migration may not create %s", fields[5])
			}
			if seenTable != "" {
				return fmt.Errorf("ClickHouse Global Radar migration must create exactly one approved table")
			}
			seenTable = fields[5]
		default:
			return fmt.Errorf("ClickHouse Global Radar migration contains unauthorized CREATE operation")
		}
	}
	if !seenDatabase || seenTable == "" {
		return fmt.Errorf("ClickHouse Global Radar migration must contain CREATE DATABASE and one approved CREATE TABLE")
	}
	return nil
}
