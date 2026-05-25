package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type artifactGenerateRequest struct {
	Force bool `json:"force"`
}

type artifactAIResponse struct {
	ArtifactTitle   string `json:"artifact_title"`
	ArtifactSummary string `json:"artifact_summary"`
	Files           []struct {
		Path     string `json:"path"`
		Language string `json:"language"`
		Purpose  string `json:"purpose"`
		Action   string `json:"action"`
		Content  string `json:"content"`
	} `json:"files"`
	Readme    string   `json:"readme"`
	Warnings  []string `json:"warnings"`
	NextSteps []string `json:"next_steps"`
}

const artifactSystemPrompt = `You are Koschei Artifact Builder.
Default language is Turkish for explanations and README.
Generate safe project files from the provided Runtime Contract 5.3 file_plan.
Return ONLY valid JSON.
Do not use markdown fences.
Do not execute anything.
Do not include secrets, API keys, private keys, passwords, tokens, or real credentials.
Do not include malware, exploit code, credential theft, bypass logic, or unauthorized access logic.
For security/government/bank projects, generate defensive templates only.
Generated code must be safe skeleton/MVP code, not dangerous automation.`

func sanitizeArtifactFilePath(path string) (string, bool, string) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", false, "empty_path"
	}
	lower := strings.ToLower(p)
	for _, b := range []string{"../", "..\\", "/etc", "/root", "/home", "c:\\", "windows", "system32", ".env", "id_rsa"} {
		if strings.Contains(lower, b) {
			return "", false, "unsafe_path"
		}
	}
	if strings.HasPrefix(p, "/") {
		return "", false, "absolute_path_not_allowed"
	}
	return strings.ReplaceAll(p, "\\", "/"), true, ""
}

