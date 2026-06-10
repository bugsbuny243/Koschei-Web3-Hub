package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/bugsbuny243/Koschei-Web3-Hub/koschei/api/pkg/agent"
)

func AIRunHandler(w http.ResponseWriter, r *http.Request) {
	type Request struct{ Prompt string }
	var req Request
	json.NewDecoder(r.Body).Decode(&req)

	resp, err := agent.CallAI(req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	branch := "ai-pr-" + generateID()
	agent.CreateBranch(branch)

	err = os.WriteFile(resp.File, []byte(resp.Code), 0644)
	if err != nil {
		http.Error(w, "Dosya yazılamadı", 500)
		return
	}

	agent.AddAndCommit(resp.File, resp.CommitMsg)
	agent.PushBranch(branch)
	agent.CreatePullRequest(resp.CommitMsg, branch, "AI otomatik oluşturdu.")

	response := map[string]string{
		"pr_url": "https://github.com/" + os.Getenv("GITHUB_REPO") + "/pull/new/" + branch,
	}
	json.NewEncoder(w).Encode(response)
}
