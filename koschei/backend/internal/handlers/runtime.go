package handlers

import (
	"net/http"

	"koschei/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h Handler) CreateRuntimeProject(c *gin.Context) {
	var req models.CreateRuntimeProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	projectID := uuid.NewString()
	_, err := h.DB.Exec(`INSERT INTO runtime_projects (id,email,title,prompt,status) VALUES ($1,$2,$3,$4,'queued')`, projectID, req.Email, req.Title, req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tasks := []struct {
		taskType, tool string
		priority       int
	}{{"planning", "workflow_router", 1}, {"code", "parallel_ai_workers", 5}, {"review", "credit_ledger", 7}}
	for _, t := range tasks {
		_, err = h.DB.Exec(`INSERT INTO runtime_tasks (id,project_id,email,task_type,tool,prompt,status,priority) VALUES ($1,$2,$3,$4,$5,$6,'queued',$7)`, uuid.NewString(), projectID, req.Email, t.taskType, t.tool, req.Prompt, t.priority)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	_, _ = h.DB.Exec(`INSERT INTO runtime_logs (id,project_id,level,message) VALUES ($1,$2,'info',$3)`, uuid.NewString(), projectID, "Project created and tasks queued")
	c.JSON(http.StatusOK, gin.H{"project_id": projectID, "task_types": []string{"planning", "code", "review"}})
}

func (h Handler) ListRuntimeProjects(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id,email,title,prompt,status,created_at,updated_at FROM runtime_projects WHERE email=$1 ORDER BY created_at DESC`, c.Query("email"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []models.RuntimeProject{}
	for rows.Next() {
		var p models.RuntimeProject
		_ = rows.Scan(&p.ID, &p.Email, &p.Title, &p.Prompt, &p.Status, &p.CreatedAt, &p.UpdatedAt)
		out = append(out, p)
	}
	c.JSON(200, out)
}
func (h Handler) GetRuntimeProject(c *gin.Context) {
	var p models.RuntimeProject
	err := h.DB.QueryRow(`SELECT id,email,title,prompt,status,created_at,updated_at FROM runtime_projects WHERE id=$1`, c.Param("id")).Scan(&p.ID, &p.Email, &p.Title, &p.Prompt, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, p)
}
func (h Handler) ListRuntimeTasks(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id,project_id,email,task_type,tool,prompt,status,priority,result,error,created_at,started_at,completed_at,updated_at FROM runtime_tasks WHERE email=$1 ORDER BY created_at DESC`, c.Query("email"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []models.RuntimeTask{}
	for rows.Next() {
		var t models.RuntimeTask
		_ = rows.Scan(&t.ID, &t.ProjectID, &t.Email, &t.TaskType, &t.Tool, &t.Prompt, &t.Status, &t.Priority, &t.Result, &t.Error, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt)
		out = append(out, t)
	}
	c.JSON(200, out)
}
func (h Handler) GetRuntimeTask(c *gin.Context) {
	var t models.RuntimeTask
	err := h.DB.QueryRow(`SELECT id,project_id,email,task_type,tool,prompt,status,priority,result,error,created_at,started_at,completed_at,updated_at FROM runtime_tasks WHERE id=$1`, c.Param("id")).Scan(&t.ID, &t.ProjectID, &t.Email, &t.TaskType, &t.Tool, &t.Prompt, &t.Status, &t.Priority, &t.Result, &t.Error, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.UpdatedAt)
	if err != nil {
		c.JSON(404, gin.H{"error": "not found"})
		return
	}
	c.JSON(200, t)
}
func (h Handler) GetRuntimeLogs(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id,project_id,task_id,level,message,created_at FROM runtime_logs WHERE project_id=$1 ORDER BY created_at DESC`, c.Param("projectId"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	out := []models.RuntimeLog{}
	for rows.Next() {
		var l models.RuntimeLog
		_ = rows.Scan(&l.ID, &l.ProjectID, &l.TaskID, &l.Level, &l.Message, &l.CreatedAt)
		out = append(out, l)
	}
	c.JSON(200, out)
}

func (h Handler) ownerAuth(c *gin.Context) bool {
	if h.OwnerGodModeKey == "" || c.GetHeader("x-admin-password") != h.OwnerGodModeKey {
		c.JSON(403, gin.H{"error": "forbidden"})
		return false
	}
	return true
}
func (h Handler) RetryTask(c *gin.Context) {
	if !h.ownerAuth(c) {
		return
	}
	_, err := h.DB.Exec(`UPDATE runtime_tasks SET status='queued',error=NULL,result=NULL,started_at=NULL,completed_at=NULL,updated_at=NOW() WHERE id=$1`, c.Param("id"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (h Handler) CancelTask(c *gin.Context) {
	if !h.ownerAuth(c) {
		return
	}
	_, err := h.DB.Exec(`UPDATE runtime_tasks SET status='failed',error='cancelled by owner',updated_at=NOW(),completed_at=NOW() WHERE id=$1`, c.Param("id"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func (h Handler) PatchTaskStatus(c *gin.Context) {
	if !h.ownerAuth(c) {
		return
	}
	var b struct {
		Status string `json:"status" binding:"required"`
	}
	if c.ShouldBindJSON(&b) != nil {
		c.JSON(400, gin.H{"error": "status required"})
		return
	}
	_, err := h.DB.Exec(`UPDATE runtime_tasks SET status=$2,updated_at=NOW() WHERE id=$1`, c.Param("id"), b.Status)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
