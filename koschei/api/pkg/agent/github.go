package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"koschei/api/internal/outboundhttp"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"koschei/api/internal/outbound"
)

type PullRequest struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
}

func CreatePullRequest(title, head, body string) error {
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	repo := strings.TrimSpace(os.Getenv("GITHUB_REPO")) // owner/repo
	parts := strings.Split(repo, "/")
	if token == "" {
		return fmt.Errorf("missing GITHUB_TOKEN")
	}
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("GITHUB_REPO must be owner/repo")
	}

	pr := PullRequest{
		Title: title,
		Head:  head,
		Base:  "main",
		Body:  body,
	}

	payload, err := json.Marshal(pr)
	if err != nil {
		return err
	}
	endpoint := url.URL{
		Scheme: "https",
		Host:   "api.github.com",
		Path:   "/repos/" + url.PathEscape(strings.TrimSpace(parts[0])) + "/" + url.PathEscape(strings.TrimSpace(parts[1])) + "/pulls",
	}
	validated, err := outbound.ValidateFixedHTTPSHost(endpoint.String(), "api.github.com")
	if err != nil {
		return fmt.Errorf("validate GitHub API endpoint: %w", err)
	}
	// #nosec G704 -- authority is fixed to api.github.com, repository components are path-escaped, and redirects are host-pinned below.
	req, err := http.NewRequest(http.MethodPost, validated.String(), bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	client := outbound.HardenFixedHostClient(&http.Client{Timeout: 15 * time.Second}, "api.github.com")
	// #nosec G704 -- request authority and every redirect are restricted to api.github.com.
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("PR create failed: %s", responseBody)
	}

	return nil
}
