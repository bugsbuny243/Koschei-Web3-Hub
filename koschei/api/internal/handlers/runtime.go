package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
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

type intakeOutput struct {
	Summary     string `json:"summary"`
	ProjectType string `json:"project_type"`
	UserIntent  string `json:"user_intent"`
}

type blueprintOutput struct {
	ProjectTitle string   `json:"project_title"`
	ProjectType  string   `json:"project_type"`
	UserIntent   string   `json:"user_intent"`
	MVPScope     []string `json:"mvp_scope"`
	NextAction   string   `json:"next_action"`
}

type architectureOutput struct {
	RequiredInfrastructure []string `json:"required_infrastructure"`
	Architecture           []string `json:"architecture"`
	APIPlan                []string `json:"api_plan"`
	DatabasePlan           []string `json:"database_plan"`
	AIModelUsage           []string `json:"ai_model_usage"`
	PlannedActions         []string `json:"planned_actions"`
}

type filePlanOutput struct {
	FilePlan       []string `json:"file_plan"`
	PlannedActions []string `json:"planned_actions"`
	FileWrites     string   `json:"file_writes"`
}

type reviewOutput struct {
	Risks           []string `json:"risks"`
	ReviewChecklist []string `json:"review_checklist"`
}

type deliveryOutput struct {
	DeliveryPackage []string `json:"delivery_package"`
	Status          string   `json:"status"`
	PlannedActions  []string `json:"planned_actions"`
	Saveable        bool     `json:"saveable"`
}

