package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func Connect(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func RunMigrations(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		tier TEXT NOT NULL DEFAULT 'public_saas',
		credits INTEGER NOT NULL DEFAULT 100,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS ai_generations (
		id UUID PRIMARY KEY,
		user_id UUID,
		task_type TEXT NOT NULL,
		model_name TEXT NOT NULL,
		output_type TEXT NOT NULL,
		credits_used INTEGER NOT NULL DEFAULT 0,
		response_payload JSONB NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed migrations: %w", err)
	}
	return nil
}