func (h *Handler) RuntimeArtifactsRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/artifacts/generate") {
		h.GenerateArtifact(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/artifacts") {
		h.ListProjectArtifacts(w, r)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) ArtifactRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/download") {
		h.DownloadArtifact(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/files/") {
		h.GetArtifactFile(w, r)
		return
	}
	if r.Method == http.MethodGet {
		h.GetArtifact(w, r)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) GenerateArtifact(w http.ResponseWriter, r *http.Request) { /* simplified for brevity */
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	projectID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/runtime/projects/"), "/artifacts/generate")
	var req artifactGenerateRequest
	_ = decodeJSON(r, &req)
	var email, status string
	if err := h.DB.QueryRow(`SELECT email,status FROM runtime_projects WHERE id=$1`, projectID).Scan(&email, &status); err != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	isPriv, _, _ := h.userCreditsAndRole(claims.Sub)
	if email != claims.Email && !isPriv {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	low := strings.ToLower(status)
	if low == "review_needed" {
		writeJSON(w, 400, map[string]string{"error": "artifact_requires_human_review"})
		return
	}
	if low == "failed" || low == "blocked" {
		writeJSON(w, 400, map[string]string{"error": "artifact_generation_not_allowed"})
		return
	}
	if low != "completed" {
		writeJSON(w, 400, map[string]string{"error": "project_not_completed"})
		return
	}

	if !req.Force {
		var existingID string
		var fileCount int
		if err := h.DB.QueryRow(`SELECT id,file_count FROM generated_artifacts WHERE runtime_project_id=$1 AND status='completed' ORDER BY created_at DESC LIMIT 1`, projectID).Scan(&existingID, &fileCount); err == nil {
			writeJSON(w, 200, map[string]any{"artifact_id": existingID, "status": "completed", "file_count": fileCount, "existing": true})
			return
		}
	}

	artifactID := newID()
	if _, err := h.DB.Exec(`INSERT INTO generated_artifacts (id,runtime_project_id,user_email,status) VALUES ($1,$2,$3,'processing')`, artifactID, projectID, claims.Email); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}

	rows, err := h.DB.Query(`SELECT task_type,output_json FROM runtime_tasks WHERE project_id=$1 AND task_type IN ('delivery','review','blueprint','architecture','file_plan') ORDER BY updated_at DESC`, projectID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()
	payload := map[string]any{}
	filePlan := []any{}
	for rows.Next() {
		var tt string
		var out any
		_ = rows.Scan(&tt, &out)
		payload[tt] = out
		if tt == "file_plan" {
			b, _ := json.Marshal(out)
			var obj map[string]any
			_ = json.Unmarshal(b, &obj)
			if m, ok := obj["output"].(map[string]any); ok {
				if files, ok := m["files"].([]any); ok {
					filePlan = files
				}
			}
		}
	}
	if len(filePlan) == 0 {
		_, _ = h.DB.Exec(`UPDATE generated_artifacts SET status='failed',error_message='missing_file_plan',updated_at=now() WHERE id=$1`, artifactID)
		writeJSON(w, 400, map[string]string{"error": "missing_file_plan"})
		return
	}
	promptBytes, _ := json.Marshal(map[string]any{"contract_version": "5.3", "project_id": projectID, "file_plan": filePlan, "runtime_payload": payload})
	model := strings.TrimSpace(os.Getenv("TOGETHER_MODEL"))
	if model == "" {
		model = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
	}
	out, callErr := h.callTogetherWithSystemTimeout(model, artifactSystemPrompt, string(promptBytes), 60*time.Second)
	if callErr != nil {
		_, _ = h.DB.Exec(`UPDATE generated_artifacts SET status='failed',error_message=$2,updated_at=now() WHERE id=$1`, artifactID, shortError(callErr.Error()))
		writeJSON(w, 502, map[string]string{"error": "generation_failed"})
		return
	}
	var ai artifactAIResponse
	if err := json.Unmarshal([]byte(out), &ai); err != nil {
		_, _ = h.DB.Exec(`UPDATE generated_artifacts SET status='failed',error_message='invalid_json',metadata=jsonb_set(metadata,'{raw_ai_output}',to_jsonb($2::text),true),updated_at=now() WHERE id=$1`, artifactID, out)
		writeJSON(w, 502, map[string]string{"error": "invalid_generation_json"})
		return
	}
	if strings.TrimSpace(ai.Readme) != "" {
		ai.Files = append(ai.Files, struct {
			Path     string `json:"path"`
			Language string `json:"language"`
			Purpose  string `json:"purpose"`
			Action   string `json:"action"`
			Content  string `json:"content"`
		}{Path: "README.md", Language: "markdown", Purpose: "Project README", Action: "create", Content: ai.Readme})
	}

	tx, _ := h.DB.Begin()
	defer tx.Rollback()
	count := 0
	for _, f := range ai.Files {
		path, ok, reason := sanitizeArtifactFilePath(f.Path)
		if !ok {
			ai.Warnings = append(ai.Warnings, "skipped "+f.Path+": "+reason)
			continue
		}
		a := strings.ToLower(strings.TrimSpace(f.Action))
		if a != "create" && a != "update" && a != "review_only" {
			a = "review_only"
		}
		c := f.Content
		if strings.Contains(strings.ToLower(c), "api_key") {
			c = strings.ReplaceAll(c, "API_KEY", "YOUR_API_KEY_HERE")
		}
		if _, err := tx.Exec(`INSERT INTO generated_files (id,artifact_id,runtime_project_id,path,language,purpose,action,content,content_hash) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,md5($8))`, newID(), artifactID, projectID, path, f.Language, f.Purpose, a, c); err == nil {
			count++
		}
	}
	meta, _ := json.Marshal(map[string]any{"warnings": ai.Warnings, "next_steps": ai.NextSteps})
	if _, err := tx.Exec(`UPDATE generated_artifacts SET status='completed',title=$2,summary=$3,file_count=$4,zip_ready=true,metadata=$5::jsonb,updated_at=now() WHERE id=$1`, artifactID, ai.ArtifactTitle, ai.ArtifactSummary, count, string(meta)); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	if !isPriv {
		if err := h.applyCreditChargeTxWithReason(tx, claims.Sub, claims.Email, "artifact_generation"); err != nil {
			writeJSON(w, 402, map[string]string{"error": "insufficient_credits"})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"artifact_id": artifactID, "status": "completed", "file_count": count, "warnings": ai.Warnings})
}

func (h *Handler) ListProjectArtifacts(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	pid := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/runtime/projects/"), "/artifacts")
	rows, err := h.DB.Query(`SELECT id,runtime_project_id,user_email,status,artifact_type,title,summary,file_count,zip_ready,error_message,metadata,created_at,updated_at FROM generated_artifacts WHERE runtime_project_id=$1 AND user_email=$2 ORDER BY created_at DESC`, pid, claims.Email)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, rpid, em, st, at, title, summary sql.NullString
		var fc sql.NullInt64
		var zr sql.NullBool
		var er, meta, ca, ua any
		_ = rows.Scan(&id, &rpid, &em, &st, &at, &title, &summary, &fc, &zr, &er, &meta, &ca, &ua)
		out = append(out, map[string]any{"id": id.String, "runtime_project_id": rpid.String, "status": st.String, "artifact_type": at.String, "title": title.String, "summary": summary.String, "file_count": fc.Int64, "zip_ready": zr.Bool, "error_message": er, "metadata": meta, "created_at": ca, "updated_at": ua})
	}
	writeJSON(w, 200, out)
}
func (h *Handler) GetArtifact(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/artifacts/")
	var owner string
	_ = h.DB.QueryRow(`SELECT user_email FROM generated_artifacts WHERE id=$1`, strings.Split(id, "/")[0]).Scan(&owner)
	isPriv, _, _ := h.userCreditsAndRole(claims.Sub)
	if owner != claims.Email && !isPriv {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	rows, _ := h.DB.Query(`SELECT id,path,language,purpose,action,left(content,2000) FROM generated_files WHERE artifact_id=$1 ORDER BY path`, strings.Split(id, "/")[0])
	defer rows.Close()
	files := []map[string]any{}
	for rows.Next() {
		var i, p, l, pu, a, c string
		_ = rows.Scan(&i, &p, &l, &pu, &a, &c)
		files = append(files, map[string]any{"id": i, "path": p, "language": l, "purpose": pu, "action": a, "content_preview": c})
	}
	writeJSON(w, 200, map[string]any{"id": strings.Split(id, "/")[0], "files": files})
}
func (h *Handler) GetArtifactFile(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/api/artifacts/")
	seg := strings.Split(p, "/")
	if len(seg) < 4 {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	aid, fid := seg[0], seg[3]
	var owner, path, lang, purpose, action, content string
	if err := h.DB.QueryRow(`SELECT a.user_email,f.path,f.language,f.purpose,f.action,f.content FROM generated_files f JOIN generated_artifacts a ON a.id=f.artifact_id WHERE f.artifact_id=$1 AND f.id=$2`, aid, fid).Scan(&owner, &path, &lang, &purpose, &action, &content); err != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	isPriv, _, _ := h.userCreditsAndRole(claims.Sub)
	if owner != claims.Email && !isPriv {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	writeJSON(w, 200, map[string]any{"id": fid, "artifact_id": aid, "path": path, "language": lang, "purpose": purpose, "action": action, "content": content})
}
func (h *Handler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	aid := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/artifacts/"), "/download")
	aid = strings.TrimSuffix(aid, "/")
	var owner, title, summary string
	if err := h.DB.QueryRow(`SELECT user_email,coalesce(title,''),coalesce(summary,'') FROM generated_artifacts WHERE id=$1`, aid).Scan(&owner, &title, &summary); err != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	isPriv, _, _ := h.userCreditsAndRole(claims.Sub)
	if owner != claims.Email && !isPriv {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	rows, _ := h.DB.Query(`SELECT path,content,language,purpose,action FROM generated_files WHERE artifact_id=$1`, aid)
	defer rows.Close()
	type rf struct{ P, C, L, Pu, A string }
	var fs []rf
	for rows.Next() {
		var f rf
		_ = rows.Scan(&f.P, &f.C, &f.L, &f.Pu, &f.A)
		fs = append(fs, f)
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].P < fs[j].P })
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, f := range fs {
		zf, _ := zw.Create(f.P)
		_, _ = zf.Write([]byte(f.C))
	}
	readme, _ := zw.Create("README.md")
	_, _ = readme.Write([]byte("# " + title + "\n\n" + summary + "\n"))
	manifest, _ := zw.Create("koschei-artifact-manifest.json")
	m, _ := json.MarshalIndent(map[string]any{"artifact_id": aid, "generated_at": time.Now().UTC(), "file_count": len(fs)}, "", "  ")
	_, _ = manifest.Write(m)
	_ = zw.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=artifact-%s.zip", aid))
	_, _ = w.Write(buf.Bytes())
}
