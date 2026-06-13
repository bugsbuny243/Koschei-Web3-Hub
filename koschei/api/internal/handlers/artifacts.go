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
	"strconv"
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

type gameStoreMetadataResponse struct {
	WhatTheGameIs         string         `json:"what_the_game_is"`
	WhoItIsFor            string         `json:"who_it_is_for"`
	UserIntentSatisfied   []string       `json:"user_intent_satisfied"`
	AskPlayWhyRecommended string         `json:"ask_play_why_recommended"`
	ShortDescription      string         `json:"short_description"`
	FullDescription       string         `json:"full_description"`
	SearchIntentPhrases   []string       `json:"search_intent_phrases"`
	TargetAudience        map[string]any `json:"target_audience"`
	CategorySuggestions   []string       `json:"category_suggestions"`
	LocalizationPlan      map[string]any `json:"localization_plan"`
	PlayShortsScript      string         `json:"play_shorts_script"`
	PlayShortsScenePlan   []any          `json:"play_shorts_scene_plan"`
	AskPlaySummary        string         `json:"ask_play_summary"`
	PolicyNotes           []string       `json:"policy_notes"`
}

const gameStoreMetadataPrompt = `You are Koschei Google Play Discovery Metadata Builder.
Return ONLY valid JSON.
Do not use markdown fences.
Do not add text outside JSON.
Do not include claims of guaranteed ranking, guaranteed installs, or guaranteed recommendation placement.
Do not use spammy keyword stuffing.
Keep all text compliant with Google Play policy and suitable for customer-owned games.
This is store publishing metadata, not a separate video generation module.
Required JSON shape:
{
  "what_the_game_is": "string",
  "who_it_is_for": "string",
  "user_intent_satisfied": ["string"],
  "ask_play_why_recommended": "string",
  "short_description": "string",
  "full_description": "string",
  "search_intent_phrases": ["string"],
  "target_audience": {"age_range": "string", "player_profiles": ["string"], "content_notes": ["string"]},
  "category_suggestions": ["string"],
  "localization_plan": {"primary_language": "string", "secondary_languages": ["string"], "adaptation_notes": ["string"]},
  "play_shorts_script": "string",
  "play_shorts_scene_plan": [{"scene": 1, "duration_seconds": 5, "visual": "string", "voiceover": "string"}],
  "ask_play_summary": "string",
  "policy_notes": ["string"]
}`
const artifactSystemPrompt = `You are Koschei Artifact Builder.
Default language is Turkish for README and explanations.
Return ONLY valid JSON.
Do not use markdown fences.
Do not add text outside JSON.
Do not execute anything.
Do not include secrets, API keys, private keys, passwords, tokens, or real credentials.
Do not include malware, exploit code, credential theft, bypass logic, or unauthorized access logic.
For security/government/bank projects, generate defensive templates only.
Required JSON shape:
{
  "artifact_title": "string",
  "artifact_summary": "string",
  "files": [
    {
      "path": "string",
      "language": "string",
      "purpose": "string",
      "action": "create|update|review_only",
      "content": "string"
    }
  ],
  "readme": "string",
  "warnings": ["string"],
  "next_steps": ["string"]
}
Rules:
- Generate 3 to 10 safe files maximum.
- Use only safe project paths.
- Include README content in readme.
- Use redacted configuration names such as YOUR_API_KEY_HERE, never real secrets.
- Keep code skeleton/MVP-safe.
- Do not include destructive shell commands.
- Do not include code that attempts unauthorized access.
- Proposed tool calls are still not executed.`

func sanitizeArtifactFilePath(path string) (string, bool, string) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", false, "empty_path"
	}
	if strings.HasPrefix(p, "/") {
		return "", false, "absolute_path_not_allowed"
	}
	lower := strings.ToLower(p)
	for _, b := range []string{"../", "..\\", "/etc", "/root", "/home", "c:\\", "windows/system32", "windows\\system32", ".env", "id_rsa", "private_key", "secrets.", "credentials."} {
		if strings.Contains(lower, b) {
			return "", false, "unsafe_path"
		}
	}
	return strings.ReplaceAll(p, "\\", "/"), true, ""
}

