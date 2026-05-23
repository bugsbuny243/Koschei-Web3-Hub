package handlers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type createRuntimeProjectRequest struct{ Email, Title, Prompt string }

type ownerRuntimeStatusReq struct {
	Status string `json:"status"`
}

func (h *Handler) CreateRuntimeProject(w http.ResponseWriter, r *http.Request) {
	if !h.Limiter.allow("runtime-project:"+clientIP(r), 20, 10_000_000_000) {
		writeJSON(w, 429, map[string]string{"error": "rate limited"})
		return
	}
	var req createRuntimeProjectRequest
	if err := decodeJSON(r, &req); err != nil || !validEmail(req.Email) || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	projectID := newID()
	tx, err := h.DB.Begin()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO runtime_projects (id,email,title,prompt,status) VALUES ($1,$2,$3,$4,'queued')`, projectID, req.Email, req.Title, req.Prompt)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	taskTypes := []string{"planning", "generation", "review", "delivery"}
	inputJSON, _ := json.Marshal(map[string]string{"title": req.Title, "prompt": req.Prompt})
	outputJSON := "{}"
	tasks := make([]map[string]any, 0, len(taskTypes))

	for _, taskType := range taskTypes {
		taskID := newID()
		_, err = tx.Exec(`INSERT INTO runtime_tasks (id,project_id,email,task_type,tool,prompt,status,priority,input_json,output_json) VALUES ($1,$2,$3,$4,$5,$6,'queued',$7,$8::jsonb,$9::jsonb)`, taskID, projectID, req.Email, taskType, "runtime_worker", req.Prompt, 5, string(inputJSON), outputJSON)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		tasks = append(tasks, map[string]any{"id": taskID, "project_id": projectID, "email": req.Email, "task_type": taskType, "status": "queued", "input_json": json.RawMessage(inputJSON), "output_json": json.RawMessage(outputJSON)})

		_, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message) VALUES ($1,$2,$3,'info',$4)`, newID(), projectID, taskID, "Task created: "+taskType)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
	}

	logCount := 1 + len(taskTypes)
	_, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Project created")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	writeJSON(w, 201, map[string]any{
		"project": map[string]any{"id": projectID, "email": req.Email, "title": req.Title, "prompt": req.Prompt, "status": "queued"},
		"tasks": tasks,
		"log_count": logCount,
	})
}

func (h *Handler) ListRuntimeProjects(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if !validEmail(email) {
		writeJSON(w, 400, map[string]string{"error": "valid email required"})
		return
	}
	rows, err := h.DB.Query(`SELECT id,email,title,prompt,status,created_at,updated_at FROM runtime_projects WHERE email=$1 ORDER BY created_at DESC`, email)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, e, title, prompt, status, created, updated string
		if err := rows.Scan(&id, &e, &title, &prompt, &status, &created, &updated); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		out = append(out, map[string]any{"id": id, "email": e, "title": title, "prompt": prompt, "status": status, "created_at": created, "updated_at": updated})
	}
	writeJSON(w, 200, out)
}
func (h *Handler) GetRuntimeProject(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/runtime/projects/")
	var pid, e, title, prompt, status, created, updated string
	if err := h.DB.QueryRow(`SELECT id,email,title,prompt,status,created_at,updated_at FROM runtime_projects WHERE id=$1`, id).Scan(&pid, &e, &title, &prompt, &status, &created, &updated); err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, map[string]any{"id": pid, "email": e, "title": title, "prompt": prompt, "status": status, "created_at": created, "updated_at": updated})
}
func (h *Handler) ListRuntimeTasks(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if !validEmail(email) {
		writeJSON(w, 400, map[string]string{"error": "valid email required"})
		return
	}
	rows, err := h.DB.Query(`SELECT id,project_id,email,task_type,tool,prompt,status,priority,result,error,created_at,started_at,completed_at,updated_at FROM runtime_tasks WHERE email=$1 ORDER BY created_at DESC`, email)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, pid, e, taskType, tool, prompt, status string
		var priority int
		var result, error, created, updated any
		var started, completed any
		if err := rows.Scan(&id, &pid, &e, &taskType, &tool, &prompt, &status, &priority, &result, &error, &created, &started, &completed, &updated); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		out = append(out, map[string]any{"id": id, "project_id": pid, "email": e, "task_type": taskType, "tool": tool, "prompt": prompt, "status": status, "priority": priority, "result": result, "error": error, "created_at": created, "started_at": started, "completed_at": completed, "updated_at": updated})
	}
	writeJSON(w, 200, out)
}
func (h *Handler) GetRuntimeTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/runtime/tasks/")
	var t map[string]any
	rows, err := h.DB.Query(`SELECT id,project_id,email,task_type,status FROM runtime_tasks WHERE id=$1`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer rows.Close()
	if rows.Next() {
		var id, pid, e, tt, s string
		_ = rows.Scan(&id, &pid, &e, &tt, &s)
		t = map[string]any{"id": id, "project_id": pid, "email": e, "task_type": tt, "status": s}
	} else {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, t)
}
func (h *Handler) GetRuntimeLogs(w http.ResponseWriter, r *http.Request) {
	pid := strings.TrimPrefix(r.URL.Path, "/api/runtime/logs/")
	rows, err := h.DB.Query(`SELECT id,project_id,task_id,level,message,created_at FROM runtime_logs WHERE project_id=$1 ORDER BY created_at DESC`, pid)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, pid, level, msg, created string
		var taskID any
		if err := rows.Scan(&id, &pid, &taskID, &level, &msg, &created); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		out = append(out, map[string]any{"id": id, "project_id": pid, "task_id": taskID, "level": level, "message": msg, "created_at": created})
	}
	writeJSON(w, 200, out)
}

func (h *Handler) OwnerRetryRuntimeTask(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/owner/runtime/tasks/"), "/retry")
	_, err := h.DB.Exec(`UPDATE runtime_tasks SET status='queued', error=NULL, result=NULL, started_at=NULL, completed_at=NULL, updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (h *Handler) OwnerCancelRuntimeTask(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/owner/runtime/tasks/"), "/cancel")
	_, err := h.DB.Exec(`UPDATE runtime_tasks SET status='cancelled', error='cancelled by owner', completed_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (h *Handler) OwnerUpdateRuntimeTaskStatus(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/owner/runtime/tasks/"), "/status")
	var req ownerRuntimeStatusReq
	if err := decodeJSON(r, &req); err != nil || !validStatus(req.Status) {
		writeJSON(w, 400, map[string]string{"error": "invalid status"})
		return
	}
	_, err := h.DB.Exec(`UPDATE runtime_tasks SET status=$2, updated_at=NOW() WHERE id=$1`, id, req.Status)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
