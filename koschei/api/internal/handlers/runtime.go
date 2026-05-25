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

type RuntimeAgentContract struct {
	ProjectTitle    string             `json:"project_title"`
	ProjectType     string             `json:"project_type"`
	UserIntent      string             `json:"user_intent"`
	Intake          IntakeOutput       `json:"intake"`
	Blueprint       BlueprintOutput    `json:"blueprint"`
	Architecture    ArchitectureOutput `json:"architecture"`
	FilePlan        FilePlanOutput     `json:"file_plan"`
	ToolPlan        ToolPlanOutput     `json:"tool_plan"`
	Review          ReviewOutput       `json:"review"`
	Delivery        DeliveryOutput     `json:"delivery"`
	RawAIOutput     string             `json:"raw_ai_output,omitempty"`
	ContractVersion string             `json:"contract_version"`
}
type IntakeOutput struct {
	Summary             string   `json:"summary"`
	DetectedProjectType string   `json:"detected_project_type"`
	TargetUser          string   `json:"target_user"`
	Complexity          string   `json:"complexity"`
	Assumptions         []string `json:"assumptions"`
}
type BlueprintOutput struct {
	ProjectTitle    string   `json:"project_title"`
	ProjectType     string   `json:"project_type"`
	MVPScope        []string `json:"mvp_scope"`
	SuccessCriteria []string `json:"success_criteria"`
	NextAction      string   `json:"next_action"`
}
type ArchitectureOutput struct {
	RequiredInfrastructure []string `json:"required_infrastructure"`
	BackendPlan            []string `json:"backend_plan"`
	FrontendPlan           []string `json:"frontend_plan"`
	DatabasePlan           []string `json:"database_plan"`
	APIPlan                []string `json:"api_plan"`
	ExternalServices       []string `json:"external_services"`
	AIModelUsage           []string `json:"ai_model_usage"`
}
type FilePlanOutput struct {
	Files []FilePlanItem `json:"files"`
}
type FilePlanItem struct {
	Path, Purpose, Language, Priority, Action string
}
type ToolPlanOutput struct {
	ProposedToolCalls []ProposedToolCall `json:"proposed_tool_calls"`
}
type ProposedToolCall struct {
	ToolName              string         `json:"tool_name"`
	Intent                string         `json:"intent"`
	Arguments             map[string]any `json:"arguments"`
	RiskLevel             string         `json:"risk_level"`
	RequiresHumanApproval bool           `json:"requires_human_approval"`
}
type ReviewOutput struct {
	Risks               []string `json:"risks"`
	GuardrailFlags      []string `json:"guardrail_flags"`
	SecurityNotes       []string `json:"security_notes"`
	MissingRequirements []string `json:"missing_requirements"`
	ReviewStatus        string   `json:"review_status"`
}
type DeliveryOutput struct {
	DeliveryPackage []string `json:"delivery_package"`
	UserSummary     string   `json:"user_summary"`
	NextSteps       []string `json:"next_steps"`
	ReadyForPhase6  bool     `json:"ready_for_phase6"`
}
type ValidationResult struct {
	Valid, ReviewNeeded, Blocked bool
	Errors, Warnings             []string
}

