package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"koschei/api/internal/outboundhttp"
	"net/http"
	"os"
	"regexp"
	"time"
)

type PullRequest struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
}

func CreatePullRequest(title, head, body string) error {
	token := os.Getenv("GITHUB_TOKEN")
	repo := os.Getenv("GITHUB_REPO") // owner/repo
	if token == "" || !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repo) {
		return fmt.Errorf("GitHub credentials or repository configuration unavailable")
	}

	pr := PullRequest{
		Title: title,
		Head:  head,
		Base:  "main",
		Body:  body,
	}

	payload, _ := json.Marshal(pr)
	req, err := outboundhttp.NewRequest(context.Background(), "POST",
		fmt.Sprintf("https://api.github.com/repos/%s/pulls", repo),
		bytes.NewBuffer(payload))

	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := outboundhttp.Do(client, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("PR create failed: %s", body)
	}

	return nil
}
