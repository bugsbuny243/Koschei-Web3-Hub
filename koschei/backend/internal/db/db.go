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

	CREATE TABLE IF NOT EXISTS runtime_projects (
		id UUID PRIMARY KEY,
		email TEXT NOT NULL,
		title TEXT NOT NULL,
		prompt TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'queued',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS runtime_tasks (
		id UUID PRIMARY KEY,
		project_id UUID REFERENCES runtime_projects(id),
		email TEXT NOT NULL,
		task_type TEXT NOT NULL,
		tool TEXT NOT NULL,
		prompt TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'queued',
		priority INTEGER NOT NULL DEFAULT 5,
		result TEXT,
		error TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		started_at TIMESTAMPTZ,
		completed_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS runtime_logs (
		id UUID PRIMARY KEY,
		project_id UUID,
		task_id UUID,
		level TEXT NOT NULL DEFAULT 'info',
		message TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS credit_events (
		id UUID PRIMARY KEY,
		email TEXT NOT NULL,
		project_id UUID,
		task_id UUID,
		amount INTEGER NOT NULL,
		reason TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed migrations: %w", err)
	}
	return nil
}
