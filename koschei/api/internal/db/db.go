package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/lib/pq"
)

func Connect(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db ping failed: %w", err)
	}
	applied, skipped, err := runMigrations(db)
	if err != nil {
		return nil, err
	}
	log.Printf("migrations applied/skipped: %d/%d", applied, skipped)
	if err := verifySchema(db); err != nil {
		return nil, fmt.Errorf("schema verification failed: %w", err)
	}
	if err := ensureCanonicalPlans(db); err != nil {
		return nil, fmt.Errorf("canonical plans sync failed: %w", err)
	}
	log.Printf("canonical plans synced")
	return db, nil
}

func ensureCanonicalPlans(db *sql.DB) error {
	if _, err := db.Exec(`ALTER TABLE plans ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now()`); err != nil {
		return err
	}
	if _, err := db.Exec(`ALTER TABLE plans ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now()`); err != nil {
		return err
	}

	canonicalPlans := []struct {
		id             string
		name           string
		priceTry       int
		monthlyCredits int
		isActive       bool
	}{
		{id: "free", name: "Free", priceTry: 0, monthlyCredits: 0, isActive: true},
		{id: "starter", name: "Starter", priceTry: 899, monthlyCredits: 20000, isActive: true},
		{id: "pro", name: "Pro", priceTry: 2299, monthlyCredits: 70000, isActive: true},
		{id: "studio", name: "Studio", priceTry: 4999, monthlyCredits: 180000, isActive: true},
	}

	for _, plan := range canonicalPlans {
		if _, err := db.Exec(`
			INSERT INTO plans (id, name, price_try, monthly_credits, is_active)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE
			SET name = EXCLUDED.name,
				price_try = EXCLUDED.price_try,
				monthly_credits = EXCLUDED.monthly_credits,
				is_active = EXCLUDED.is_active,
				updated_at = now()
		`, plan.id, plan.name, plan.priceTry, plan.monthlyCredits, plan.isActive); err != nil {
			return err
		}
	}

	return nil
}
func runMigrations(db *sql.DB) (int, int, error) {
	applied := 0
	skipped := 0
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return 0, 0, err
	}
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		return 0, 0, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		log.Printf("warning: no migrations found at %s; continuing with schema verification", filepath.Join("migrations", "*.sql"))
	}
	for _, f := range files {
		v := filepath.Base(f)
		var exists string
		err = db.QueryRow(`SELECT version FROM schema_migrations WHERE version=$1`, v).Scan(&exists)
		if err == nil {
			skipped++
			continue
		}
		if err != sql.ErrNoRows {
			return applied, skipped, err
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return applied, skipped, err
		}
		if _, err := db.Exec(string(b)); err != nil {
			return applied, skipped, fmt.Errorf("migration failed: %s %w", v, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, v); err != nil {
			return applied, skipped, err
		}
		applied++
	}
	return applied, skipped, nil
}

func verifySchema(db *sql.DB) error {
	required := []string{"schema_migrations", "plans", "payment_requests", "credit_events", "generation_jobs", "model_route_logs", "runtime_projects", "runtime_tasks", "runtime_logs"}
	for _, t := range required {
		var ok bool
		if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)`, t).Scan(&ok); err != nil || !ok {
			return fmt.Errorf("required table missing: %s", t)
		}
	}
	return nil
}
