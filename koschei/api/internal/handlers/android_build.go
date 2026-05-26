package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	modelrouter "koschei/api/internal/router"
)

type androidBuildRequest struct {
	ProjectID  string `json:"project_id"`
	ArtifactID string `json:"artifact_id"`
	Prompt     string `json:"prompt"`
	Tool       string `json:"tool"`
}

func (h *Handler) BuildAndroid(w http.ResponseWriter, r *http.Request) {
	var req androidBuildRequest
	if err := decodeJSON(r, &req); err != nil || req.ProjectID == "" || req.ArtifactID == "" || req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	claims, ok := userFromContext(r.Context())
	if !ok || !validEmail(claims.Email) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db failed"})
		return
	}
	defer tx.Rollback()

	var projectOwner string
	if err := tx.QueryRow(`SELECT email FROM runtime_projects WHERE id=$1`, req.ProjectID).Scan(&projectOwner); err != nil {
		status := http.StatusInternalServerError
		if err == sql.ErrNoRows {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": "project not found"})
		return
	}
	if !strings.EqualFold(projectOwner, claims.Email) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if _, err := tx.Exec(`UPDATE generated_artifacts SET build_status='pending',status='processing',updated_at=now() WHERE id=$1 AND runtime_project_id=$2`, req.ArtifactID, req.ProjectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "artifact update failed"})
		return
	}

	taskID := newID()
	input := `{"kind":"android_build"}`
	if _, err := tx.Exec(`INSERT INTO runtime_tasks (id,project_id,email,task_type,status,input_json) VALUES ($1,$2,$3,'android_build','queued',$4::jsonb)`, taskID, req.ProjectID, claims.Email, input); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "task enqueue failed"})
		return
	}

	logID := newID()
	if _, err := tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message,metadata) VALUES ($1,$2,$3,'info',$4,$5::jsonb)`, logID, req.ProjectID, taskID, "Android build request queued", `{"artifact_id":"`+req.ArtifactID+`"}`); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "log write failed"})
		return
	}
	if _, err := tx.Exec(`INSERT INTO runtime_build_logs (artifact_id,runtime_log_id,level,message,payload) VALUES ($1,$2,'info',$3,$4::jsonb)`, req.ArtifactID, logID, "Build request accepted", `{"project_id":"`+req.ProjectID+`"}`); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "build log write failed"})
		return
	}

	route := strings.TrimSpace(req.Tool)
	if route == "" {
		route = "analysis"
	}
	mr := modelrouter.ResolveModelRoute(route)
	_, _ = tx.Exec(`INSERT INTO model_route_logs (email, tool, route, model, provider, prompt, status) VALUES ($1,$2,$3,$4,$5,$6,$7)`, claims.Email, req.Tool, mr.Route, mr.Route, mr.Provider, req.Prompt, mr.Status)

	creditRes, err := tx.Exec(`UPDATE app_user_profiles SET credits = credits - 1, updated_at = now() WHERE lower(email)=lower($1) AND credits > 0`, claims.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "credit update failed"})
		return
	}
	creditRows, _ := creditRes.RowsAffected()
	if creditRows == 0 {
		writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "insufficient credits"})
		return
	}
	if _, err := tx.Exec(`INSERT INTO credit_events (email, amount, reason, event_type) VALUES ($1,-1,$2,'android_build')`, claims.Email, "android_build:"+req.ProjectID+":"+taskID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "credit event failed"})
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "task_id": taskID, "artifact_id": req.ArtifactID})
}