const runtimeSystemPrompt = `You are Koschei Agentic Runtime Factory.
Default language is Turkish.
Return ONLY valid JSON.
Do not use markdown.
Do not wrap in ` + "```json" + `.
Do not add explanation outside JSON.
Required JSON shape:
{
  "contract_version": "5.3",
  "project_title": "string",
  "project_type": "string",
  "user_intent": "string",
  "intake": {},
  "blueprint": {},
  "architecture": {},
  "file_plan": {},
  "tool_plan": {},
  "review": {},
  "delivery": {}
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
	taskOrder := []string{"intake", "blueprint", "architecture", "file_plan", "build_steps", "review", "delivery"}
	tx, txErr := h.DB.Begin()
	if txErr != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	defer tx.Rollback()
	if _, txErr = tx.Exec(`INSERT INTO runtime_projects (id,email,title,prompt,status) VALUES ($1,$2,$3,$4,'running')`, projectID, claims.Email, title, req.Prompt); txErr != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	for _, t := range taskOrder {
		inp, _ := json.Marshal(map[string]any{"title": title, "prompt": req.Prompt, "stage": t})
		if _, txErr = tx.Exec(`INSERT INTO runtime_tasks (id,project_id,email,task_type,status,input_json,output_json) VALUES ($1,$2,$3,$4,'queued',$5,'{}'::jsonb)`, newID(), projectID, claims.Email, t, inp); txErr != nil {
			writeJSON(w, 500, map[string]string{"error": "db_failed"})
			return
		}
	}
	if _, txErr = tx.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Runtime project queued"); txErr != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	if txErr = tx.Commit(); txErr != nil {
		writeJSON(w, 500, map[string]string{"error": "db_failed"})
		return
	}
	go h.processRuntimeProject(projectID, claims.Sub, claims.Email, req.Prompt, isPrivileged)
	writeJSON(w, 201, map[string]any{"project_id": projectID, "status": "running", "message": "Runtime project queued", "credits_charged": false})
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
func (h *Handler) processRuntimeProject(projectID, authSub, email, prompt string, isPrivileged bool) {
	res, err := h.DB.Exec(`UPDATE runtime_projects SET status='processing', updated_at=NOW() WHERE id=$1 AND status='running'`, projectID)
	if err != nil {
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return
	}
	_, _ = h.DB.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, newID(), projectID, "Runtime AI pipeline started")
	_, _ = h.DB.Exec(`UPDATE runtime_tasks SET status='running', updated_at=NOW() WHERE project_id=$1 AND task_type='blueprint'`, projectID)
	aiOut, aiErr := h.callTogetherRuntimeBlueprint(projectID, prompt)
	if aiErr != nil {
		failureMsg := "Runtime AI pipeline failed: " + shortError(aiErr.Error())
		if isTimeoutError(aiErr) {
			failureMsg = "Runtime AI pipeline failed: provider timeout"
		}
		_ = h.markRuntimeFailed(projectID, failureMsg, "blueprint", aiErr.Error())
		return
	}
	contract := buildRuntimeAgentContract(aiOut, prompt)
	if _, err = h.completeRuntimePipeline(projectID, authSub, email, contract, isPrivileged); err != nil {
		_ = h.markRuntimeFailed(projectID, "Runtime AI pipeline failed: "+shortError(err.Error()), "delivery", err.Error())
	}
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
	contract := buildRuntimeAgentContract(aiOut, prompt)
	creditCharged, err := h.completeRuntimePipeline(projectID, authSub, email, contract, isPrivileged)
	if err != nil {
		_ = h.markRuntimeFailed(projectID, "Runtime AI pipeline failed: "+shortError(err.Error()), "delivery", err.Error())
		return nil, err
	}
	return h.fetchRuntimeResponse(projectID, contract, creditCharged)
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

func (h *Handler) markRuntimeFailed(projectID, msg, taskType, taskErr string) error {
	_, _ = h.DB.Exec(`UPDATE runtime_projects SET status='failed', updated_at=NOW() WHERE id=$1`, projectID)
	_, _ = h.DB.Exec(`UPDATE runtime_tasks SET status='failed', error=$3, updated_at=NOW() WHERE project_id=$1 AND task_type=$2`, projectID, taskType, shortError(taskErr))
	_, err := h.DB.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'error',$3)`, newID(), projectID, msg)
	return err
}
func buildRuntimeAgentContract(raw, prompt string) RuntimeAgentContract {
	var c RuntimeAgentContract
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return c
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start < 0 || end <= start || json.Unmarshal([]byte(raw[start:end+1]), &c) != nil {
			c.RawAIOutput = raw
		}
	}
	if c.ContractVersion == "" {
		c.ContractVersion = "5.3"
	}
	if c.ProjectTitle == "" {
		c.ProjectTitle = normalizeRuntimeTitle("", prompt)
	}
	if c.ProjectType == "" {
		c.ProjectType = "software_mvp"
	}
	if c.UserIntent == "" {
		c.UserIntent = prompt
	}
	if len(c.Blueprint.MVPScope) == 0 {
		c.Blueprint.MVPScope = []string{"MVP kapsamını netleştir"}
	}
	if len(c.Architecture.RequiredInfrastructure) == 0 {
		c.Architecture.RequiredInfrastructure = []string{"backend", "database"}
	}
	if len(c.FilePlan.Files) == 0 {
		c.FilePlan.Files = []FilePlanItem{{Path: "docs/plan.md", Purpose: "plan", Language: "markdown", Priority: "high", Action: "review_only"}}
	}
	if len(c.Delivery.NextSteps) == 0 {
		c.Delivery.NextSteps = []string{"İnsan onayı al"}
	}
	if c.Review.ReviewStatus == "" {
		c.Review.ReviewStatus = "review_needed"
	}
	if c.RawAIOutput != "" {
		c.Review.GuardrailFlags = append(c.Review.GuardrailFlags, "invalid_json")
	}
	return c
}