func sanitizeArtifactContent(content string) (string, bool) {
	c := content
	if strings.Contains(c, "BEGIN PRIVATE KEY") || strings.Contains(c, "BEGIN RSA PRIVATE KEY") || strings.Contains(c, "BEGIN OPENSSH PRIVATE KEY") {
		return "", false
	}
	c = strings.ReplaceAll(c, "API_KEY", "YOUR_API_KEY_HERE")
	c = strings.ReplaceAll(c, "api_key", "YOUR_API_KEY_HERE")
	c = strings.ReplaceAll(c, "token", "YOUR_SECRET_HERE")
	c = strings.ReplaceAll(c, "TOKEN", "YOUR_SECRET_HERE")
	c = strings.ReplaceAll(c, "SECRET", "YOUR_SECRET_HERE")
	c = strings.ReplaceAll(c, "password", "YOUR_PASSWORD_HERE")
	c = strings.ReplaceAll(c, "PASSWORD", "YOUR_PASSWORD_HERE")
	return c, true
}

func decodeJSONB(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	switch v := value.(type) {
	case map[string]any:
		return v
	case []byte:
		var out map[string]any
		_ = json.Unmarshal(v, &out)
		if out != nil {
			return out
		}
	case string:
		var out map[string]any
		_ = json.Unmarshal([]byte(v), &out)
		if out != nil {
			return out
		}
	case json.RawMessage:
		var out map[string]any
		_ = json.Unmarshal(v, &out)
		if out != nil {
			return out
		}
	}
	return map[string]any{}
}

func buildArtifactAIResponse(raw string) (artifactAIResponse, string, error) {
	var ai artifactAIResponse
	if err := json.Unmarshal([]byte(raw), &ai); err == nil {
		return ai, "", nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		sub := raw[start : end+1]
		if err := json.Unmarshal([]byte(sub), &ai); err == nil {
			return ai, "", nil
		}
	}
	return artifactAIResponse{}, raw, fmt.Errorf("invalid_json")
}

