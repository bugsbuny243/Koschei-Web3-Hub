package handlers

import "net/http"

type createJobRequest struct { Email string `json:"email"`; Tool string `json:"tool"`; Prompt string `json:"prompt"` }

func (h *Handler) GetJobs(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" { writeJSON(w,400,map[string]string{"error":"email required"}); return }
	rows, err := h.DB.Query(`SELECT id, tool, prompt, status, result, created_at, updated_at FROM generation_jobs WHERE email=$1 ORDER BY created_at DESC`, email)
	if err != nil { writeJSON(w,500,map[string]string{"error":"db failed"}); return }
	defer rows.Close()
	var jobs []map[string]any
	for rows.Next() { var id,tool,prompt,status string; var result *string; var created,updated string; if err:=rows.Scan(&id,&tool,&prompt,&status,&result,&created,&updated); err!=nil { writeJSON(w,500,map[string]string{"error":"db failed"}); return }; jobs=append(jobs,map[string]any{"id":id,"tool":tool,"prompt":prompt,"status":status,"result":result,"created_at":created,"updated_at":updated}) }
	writeJSON(w,200,map[string]any{"jobs":jobs})
}

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err:=decodeJSON(r,&req); err!=nil || req.Email=="" || req.Tool=="" || req.Prompt=="" { writeJSON(w,400,map[string]string{"error":"invalid body"}); return }
	var id, status string
	if err:=h.DB.QueryRow(`INSERT INTO generation_jobs (email, tool, prompt, status) VALUES ($1,$2,$3,'queued') RETURNING id, status`, req.Email, req.Tool, req.Prompt).Scan(&id,&status); err!=nil { writeJSON(w,500,map[string]string{"error":"db failed"}); return }
	writeJSON(w,201,map[string]any{"id":id,"status":status})
}
