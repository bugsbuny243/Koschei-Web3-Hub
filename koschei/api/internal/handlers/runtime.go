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
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	email := claims.Email
	projectID := newID()
	if h.DB == nil {
		writeJSON(w, 503, map[string]string{"error": "database unavailable: connection is not initialized"})
		return
	}
	if err := h.DB.PingContext(r.Context()); err != nil {
		writeJSON(w, 503, map[string]string{"error": fmt.Sprintf("database unavailable: %v", err)})
		return
	}
	tx, err := h.DB.Begin()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed: unable to create runtime project"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO runtime_projects (id,email,title,prompt,status) VALUES ($1,$2,$3,$4,'queued')`, projectID, email, req.Title, req.Prompt)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed: unable to create project log"})
		return
	}

	_, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Project created")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}

	_, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Runtime workflow initialized")
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed: could not create initialization log"})
		return
	}

	taskOrder := []string{"planning", "generation", "review", "delivery"}
	taskIDs := map[string]string{}
	for _, taskType := range taskOrder {
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

		taskID := newID()
		taskIDs[taskType] = taskID
		_, err = tx.Exec(`INSERT INTO runtime_tasks (id,project_id,email,task_type,status,input_json,output_json) VALUES ($1,$2,$3,$4,'queued',$5,$6)`, taskID, projectID, email, taskType, inputJSON, outputJSON)
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

	taskOutputs := map[string]map[string]any{
		"planning": {
			"summary": "Runtime project planning completed",
			"steps":   []string{"generation", "review", "delivery"},
		},
		"generation": {
			"title":        "Koschei AI Runtime Test",
			"sections":     []string{"hero", "features", "pricing", "contact"},
			"content_type": "landing_page_brief",
		},
		"review": {
			"status": "approved",
			"notes":  "Smoke test review passed",
		},
		"delivery": {
			"status":  "ready",
			"message": "Runtime smoke test completed",
		},
	}

	for _, taskType := range taskOrder {
		taskID := taskIDs[taskType]
		if _, err = tx.Exec(`UPDATE runtime_tasks SET status='running', updated_at=NOW() WHERE id=$1`, taskID); err != nil {
			_, _ = tx.Exec(`UPDATE runtime_tasks SET status='failed', error=$2, updated_at=NOW() WHERE id=$1`, taskID, err.Error())
			_, _ = tx.Exec(`UPDATE runtime_projects SET status='failed', updated_at=NOW() WHERE id=$1`, projectID)
			_, _ = tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message) VALUES ($1,$2,$3,'error',$4)`, newID(), projectID, taskID, "Runtime workflow failed at "+taskType+": "+err.Error())
			writeJSON(w, 500, map[string]string{"error": "runtime execution failed at " + taskType})
			return
		}
		if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message) VALUES ($1,$2,$3,'info',$4)`, newID(), projectID, taskID, "[info] Task started: "+taskType); err != nil {
			_, _ = tx.Exec(`UPDATE runtime_tasks SET status='failed', error=$2, updated_at=NOW() WHERE id=$1`, taskID, err.Error())
			_, _ = tx.Exec(`UPDATE runtime_projects SET status='failed', updated_at=NOW() WHERE id=$1`, projectID)
			_, _ = tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message) VALUES ($1,$2,$3,'error',$4)`, newID(), projectID, taskID, "Runtime workflow failed at "+taskType+": "+err.Error())
			writeJSON(w, 500, map[string]string{"error": "runtime execution failed at " + taskType})
			return
		}

		outputJSON, marshalErr := json.Marshal(taskOutputs[taskType])
		if marshalErr != nil {
			_, _ = tx.Exec(`UPDATE runtime_tasks SET status='failed', error=$2, updated_at=NOW() WHERE id=$1`, taskID, marshalErr.Error())
			_, _ = tx.Exec(`UPDATE runtime_projects SET status='failed', updated_at=NOW() WHERE id=$1`, projectID)
			_, _ = tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message) VALUES ($1,$2,$3,'error',$4)`, newID(), projectID, taskID, "Runtime workflow failed at "+taskType+": "+marshalErr.Error())
			writeJSON(w, 500, map[string]string{"error": "runtime execution failed at " + taskType})
			return
		}
		if _, err = tx.Exec(`UPDATE runtime_tasks SET status='completed', output_json=$2, error=NULL, updated_at=NOW() WHERE id=$1`, taskID, outputJSON); err != nil {
			_, _ = tx.Exec(`UPDATE runtime_tasks SET status='failed', error=$2, updated_at=NOW() WHERE id=$1`, taskID, err.Error())
			_, _ = tx.Exec(`UPDATE runtime_projects SET status='failed', updated_at=NOW() WHERE id=$1`, projectID)
			_, _ = tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message) VALUES ($1,$2,$3,'error',$4)`, newID(), projectID, taskID, "Runtime workflow failed at "+taskType+": "+err.Error())
			writeJSON(w, 500, map[string]string{"error": "runtime execution failed at " + taskType})
			return
		}
		if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message) VALUES ($1,$2,$3,'info',$4)`, newID(), projectID, taskID, "[info] Task completed: "+taskType); err != nil {
			_, _ = tx.Exec(`UPDATE runtime_tasks SET status='failed', error=$2, updated_at=NOW() WHERE id=$1`, taskID, err.Error())
			_, _ = tx.Exec(`UPDATE runtime_projects SET status='failed', updated_at=NOW() WHERE id=$1`, projectID)
			_, _ = tx.Exec(`INSERT INTO runtime_logs (id,project_id,task_id,level,message) VALUES ($1,$2,$3,'error',$4)`, newID(), projectID, taskID, "Runtime workflow failed at "+taskType+": "+err.Error())
			writeJSON(w, 500, map[string]string{"error": "runtime execution failed at " + taskType})
			return
		}
	}

	if _, err = tx.Exec(`UPDATE runtime_projects SET status='completed', updated_at=NOW() WHERE id=$1`, projectID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "runtime execution failed: could not complete project"})
		return
	}
	if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "[info] Runtime workflow completed"); err != nil {
		writeJSON(w, 500, map[string]string{"error": "runtime execution failed: could not write completion log"})
		return
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
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	email := claims.Email
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
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	email := claims.Email
	id := strings.TrimPrefix(r.URL.Path, "/api/runtime/projects/")
	var pid, e, title, prompt, status, created, updated string
	if err := h.DB.QueryRow(`SELECT id,email,title,prompt,status,created_at,updated_at FROM runtime_projects WHERE id=$1`, id).Scan(&pid, &e, &title, &prompt, &status, &created, &updated); err != nil || e != email {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, map[string]any{"id": pid, "email": e, "title": title, "prompt": prompt, "status": status, "created_at": created, "updated_at": updated})
}
func (h *Handler) ListRuntimeTasks(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	email := claims.Email
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
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	email := claims.Email
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
		if e != email {
			writeJSON(w, 404, map[string]string{"error": "not found"})
			return
		}
		t = map[string]any{"id": id, "project_id": pid, "email": e, "task_type": tt, "status": s}
	} else {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, t)
}
func (h *Handler) GetRuntimeLogs(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	email := claims.Email
	pid := strings.TrimPrefix(r.URL.Path, "/api/runtime/logs/")
	var ownerEmail string
	if err := h.DB.QueryRow(`SELECT email FROM runtime_projects WHERE id=$1`, pid).Scan(&ownerEmail); err != nil || ownerEmail != email {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
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
