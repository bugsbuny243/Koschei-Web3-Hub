package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	githubOwner       = "bugsbuny243"
	githubRepo        = "Koschei-Web3-Hub"
	githubCommitsPath = "https://api.github.com/repos/%s/%s/commits?per_page=5"
)

type GitHubCommit struct {
	Author  string    `json:"author"`
	Message string    `json:"message"`
	Date    time.Time `json:"date"`
	URL     string    `json:"url"`
}

type githubManager struct {
	token  string
	client *http.Client
}

type githubCommitAPIResponse struct {
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
}

func (h *Handler) AdminGitHubCommits(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}

	manager, err := newGitHubManager()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	commits, err := manager.latestCommits(r.Context(), githubOwner, githubRepo)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "GitHub commits unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "commits": commits})
}

func newGitHubManager() (*githubManager, error) {
	loadDotEnvIfNeeded("GITHUB_TOKEN")
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is not configured")
	}
	return &githubManager{
		token: token,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (g *githubManager) latestCommits(ctx context.Context, owner, repo string) ([]GitHubCommit, error) {
	url := fmt.Sprintf(githubCommitsPath, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Koschei-Web3-Hub-Admin")

	res, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("github api returned %s", res.Status)
	}

	var apiCommits []githubCommitAPIResponse
	if err := json.NewDecoder(res.Body).Decode(&apiCommits); err != nil {
		return nil, err
	}

	commits := make([]GitHubCommit, 0, len(apiCommits))
	for _, item := range apiCommits {
		author := strings.TrimSpace(item.Commit.Author.Name)
		if item.Author != nil && strings.TrimSpace(item.Author.Login) != "" {
			author = strings.TrimSpace(item.Author.Login)
		}
		commits = append(commits, GitHubCommit{
			Author:  author,
			Message: strings.TrimSpace(item.Commit.Message),
			Date:    item.Commit.Author.Date,
			URL:     strings.TrimSpace(item.HTMLURL),
		})
	}
	return commits, nil
}

func loadDotEnvIfNeeded(requiredKey string) {
	if strings.TrimSpace(os.Getenv(requiredKey)) != "" {
		return
	}
	for _, path := range dotenvCandidates() {
		if loadDotEnvFile(path) == nil && strings.TrimSpace(os.Getenv(requiredKey)) != "" {
			return
		}
	}
}

func dotenvCandidates() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return []string{".env"}
	}
	return []string{
		filepath.Join(cwd, ".env"),
		filepath.Join(cwd, "..", ".env"),
		filepath.Join(cwd, "..", "..", ".env"),
		filepath.Join(cwd, "koschei", "api", ".env"),
		filepath.Join(cwd, ".env.local"),
	}
}

func loadDotEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := parseDotEnvLine(line)
		if !ok || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
	return nil
}

func parseDotEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	key, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	value = strings.Trim(value, `"'`)
	return key, value, true
}