func validateRuntimeAgentContract(contract RuntimeAgentContract) ValidationResult {
	v := ValidationResult{Valid: true}
	if contract.ProjectTitle == "" {
		v.Valid = false
		v.Errors = append(v.Errors, "project_title required")
	}
	if contract.ProjectType == "" {
		v.Valid = false
		v.Errors = append(v.Errors, "project_type required")
	}
	if contract.UserIntent == "" {
		v.Valid = false
		v.Errors = append(v.Errors, "user_intent required")
	}
	if len(contract.Blueprint.MVPScope) == 0 {
		v.Valid = false
		v.Errors = append(v.Errors, "blueprint.mvp_scope required")
	}
	if len(contract.Architecture.RequiredInfrastructure) == 0 {
		v.Valid = false
		v.Errors = append(v.Errors, "architecture.required_infrastructure required")
	}
	if len(contract.FilePlan.Files) == 0 {
		v.Valid = false
		v.Errors = append(v.Errors, "file_plan.files required")
	}
	if len(contract.Delivery.NextSteps) == 0 {
		v.Valid = false
		v.Errors = append(v.Errors, "delivery.next_steps required")
	}
	for _, f := range contract.FilePlan.Files {
		if strings.Contains(f.Path, "../") {
			v.Blocked = true
			v.Errors = append(v.Errors, "dangerous file path")
		}
	}
	for _, t := range contract.ToolPlan.ProposedToolCalls {
		if strings.EqualFold(t.RiskLevel, "high") && !t.RequiresHumanApproval {
			v.ReviewNeeded = true
			v.Warnings = append(v.Warnings, "high risk tool call without approval")
		}
	}
	if contract.Review.ReviewStatus == "blocked" {
		v.Blocked = true
	}
	if contract.Review.ReviewStatus == "review_needed" {
		v.ReviewNeeded = true
	}
	return v
}

func (h *Handler) completeRuntimePipeline(projectID, authSub, email string, contract RuntimeAgentContract, isPrivileged bool) (bool, error) {
	validation := validateRuntimeAgentContract(contract)
	status := "completed"
	if validation.Blocked {
		status = "failed"
	} else if validation.ReviewNeeded || !validation.Valid {
		status = "review_needed"
	}
	taskOutputs := map[string]map[string]any{
		"intake":       {"agent": "intake", "contract_version": "5.3", "output": contract.Intake, "validation": validation},
		"blueprint":    {"agent": "blueprint", "contract_version": "5.3", "output": contract.Blueprint, "project_title": contract.ProjectTitle, "project_type": contract.ProjectType, "user_intent": contract.UserIntent, "validation": validation},
		"architecture": {"agent": "architecture", "contract_version": "5.3", "output": contract.Architecture, "validation": validation},
		"file_plan":    {"agent": "file_plan", "contract_version": "5.3", "output": contract.FilePlan, "validation": validation},
		"build_steps":  {"agent": "tool_plan", "contract_version": "5.3", "output": contract.ToolPlan, "note": "Proposed tool calls only. No tool execution in Phase 5.3.", "validation": validation},
		"review":       {"agent": "review", "contract_version": "5.3", "output": contract.Review, "validation": validation},
		"delivery":     {"agent": "delivery", "contract_version": "5.3", "output": contract.Delivery, "raw_ai_output": contract.RawAIOutput, "validation": validation},
	}
	tx, err := h.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	for _, t := range []string{"intake", "blueprint", "architecture", "file_plan", "build_steps", "review", "delivery"} {
		out, _ := json.Marshal(taskOutputs[t])
		ts := "completed"
		if status != "completed" && t == "delivery" {
			ts = status
		}
		if _, err = tx.Exec(`UPDATE runtime_tasks SET status=$4, output_json=$3, updated_at=NOW() WHERE project_id=$1 AND task_type=$2`, projectID, t, out, ts); err != nil {
			return false, err
		}
	}
	if _, err = tx.Exec(`UPDATE runtime_projects SET status=$3, title=$2, updated_at=NOW() WHERE id=$1`, projectID, contract.ProjectTitle, status); err != nil {
		return false, err
	}
	creditCharged := false
	if !isPrivileged && status == "completed" {
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

func (h *Handler) fetchRuntimeResponse(projectID string, contract RuntimeAgentContract, creditCharged bool) (map[string]any, error) {
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
	return map[string]any{"project_id": projectID, "status": "completed", "credits_charged": creditCharged, "task_count": taskCount, "log_count": logCount, "blueprint": contract, "tasks": tasks}, nil
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
