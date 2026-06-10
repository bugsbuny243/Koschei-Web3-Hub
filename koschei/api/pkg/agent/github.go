package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

	pr := PullRequest{
		Title: title,
		Head:  head,
		Base:  "main",
		Body:  body,
	}

	payload, _ := json.Marshal(pr)
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("https://api.github.com/repos/%s/pulls", repo),
		bytes.NewBuffer(payload))

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PR create failed: %s", body)
	}

	return nil
}
