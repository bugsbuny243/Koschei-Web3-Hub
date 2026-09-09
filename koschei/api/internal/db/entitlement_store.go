package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ConnectEntitlementStore opens an existing commercial authorization ledger
// without running application migrations. It is intended for stateless Web3
// runtimes that must authorize paid customer operations while leaving durable
// investigation/application persistence disabled.
func ConnectEntitlementStore(databaseURL string) (*sql.DB, error) {
	store, err := open(databaseURL)
	if err != nil {
		return nil, err
	}
	if err := verifyEntitlementStore(store); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("entitlement store verification failed: %w", err)
	}
	return store, nil
}

func verifyEntitlementStore(store *sql.DB) error {
	if store == nil {
		return fmt.Errorf("nil entitlement store")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checks := []string{
		`SELECT id, email, plan_id, outputs_total, outputs_remaining, status, starts_at, expires_at FROM entitlements LIMIT 0`,
		`SELECT email, auth_subject, status FROM app_user_profiles LIMIT 0`,
		`SELECT email, amount, reason, event_type FROM credit_events LIMIT 0`,
	}
	for _, query := range checks {
		rows, err := store.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
	}
	return nil
}
