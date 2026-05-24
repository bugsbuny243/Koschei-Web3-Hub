package handlers

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type createRuntimeProjectRequest struct{ Email, Title, Prompt string }

type ownerRuntimeStatusReq struct {
	Status string `json:"status"`
}

type runtimeBlueprint struct {
	ProjectTitle           string   `json:"project_title"`
	ProjectType            string   `json:"project_type"`
	UserIntent             string   `json:"user_intent"`
	MVPScope               []string `json:"mvp_scope"`
	RequiredInfrastructure []string `json:"required_infrastructure"`
	Architecture           []string `json:"architecture"`
	FilePlan               []string `json:"file_plan"`
	APIPlan                []string `json:"api_plan"`
	DatabasePlan           []string `json:"database_plan"`
	AIModelUsage           []string `json:"ai_model_usage"`
	BuildSteps             []string `json:"build_steps"`
	Risks                  []string `json:"risks"`
	ReviewChecklist        []string `json:"review_checklist"`
	DeliveryPackage        []string `json:"delivery_package"`
	NextAction             string   `json:"next_action"`
	RawAIOutput            string   `json:"raw_ai_output,omitempty"`
}

const runtimeSystemPrompt = `You are Koschei Runtime Factory.
Default language is Turkish.
You turn a user idea into a real production blueprint.
Do not act like a normal chatbot.
Be practical, technical, and production-focused.
For serious game/app/web/software ideas, produce MVP-first plan.
Do not promise instant full clone.
If the idea resembles PUBG, GTA, TikTok, YouTube, marketplace, SaaS, or social app:
- make it original/IP-safe
- explain realistic MVP
- list infrastructure
- list technical risks
- list build sequence
Respond in JSON with keys: project_title, project_type, user_intent, mvp_scope, required_infrastructure, architecture, file_plan, api_plan, database_plan, ai_model_usage, build_steps, risks, review_checklist, delivery_package, next_action.`

func (h *Handler) CreateRuntimeProject(w http.ResponseWriter, r *http.Request) {
	if !h.Limiter.allow("runtime-project:"+clientIP(r), 20, 10_000_000_000) {
		writeJSON(w, 429, map[string]string{"error": "rate limited"})
		return
	}
	var req createRuntimeProjectRequest
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	if !togetherAIEnabled() || strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) == "" {
		writeJSON(w, 503, map[string]any{"error": "ai_provider_not_configured", "credits_charged": false})
		return
	}
	isPrivileged, credits, err := h.userCreditsAndRole(claims.Sub)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	if !isPrivileged && credits <= 0 {
		writeJSON(w, 402, map[string]string{"error": "insufficient_credits"})
		return
	}
	projectID := newID()
	title := normalizeRuntimeTitle(req.Title, req.Prompt)
	result, runErr := h.runRuntimeProductionPipeline(projectID, claims.Sub, claims.Email, title, req.Prompt, isPrivileged)
	if runErr != nil {
		writeJSON(w, 502, map[string]any{"error": "runtime_generation_failed", "detail": shortError(runErr.Error()), "credits_charged": false})
		return
	}
	writeJSON(w, 201, result)
}

