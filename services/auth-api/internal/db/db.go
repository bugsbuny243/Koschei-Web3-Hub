package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Profile struct {
	Subject string
	Email   string
}

type Client struct {
	databaseURL string
	httpClient  *http.Client
}

func New(databaseURL string, httpClient *http.Client) (*Client, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("DATABASE_URL is invalid")
	}
	return &Client{databaseURL: databaseURL, httpClient: httpClient}, nil
}

func (client *Client) UpsertProfile(ctx context.Context, profile Profile) error {
	database, _ := url.Parse(client.databaseURL)
	payload, _ := json.Marshal(map[string]any{
		"query":  `INSERT INTO app_user_profiles (auth_subject, email) VALUES ($1, lower($2)) ON CONFLICT (auth_subject) DO UPDATE SET email = EXCLUDED.email, updated_at = now()`,
		"params": []string{profile.Subject, profile.Email},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+database.Hostname()+"/sql", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Neon-Connection-String", client.databaseURL)
	res, err := client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Neon SQL endpoint returned %d", res.StatusCode)
	}
	return nil
}
