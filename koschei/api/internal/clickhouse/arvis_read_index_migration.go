package clickhouse

import (
	"context"
	"fmt"
	"strings"
)

// ApplyTrustedARVISReadIndexMigration applies only the reviewed additive
// data-skipping indexes used by the bounded ARVIS memory read path.
func (c *Client) ApplyTrustedARVISReadIndexMigration(ctx context.Context, migrationSQL string) error {
	if c == nil {
		return fmt.Errorf("ClickHouse client is unavailable")
	}
	if err := validateTrustedARVISReadIndexMigration(migrationSQL); err != nil {
		return err
	}
	statements := trustedMigrationStatements(migrationSQL)
	for index, statement := range statements {
		if err := c.applyTrustedMigrationStatement(ctx, statement); err != nil {
			return fmt.Errorf("ClickHouse ARVIS read-index migration statement %d/%d failed: %w", index+1, len(statements), err)
		}
	}
	return nil
}

func validateTrustedARVISReadIndexMigration(migrationSQL string) error {
	trimmed := strings.TrimSpace(migrationSQL)
	if trimmed == "" {
		return fmt.Errorf("ClickHouse ARVIS read-index migration is empty")
	}
	if len(trimmed) > maxTrustedMigrationBytes {
		return fmt.Errorf("ClickHouse ARVIS read-index migration exceeds %d byte safety limit", maxTrustedMigrationBytes)
	}
	allowedTables := map[string]bool{
		"koschei_web3.arvis_verdict_snapshots":      true,
		"koschei_web3.security_radar_stream_events": true,
	}
	seen := 0
	for _, raw := range strings.Split(stripSQLLineComments(trimmed), ";") {
		statement := strings.TrimSpace(raw)
		if statement == "" {
			continue
		}
		fields := strings.Fields(statement)
		if len(fields) < 10 ||
			strings.ToUpper(fields[0]) != "ALTER" || strings.ToUpper(fields[1]) != "TABLE" ||
			!allowedTables[fields[2]] ||
			strings.ToUpper(fields[3]) != "ADD" || strings.ToUpper(fields[4]) != "INDEX" ||
			strings.ToUpper(fields[5]) != "IF" || strings.ToUpper(fields[6]) != "NOT" || strings.ToUpper(fields[7]) != "EXISTS" {
			return fmt.Errorf("ClickHouse ARVIS read-index migration contains non-additive or unauthorized statement")
		}
		upper := strings.ToUpper(statement)
		for _, forbidden := range []string{" MATERIALIZE ", " DROP ", " DELETE ", " UPDATE ", " MODIFY ", " RENAME ", " TRUNCATE "} {
			if strings.Contains(" "+upper+" ", forbidden) {
				return fmt.Errorf("ClickHouse ARVIS read-index migration contains forbidden operation")
			}
		}
		seen++
	}
	if seen == 0 {
		return fmt.Errorf("ClickHouse ARVIS read-index migration contains no index statements")
	}
	return nil
}