func togetherAIEnabled() bool {
	if strings.ToLower(strings.TrimSpace(os.Getenv("TOGETHER_AI_ENABLED"))) == "true" {
		return true
	}
	return strings.ToLower(strings.TrimSpace(os.Getenv("TOGETHER_ENABLED"))) == "true"
}
func normalizeRuntimeTitle(title, prompt string) string {
	clean := strings.TrimSpace(title)
	if clean == "" || strings.HasPrefix(clean, "Project ") {
		w := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(prompt), -1)
		if len(w) > 8 {
			w = w[:8]
		}
		if len(w) == 0 {
			return "Runtime Project"
		}
		return strings.Join(w, " ")
	}
	return clean
}
func (h *Handler) runRuntimeProductionPipeline(projectID, authSub, email, title, prompt string, isPrivileged bool) (map[string]any, error) {
	taskOrder := []string{"intake", "blueprint", "architecture", "file_plan", "build_steps", "review", "delivery"}
	tx, err := h.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO runtime_projects (id,email,title,prompt,status) VALUES ($1,$2,$3,$4,'running')`, projectID, email, title, prompt); err != nil {
		return nil, err
	}
	for _, m := range []string{"Project created", "Runtime AI pipeline started"} {
		if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, m); err != nil {
			return nil, err
		}
	}
	for _, t := range taskOrder {
		inp, _ := json.Marshal(map[string]any{"title": title, "prompt": prompt, "stage": t})
		if _, err = tx.Exec(`INSERT INTO runtime_tasks (id,project_id,email,task_type,status,input_json,output_json) VALUES ($1,$2,$3,$4,'queued',$5,'{}'::jsonb)`, newID(), projectID, email, t, inp); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	aiOut, err := h.callTogetherRuntimeBlueprint(prompt)
	if err != nil {
		_ = h.markRuntimeFailed(projectID, "Runtime AI pipeline failed: "+shortError(err.Error()), "blueprint", err.Error())
		return nil, err
	}
	bp := buildRuntimeBlueprint(aiOut, prompt)
	if err := h.completeRuntimePipeline(projectID, authSub, email, bp, isPrivileged); err != nil {
		_ = h.markRuntimeFailed(projectID, "Runtime AI pipeline failed: "+shortError(err.Error()), "delivery", err.Error())
		return nil, err
	}
	return h.fetchRuntimeResponse(projectID, bp)
}
func (h *Handler) callTogetherRuntimeBlueprint(prompt string) (string, error) {
	model := firstEnv("TOGETHER_MODEL_REASONING", "TOGETHER_MODEL_COMPLEX", "TOGETHER_MODEL")
	if strings.TrimSpace(model) == "" {
		return "", errors.New("together model is empty")
	}
	return h.callTogetherChat(model, "reason", "SYSTEM OVERRIDE:\n"+runtimeSystemPrompt+"\n\n"+prompt)
}
func buildRuntimeBlueprint(raw, prompt string) runtimeBlueprint {
	var bp runtimeBlueprint
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &bp); err != nil {
		bp.RawAIOutput = raw
	}
	if strings.TrimSpace(bp.ProjectTitle) == "" {
		bp.ProjectTitle = normalizeRuntimeTitle("", prompt)
	}
	if strings.TrimSpace(bp.ProjectType) == "" {
		bp.ProjectType = "software_mvp"
	}
	if strings.TrimSpace(bp.UserIntent) == "" {
		bp.UserIntent = prompt
	}
	if len(bp.MVPScope) == 0 {
		bp.MVPScope = []string{"MVP hedefleri netleştir", "Çekirdek özellikleri çıkar", "İlk sürümü üret"}
	}
	if len(bp.RequiredInfrastructure) == 0 {
		bp.RequiredInfrastructure = []string{"backend api", "database", "auth", "monitoring"}
	}
	if len(bp.Architecture) == 0 {
		bp.Architecture = []string{"istemci", "api servisi", "veri katmanı"}
	}
	if len(bp.FilePlan) == 0 {
		bp.FilePlan = []string{"src/app", "src/api", "migrations"}
	}
	if len(bp.BuildSteps) == 0 {
		bp.BuildSteps = []string{"Analiz", "MVP geliştirme", "Test ve yayın"}
	}
	if len(bp.Risks) == 0 {
		bp.Risks = []string{"kapsam şişmesi", "altyapı maliyeti"}
	}
	if len(bp.ReviewChecklist) == 0 {
		bp.ReviewChecklist = []string{"testler geçiyor", "güvenlik kontrolleri tamam"}
	}
	if len(bp.DeliveryPackage) == 0 {
		bp.DeliveryPackage = []string{"kaynak kod", "deploy notları", "test raporu"}
	}
	if strings.TrimSpace(bp.NextAction) == "" {
		bp.NextAction = "MVP backlog’unu onayla ve sprint 1’i başlat."
	}
	return bp
}
func (h *Handler) markRuntimeFailed(projectID, msg, taskType, taskErr string) error {
	_, _ = h.DB.Exec(`UPDATE runtime_projects SET status='failed', updated_at=NOW() WHERE id=$1`, projectID)
	_, _ = h.DB.Exec(`UPDATE runtime_tasks SET status='failed', error=$3, updated_at=NOW() WHERE project_id=$1 AND task_type=$2`, projectID, taskType, shortError(taskErr))
	_, err := h.DB.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'error',$3)`, newID(), projectID, msg)
	return err
}
func (h *Handler) completeRuntimePipeline(projectID, authSub, email string, bp runtimeBlueprint, isPrivileged bool) error {
	taskOutputs := map[string]map[string]any{
		"intake":       {"summary": bp.UserIntent, "project_type": bp.ProjectType, "user_intent": bp.UserIntent},
		"blueprint":    {"project_title": bp.ProjectTitle, "mvp_scope": bp.MVPScope, "next_action": bp.NextAction},
		"architecture": {"required_infrastructure": bp.RequiredInfrastructure, "architecture": bp.Architecture},
		"file_plan":    {"file_plan": bp.FilePlan},
		"build_steps":  {"build_steps": bp.BuildSteps},
		"review":       {"risks": bp.Risks, "review_checklist": bp.ReviewChecklist},
		"delivery":     {"delivery_package": bp.DeliveryPackage, "status": "ready"},
	}
	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range []string{"intake", "blueprint", "architecture", "file_plan", "build_steps", "review", "delivery"} {
		out, _ := json.Marshal(taskOutputs[t])
		if _, err = tx.Exec(`UPDATE runtime_tasks SET status='completed', output_json=$3, error=NULL, updated_at=NOW() WHERE project_id=$1 AND task_type=$2`, projectID, t, out); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Task completed: "+t); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE runtime_projects SET status='completed', title=$2, updated_at=NOW() WHERE id=$1`, projectID, bp.ProjectTitle); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "AI blueprint generated"); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Runtime workflow completed"); err != nil {
		return err
	}
	if !isPrivileged {
		if err := h.applyCreditChargeTx(tx, authSub, email); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (h *Handler) fetchRuntimeResponse(projectID string, bp runtimeBlueprint) (map[string]any, error) {
	rows, err := h.DB.Query(`SELECT id,project_id,email,task_type,status,input_json,output_json,error,created_at,updated_at FROM runtime_tasks WHERE project_id=$1 ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []map[string]any
	for rows.Next() {
		var id, pid, e, tt, s string
		var in, out, e2, ca, ua any
		if err := rows.Scan(&id, &pid, &e, &tt, &s, &in, &out, &e2, &ca, &ua); err != nil {
			return nil, err
		}
		tasks = append(tasks, map[string]any{"id": id, "project_id": pid, "email": e, "task_type": tt, "status": s, "input_json": in, "output_json": out, "error": e2, "created_at": ca, "updated_at": ua})
	}
	var taskCount, logCount int
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM runtime_tasks WHERE project_id=$1`, projectID).Scan(&taskCount); err != nil {
		return nil, err
	}
	if err := h.DB.QueryRow(`SELECT COUNT(*) FROM runtime_logs WHERE project_id=$1`, projectID).Scan(&logCount); err != nil {
		return nil, err
	}
	return map[string]any{"project_id": projectID, "status": "completed", "credits_charged": true, "task_count": taskCount, "log_count": logCount, "blueprint": bp, "tasks": tasks}, nil
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
