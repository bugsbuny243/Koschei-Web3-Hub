package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Store struct {
	databaseURL, sqlURL string
	http                *http.Client
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("DATABASE_URL is not configured")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("DATABASE_URL is invalid")
	}
	return &Store{databaseURL: databaseURL, sqlURL: "https://" + parsed.Hostname() + "/sql", http: &http.Client{Timeout: 12 * time.Second}}, nil
}
func (s *Store) Close() {}
func (s *Store) UpsertProfile(ctx context.Context, subject, email string) error {
	body, _ := json.Marshal(map[string]any{"query": `INSERT INTO app_user_profiles (auth_subject, email)
VALUES ($1, lower($2))
ON CONFLICT (auth_subject) DO UPDATE SET email = EXCLUDED.email, updated_at = now()`, "params": []string{subject, email}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.sqlURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Neon-Connection-String", s.databaseURL)
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Neon profile upsert failed (%d)", resp.StatusCode)
	}
	return nil
}
