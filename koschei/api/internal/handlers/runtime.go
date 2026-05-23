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

	_, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Project created")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	for _, taskType := range []string{"planning", "generation", "review", "delivery"} {
		inputJSON, marshalErr := json.Marshal(map[string]string{
			"title":  req.Title,
			"prompt": req.Prompt,
			"stage":  taskType,
		})
		if marshalErr != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		outputJSON, marshalErr := json.Marshal(map[string]any{})
		if marshalErr != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}

		_, err = tx.Exec(`INSERT INTO runtime_tasks (id,project_id,email,task_type,status,input_json,output_json) VALUES ($1,$2,$3,$4,'queued',$5,$6)`, newID(), projectID, req.Email, taskType, inputJSON, outputJSON)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		_, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Task created: "+taskType)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
	}

	var createdTasks []map[string]any
	rows, err := tx.Query(`SELECT id,project_id,email,task_type,status,input_json,output_json,error,created_at,updated_at FROM runtime_tasks WHERE project_id=$1 ORDER BY created_at ASC`, projectID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, pid, e, taskType, status string
		var inputJSON, outputJSON, runtimeErr, created, updated any
		if err := rows.Scan(&id, &pid, &e, &taskType, &status, &inputJSON, &outputJSON, &runtimeErr, &created, &updated); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		createdTasks = append(createdTasks, map[string]any{"id": id, "project_id": pid, "email": e, "task_type": taskType, "status": status, "input_json": inputJSON, "output_json": outputJSON, "error": runtimeErr, "created_at": created, "updated_at": updated})
	}

	var taskCount, logCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM runtime_tasks WHERE project_id=$1`, projectID).Scan(&taskCount); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM runtime_logs WHERE project_id=$1`, projectID).Scan(&logCount); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	writeJSON(w, 201, map[string]any{"project_id": projectID, "task_count": taskCount, "log_count": logCount, "tasks": createdTasks})
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
	rows, err := h.DB.Query(`SELECT id,project_id,email,task_type,status,input_json,output_json,error,created_at,updated_at FROM runtime_tasks WHERE email=$1 ORDER BY created_at DESC`, email)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, pid, e, taskType, status string
		var inputJSON, outputJSON, runtimeErr, created, updated any
		if err := rows.Scan(&id, &pid, &e, &taskType, &status, &inputJSON, &outputJSON, &runtimeErr, &created, &updated); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		out = append(out, map[string]any{"id": id, "project_id": pid, "email": e, "task_type": taskType, "status": status, "input_json": inputJSON, "output_json": outputJSON, "error": runtimeErr, "created_at": created, "updated_at": updated})
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
	outputJSON, _ := json.Marshal(map[string]any{})
	_, err := h.DB.Exec(`UPDATE runtime_tasks SET status='queued', error=NULL, output_json=$2, updated_at=NOW() WHERE id=$1`, id, outputJSON)
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
	_, err := h.DB.Exec(`UPDATE runtime_tasks SET status='cancelled', error='cancelled by owner', updated_at=NOW() WHERE id=$1`, id)
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