func (h *Handler) failArtifact(artifactID string, errorMessage string, metadata map[string]any) {
	if strings.TrimSpace(artifactID) == "" {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metaBytes, _ := json.Marshal(metadata)
	_, _ = h.DB.Exec(`UPDATE generated_artifacts SET status='failed',error_message=$2,metadata=coalesce(metadata,'{}'::jsonb) || $3::jsonb,updated_at=now() WHERE id=$1`, artifactID, errorMessage, string(metaBytes))
}

func buildGameStoreMetadataResponse(raw string) (gameStoreMetadataResponse, error) {
	var ai gameStoreMetadataResponse
	if err := json.Unmarshal([]byte(raw), &ai); err == nil {
		return ai, nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		sub := raw[start : end+1]
		if err := json.Unmarshal([]byte(sub), &ai); err == nil {
			return ai, nil
		}
	}
	return gameStoreMetadataResponse{}, fmt.Errorf("invalid_json")
}

func (h *Handler) persistGameStoreMetadata(projectID string, payload map[string]any) {
	_, _ = h.DB.Exec(`INSERT INTO game_store_metadata (id,game_project_id,short_description,full_description,search_intent_phrases,target_audience,category_suggestions,localization_plan,play_shorts_script,play_shorts_scene_plan,ask_play_summary,updated_at)
VALUES (gen_random_uuid(),$1,$2,$3,$4::jsonb,$5::jsonb,$6::jsonb,$7::jsonb,$8,$9::jsonb,$10,now())
ON CONFLICT (game_project_id) DO UPDATE SET
  short_description=EXCLUDED.short_description,
  full_description=EXCLUDED.full_description,
  search_intent_phrases=EXCLUDED.search_intent_phrases,
  target_audience=EXCLUDED.target_audience,
  category_suggestions=EXCLUDED.category_suggestions,
  localization_plan=EXCLUDED.localization_plan,
  play_shorts_script=EXCLUDED.play_shorts_script,
  play_shorts_scene_plan=EXCLUDED.play_shorts_scene_plan,
  ask_play_summary=EXCLUDED.ask_play_summary,
  updated_at=now()`,
		projectID,
		payload["short_description"],
		payload["full_description"],
		stringifyJSONValue(payload["search_intent_phrases"]),
		stringifyJSONValue(payload["target_audience"]),
		stringifyJSONValue(payload["category_suggestions"]),
		stringifyJSONValue(payload["localization_plan"]),
		payload["play_shorts_script"],
		stringifyJSONValue(payload["play_shorts_scene_plan"]),
		payload["ask_play_summary"],
	)
}

func stringifyJSONValue(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
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
	isPriv, credits, _ := h.userCreditsAndRole(claims.Sub)
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
			writeJSON(w, 200, map[string]any{"artifact_id": existingID, "status": "completed", "file_count": fileCount, "existing": true, "credits_charged": false})
			return
		}
	}
	if !isPriv && credits < 1 {
		writeJSON(w, 402, insufficientOutputsResponse())
		return
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
		if err := rows.Scan(&tt, &out); err != nil {
			h.failArtifact(artifactID, "db_failed", map[string]any{"credits_charged": false})
			writeJSON(w, 500, map[string]any{"error": "db_failed", "detail": "db_failed", "artifact_id": artifactID, "credits_charged": false})
			return
		}
		decoded := decodeJSONB(out)
		payload[tt] = decoded
		if tt == "file_plan" {
			if m, ok := decoded["output"].(map[string]any); ok {
				if files, ok := m["files"].([]any); ok {
					filePlan = files
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		h.failArtifact(artifactID, "db_failed", map[string]any{"credits_charged": false})
		writeJSON(w, 500, map[string]any{"error": "db_failed", "detail": "db_failed", "artifact_id": artifactID, "credits_charged": false})
		return
	}
	if len(filePlan) == 0 {
		h.failArtifact(artifactID, "missing_file_plan", map[string]any{"credits_charged": false})
		writeJSON(w, 400, map[string]any{"error": "missing_file_plan", "detail": "missing_file_plan", "artifact_id": artifactID, "credits_charged": false})
		return
	}
	promptBytes, _ := json.Marshal(map[string]any{"contract_version": "5.3", "project_id": projectID, "file_plan": filePlan, "runtime_payload": payload})
	model := strings.TrimSpace(os.Getenv("TOGETHER_MODEL_GAME_CODE"))
	if model == "" {
		model = strings.TrimSpace(os.Getenv("TOGETHER_MODEL_GAME_CODE"))
	}
	if model == "" {
		model = strings.TrimSpace(os.Getenv("TOGETHER_MODEL_GAME_CODE"))
	}
	if model == "" {
		model = strings.TrimSpace(os.Getenv("TOGETHER_MODEL_GAME_DESIGN"))
	}
	if model == "" {
		model = "meta-llama/Llama-3.3-70B-Instruct-Turbo"
	}
	timeoutSeconds := 120
	if v := strings.TrimSpace(os.Getenv("TOGETHER_ARTIFACT_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSeconds = n
		}
	}
	maxTokens := 3000
	if v := strings.TrimSpace(os.Getenv("TOGETHER_ARTIFACT_MAX_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxTokens = n
		}
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	out, callErr := h.callTogetherWithSystemTimeoutAndMaxTokens(model, artifactSystemPrompt, string(promptBytes), timeout, maxTokens)
	if callErr != nil && strings.Contains(strings.ToLower(callErr.Error()), "timeout") {
		fallback := strings.TrimSpace(os.Getenv("TOGETHER_MODEL_GAME_CODE"))
		if fallback == "" || fallback == model {
			fallback = strings.TrimSpace(os.Getenv("TOGETHER_MODEL_GAME_DESIGN"))
		}
		if fallback != "" && fallback != model {
			out, callErr = h.callTogetherWithSystemTimeoutAndMaxTokens(fallback, artifactSystemPrompt, string(promptBytes), timeout, maxTokens)
		}
	}
	if callErr != nil {
		h.failArtifact(artifactID, shortError(callErr.Error()), map[string]any{"credits_charged": false})
		writeJSON(w, 502, map[string]any{"error": "generation_failed", "detail": shortError(callErr.Error()), "artifact_id": artifactID, "credits_charged": false})
		return
	}
	ai, rawFallback, err := buildArtifactAIResponse(out)
	if err != nil {
		h.failArtifact(artifactID, "invalid_json", map[string]any{"raw_ai_output": out, "credits_charged": false})
		writeJSON(w, 502, map[string]any{"error": "invalid_generation_json", "detail": "invalid_json", "raw_ai_output": rawFallback, "artifact_id": artifactID, "credits_charged": false})
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

	tx, txErr := h.DB.Begin()
	if txErr != nil {
		h.failArtifact(artifactID, "db_failed", map[string]any{"credits_charged": false})
		writeJSON(w, 500, map[string]any{"error": "db_failed", "detail": "db_failed", "artifact_id": artifactID, "credits_charged": false})
		return
	}
	txClosed := false
	defer func() {
		if !txClosed {
			_ = tx.Rollback()
		}
	}()
	rollbackThenFail := func(errorMessage string, metadata map[string]any) {
		_ = tx.Rollback()
		txClosed = true
		h.failArtifact(artifactID, errorMessage, metadata)
	}
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
		c, contentSafe := sanitizeArtifactContent(f.Content)
		if !contentSafe {
			ai.Warnings = append(ai.Warnings, "skipped "+f.Path+": unsafe_content")
			continue
		}
		if _, err := tx.Exec(`INSERT INTO generated_files (id,artifact_id,runtime_project_id,path,language,purpose,action,content,content_hash) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,md5($8))`, newID(), artifactID, projectID, path, f.Language, f.Purpose, a, c); err == nil {
			count++
		}
	}
	if count == 0 {
		rollbackThenFail("no_safe_files_generated", map[string]any{"warnings": ai.Warnings, "credits_charged": false})
		writeJSON(w, 400, map[string]any{"error": "no_safe_files_generated", "detail": "no_safe_files_generated", "artifact_id": artifactID, "credits_charged": false})
		return
	}
	if !isPriv {
		if err := h.applyCreditChargeTxWithReason(tx, claims.Sub, claims.Email, "artifact_generation"); err != nil {
			rollbackThenFail("insufficient_credits", map[string]any{"credits_charged": false})
			writeJSON(w, 402, insufficientOutputsResponse())
			return
		}
	}
	meta, _ := json.Marshal(map[string]any{"warnings": ai.Warnings, "next_steps": ai.NextSteps})
	if _, err := tx.Exec(`UPDATE generated_artifacts SET status='completed',title=$2,summary=$3,file_count=$4,zip_ready=true,metadata=$5::jsonb,updated_at=now() WHERE id=$1`, artifactID, ai.ArtifactTitle, ai.ArtifactSummary, count, string(meta)); err != nil {
		rollbackThenFail("db_failed", map[string]any{"credits_charged": false})
		writeJSON(w, 500, map[string]any{"error": "db_failed", "detail": "db_failed", "artifact_id": artifactID, "credits_charged": false})
		return
	}
	if err := tx.Commit(); err != nil {
		txClosed = true
		h.failArtifact(artifactID, "db_failed", map[string]any{"credits_charged": false})
		writeJSON(w, 500, map[string]any{"error": "db_failed", "detail": "db_failed", "artifact_id": artifactID, "credits_charged": false})
		return
	}
	txClosed = true

	storeInput, _ := json.Marshal(map[string]any{
		"project_id":       projectID,
		"artifact_title":   ai.ArtifactTitle,
		"artifact_summary": ai.ArtifactSummary,
		"warnings":         ai.Warnings,
		"next_steps":       ai.NextSteps,
		"runtime_payload":  payload,
	})
	storeOut, storeErr := h.callTogetherWithSystemTimeoutAndMaxTokens(firstEnv("TOGETHER_MODEL_GAME_DESIGN", "TOGETHER_MODEL_GAME_CODE"), gameStoreMetadataPrompt, string(storeInput), timeout, 2000)
	if storeErr == nil {
		if metaAI, parseErr := buildGameStoreMetadataResponse(storeOut); parseErr == nil {
			h.persistGameStoreMetadata(projectID, map[string]any{
				"short_description":      metaAI.ShortDescription,
				"full_description":       metaAI.FullDescription,
				"search_intent_phrases":  metaAI.SearchIntentPhrases,
				"target_audience":        metaAI.TargetAudience,
				"category_suggestions":   metaAI.CategorySuggestions,
				"localization_plan":      metaAI.LocalizationPlan,
				"play_shorts_script":     metaAI.PlayShortsScript,
				"play_shorts_scene_plan": metaAI.PlayShortsScenePlan,
				"ask_play_summary":       metaAI.AskPlaySummary,
			})
		}
	}
	writeJSON(w, 200, map[string]any{"artifact_id": artifactID, "status": "completed", "file_count": count, "warnings": ai.Warnings, "credits_charged": !isPriv})
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
		if err := rows.Scan(&id, &rpid, &em, &st, &at, &title, &summary, &fc, &zr, &er, &meta, &ca, &ua); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db_failed"})
			return
		}
		out = append(out, map[string]any{"id": id.String, "runtime_project_id": rpid.String, "status": st.String, "artifact_type": at.String, "title": title.String, "summary": summary.String, "file_count": fc.Int64, "zip_ready": zr.Bool, "error_message": er, "metadata": decodeJSONB(meta), "created_at": ca, "updated_at": ua})
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
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
	if err := h.DB.QueryRow(`SELECT user_email FROM generated_artifacts WHERE id=$1`, strings.Split(id, "/")[0]).Scan(&owner); err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	isPriv, _, _ := h.userCreditsAndRole(claims.Sub)
	if owner != claims.Email && !isPriv {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return
	}
	rows, err := h.DB.Query(`SELECT id,path,language,purpose,action,left(content,2000) FROM generated_files WHERE artifact_id=$1 ORDER BY path`, strings.Split(id, "/")[0])
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()
	files := []map[string]any{}
	for rows.Next() {
		var i, p, l, pu, a, c string
		if err := rows.Scan(&i, &p, &l, &pu, &a, &c); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db_failed"})
			return
		}
		files = append(files, map[string]any{"id": i, "path": p, "language": l, "purpose": pu, "action": a, "content_excerpt": c})
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
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
	if len(seg) < 3 || seg[1] != "files" {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	aid, fid := seg[0], seg[2]
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
	rows, err := h.DB.Query(`SELECT path,content,language,purpose,action FROM generated_files WHERE artifact_id=$1`, aid)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	defer rows.Close()
	type rf struct{ P, C, L, Pu, A string }
	var fs []rf
	for rows.Next() {
		var f rf
		if err := rows.Scan(&f.P, &f.C, &f.L, &f.Pu, &f.A); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db_failed"})
			return
		}
		fs = append(fs, f)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].P < fs[j].P })
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, f := range fs {
		zf, _ := zw.Create(f.P)
		_, _ = zf.Write([]byte(f.C))
	}
	hasReadme := false
	for _, f := range fs {
		if strings.EqualFold(f.P, "README.md") {
			hasReadme = true
			break
		}
	}
	if !hasReadme {
		readme, _ := zw.Create("README.md")
		_, _ = readme.Write([]byte("# " + title + "\n\n" + summary + "\n"))
	}
	manifest, _ := zw.Create("koschei-artifact-manifest.json")
	m, _ := json.MarshalIndent(map[string]any{"artifact_id": aid, "generated_at": time.Now().UTC(), "file_count": len(fs), "title": title, "summary": summary, "safety_note": "Generated by Koschei. Review before production use."}, "", "  ")
	_, _ = manifest.Write(m)
	_ = zw.Close()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=artifact-%s.zip", aid))
	_, _ = w.Write(buf.Bytes())
}