const runtimeSystemPrompt = `You are Koschei Runtime Factory.
Default language is Turkish.
Return ONLY valid JSON.
Do not wrap in markdown.
Do not use ` + "```json" + ` fences.
Do not add explanation outside JSON.
Be concise.
Each array should contain 3 to 7 items.
Do not write huge paragraphs.
Keep total response compact but useful.
For PUBG-like, GTA-like, TikTok-like, marketplace-like, YouTube-like:
- do not refuse
- do not promise instant full clone
- make it original/IP-safe
- MVP first
- list real infrastructure
- list risks
- list build sequence
Required JSON shape:
{
  "project_title": "string",
  "project_type": "string",
  "user_intent": "string",
  "mvp_scope": ["string"],
  "required_infrastructure": ["string"],
  "architecture": ["string"],
  "file_plan": ["string"],
  "api_plan": ["string"],
  "database_plan": ["string"],
  "ai_model_usage": ["string"],
  "build_steps": ["string"],
  "risks": ["string"],
  "review_checklist": ["string"],
  "delivery_package": ["string"],
  "next_action": "string"
}`

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
	aiOut, err := h.callTogetherRuntimeBlueprint(projectID, prompt)
	if err != nil {
		timeoutFailure := isTimeoutError(err)
		failureMsg := "Runtime AI pipeline failed: " + shortError(err.Error())
		responseErr := err
		if timeoutFailure {
			failureMsg = "Runtime AI pipeline failed: provider timeout"
			responseErr = errors.New("runtime_generation_failed: Together provider timeout. Credits not charged.")
		}
		_ = h.markRuntimeFailed(projectID, failureMsg, "blueprint", err.Error())
		return nil, responseErr
	}
	bp := buildRuntimeBlueprint(aiOut, prompt)
	creditCharged, err := h.completeRuntimePipeline(projectID, authSub, email, bp, isPrivileged)
	if err != nil {
		_ = h.markRuntimeFailed(projectID, "Runtime AI pipeline failed: "+shortError(err.Error()), "delivery", err.Error())
		return nil, err
	}
	return h.fetchRuntimeResponse(projectID, bp, creditCharged)
}
func (h *Handler) callTogetherRuntimeBlueprint(projectID, prompt string) (string, error) {
	model := firstEnv("TOGETHER_MODEL_RUNTIME", "TOGETHER_MODEL_REASONING", "TOGETHER_MODEL_COMPLEX", "TOGETHER_MODEL")
	if strings.TrimSpace(model) == "" {
		return "", errors.New("together model is empty")
	}
	timeout := 120 * time.Second
	if v := strings.TrimSpace(os.Getenv("TOGETHER_RUNTIME_TIMEOUT_SECONDS")); v != "" {
		if parsed, parseErr := time.ParseDuration(v + "s"); parseErr == nil && parsed >= 5*time.Second {
			timeout = parsed
		}
	}
	maxTokens := 2200
	if v := strings.TrimSpace(os.Getenv("TOGETHER_RUNTIME_MAX_TOKENS")); v != "" {
		var parsed int
		if _, scanErr := fmt.Sscanf(v, "%d", &parsed); scanErr == nil && parsed >= 200 {
			maxTokens = parsed
		}
	}
	out, err := h.callTogetherWithSystemTimeoutAndMaxTokens(model, runtimeSystemPrompt, prompt, timeout, maxTokens)
	if err == nil {
		return out, nil
	}
	if !isTimeoutError(err) {
		return "", err
	}
	fmt.Println("Runtime AI provider timeout on model:", model)
	_, _ = h.DB.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'error',$3)`, newID(), projectID, "Runtime AI provider timeout on model: "+model)
	fallbackModel := firstEnv("TOGETHER_MODEL", "TOGETHER_MODEL_COMPLEX", "TOGETHER_MODEL_REASONING")
	if strings.TrimSpace(fallbackModel) == "" || fallbackModel == model {
		return "", err
	}
	fmt.Println("Runtime AI fallback started with model:", fallbackModel)
	_, _ = h.DB.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Runtime AI fallback started with model: "+fallbackModel)
	fallbackOut, fallbackErr := h.callTogetherWithSystemTimeoutAndMaxTokens(fallbackModel, runtimeSystemPrompt, prompt, timeout, maxTokens)
	if fallbackErr != nil {
		return "", fallbackErr
	}
	fmt.Println("Runtime AI fallback succeeded")
	_, _ = h.DB.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Runtime AI fallback succeeded with model: "+fallbackModel)
	return fallbackOut, nil
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "client.timeout exceeded")
}
func buildRuntimeBlueprint(raw, prompt string) runtimeBlueprint {
	var bp runtimeBlueprint
	raw = strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(raw), &bp); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			if extractErr := json.Unmarshal([]byte(raw[start:end+1]), &bp); extractErr == nil {
				goto defaults
			}
		}
		if raw != "" {
			bp.RawAIOutput = raw
			bp.ReviewChecklist = append(bp.ReviewChecklist, "raw_ai_output_available")
			bp.DeliveryPackage = append(bp.DeliveryPackage, "raw_ai_output_available")
		}
	}
defaults:
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
func (h *Handler) completeRuntimePipeline(projectID, authSub, email string, bp runtimeBlueprint, isPrivileged bool) (bool, error) {
	plannedActions := []string{"create_file_plan", "propose_api_routes", "propose_db_migration", "estimate_infra", "prepare_delivery_package"}
	intakeOut := intakeOutput{Summary: bp.UserIntent, ProjectType: bp.ProjectType, UserIntent: bp.UserIntent}
	blueprintOut := blueprintOutput{ProjectTitle: bp.ProjectTitle, ProjectType: bp.ProjectType, UserIntent: bp.UserIntent, MVPScope: bp.MVPScope, NextAction: bp.NextAction}
	architectureOut := architectureOutput{
		RequiredInfrastructure: bp.RequiredInfrastructure,
		Architecture:           bp.Architecture,
		APIPlan:                bp.APIPlan,
		DatabasePlan:           bp.DatabasePlan,
		AIModelUsage:           bp.AIModelUsage,
		PlannedActions:         []string{"propose_api_routes", "propose_db_migration", "estimate_infra"},
	}
	filePlanOut := filePlanOutput{
		FilePlan:       bp.FilePlan,
		PlannedActions: []string{"create_file_plan"},
		FileWrites:     "planned_only_no_writes",
	}
	reviewOut := reviewOutput{Risks: bp.Risks, ReviewChecklist: bp.ReviewChecklist}
	deliveryOut := deliveryOutput{
		DeliveryPackage: bp.DeliveryPackage,
		Status:          "ready",
		PlannedActions:  []string{"prepare_delivery_package"},
		Saveable:        true,
	}
	if strings.TrimSpace(bp.RawAIOutput) != "" {
		deliveryOut.Saveable = false
		deliveryOut.Status = "review_needed"
	}

	taskOutputs := map[string]map[string]any{
		"intake":       {"summary": intakeOut.Summary, "project_type": intakeOut.ProjectType, "user_intent": intakeOut.UserIntent},
		"blueprint":    {"project_title": blueprintOut.ProjectTitle, "project_type": blueprintOut.ProjectType, "user_intent": blueprintOut.UserIntent, "mvp_scope": blueprintOut.MVPScope, "next_action": blueprintOut.NextAction},
		"architecture": {"required_infrastructure": architectureOut.RequiredInfrastructure, "architecture": architectureOut.Architecture, "api_plan": architectureOut.APIPlan, "database_plan": architectureOut.DatabasePlan, "ai_model_usage": architectureOut.AIModelUsage, "planned_actions": architectureOut.PlannedActions},
		"file_plan":    {"file_plan": filePlanOut.FilePlan, "planned_actions": filePlanOut.PlannedActions, "file_writes": filePlanOut.FileWrites},
		"build_steps":  {"build_steps": bp.BuildSteps},
		"review":       {"risks": reviewOut.Risks, "review_checklist": reviewOut.ReviewChecklist},
		"delivery":     {"delivery_package": deliveryOut.DeliveryPackage, "status": deliveryOut.Status, "planned_actions": deliveryOut.PlannedActions, "saveable": deliveryOut.Saveable, "pipeline_actions": plannedActions},
	}
	if strings.TrimSpace(bp.RawAIOutput) != "" {
		taskOutputs["blueprint"]["raw_ai_output"] = bp.RawAIOutput
		taskOutputs["delivery"]["raw_ai_output"] = bp.RawAIOutput
	}
	tx, err := h.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	reviewNeeded := false
	for _, t := range []string{"intake", "blueprint", "architecture", "file_plan", "build_steps", "review", "delivery"} {
		if !runtimeTaskOutputValid(t, taskOutputs[t]) {
			taskOutputs[t]["status"] = "review_needed"
			taskOutputs[t]["validation_error"] = "required_fields_missing"
			reviewNeeded = true
		}
		out, _ := json.Marshal(taskOutputs[t])
		taskStatus := "completed"
		taskErr := any(nil)
		if reviewNeeded && t == "delivery" {
			taskStatus = "review_needed"
			taskErr = "delivery_requires_review_before_save"
		}
		if _, err = tx.Exec(`UPDATE runtime_tasks SET status=$4, output_json=$3, error=$5, updated_at=NOW() WHERE project_id=$1 AND task_type=$2`, projectID, t, out, taskStatus, taskErr); err != nil {
			return false, err
		}
		if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Task completed: "+t); err != nil {
			return false, err
		}
	}
	projectStatus := "completed"
	if reviewNeeded || !deliveryOut.Saveable {
		projectStatus = "review_needed"
	}
	if _, err = tx.Exec(`UPDATE runtime_projects SET status=$3, title=$2, updated_at=NOW() WHERE id=$1`, projectID, bp.ProjectTitle, projectStatus); err != nil {
		return false, err
	}
	if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "AI blueprint generated"); err != nil {
		return false, err
	}
	if _, err = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Runtime workflow completed"); err != nil {
		return false, err
	}
	creditCharged := false
	if !isPrivileged && projectStatus == "completed" {
		if err := h.applyCreditChargeTxWithReason(tx, authSub, email, "runtime_project"); err != nil {
			return false, err
		}
		creditCharged = true
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return creditCharged, nil
}

func runtimeTaskOutputValid(taskType string, out map[string]any) bool {
	required := map[string][]string{
		"intake":       {"summary", "project_type", "user_intent"},
		"blueprint":    {"project_title", "project_type", "user_intent", "mvp_scope", "next_action"},
		"architecture": {"required_infrastructure", "architecture", "api_plan", "database_plan", "ai_model_usage"},
		"file_plan":    {"file_plan"},
		"build_steps":  {"build_steps"},
		"review":       {"risks", "review_checklist"},
		"delivery":     {"delivery_package", "status"},
	}
	for _, key := range required[taskType] {
		v, ok := out[key]
		if !ok {
			return false
		}
		switch val := v.(type) {
		case string:
			if strings.TrimSpace(val) == "" {
				return false
			}
		case []string:
			if len(val) == 0 {
				return false
			}
		case []any:
			if len(val) == 0 {
				return false
			}
		}
	}
	return true
}
func (h *Handler) fetchRuntimeResponse(projectID string, bp runtimeBlueprint, creditCharged bool) (map[string]any, error) {
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
	return map[string]any{"project_id": projectID, "status": "completed", "credits_charged": creditCharged, "task_count": taskCount, "log_count": logCount, "blueprint": bp, "tasks": tasks}, nil
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
