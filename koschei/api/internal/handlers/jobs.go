package handlers

import (
	"net/http"

	modelrouter "koschei/api/internal/router"
)

type createJobRequest struct {
	Email  string `json:"email"`
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

func (h *Handler) GetJobs(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		writeJSON(w, 400, map[string]string{"error": "email required"})
		return
	}
	rows, err := h.DB.Query(`SELECT id, tool, prompt, route, provider, status, result, created_at, updated_at FROM generation_jobs WHERE email=$1 ORDER BY created_at DESC`, email)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	defer rows.Close()
	var jobs []map[string]any
	for rows.Next() {
		var id, tool, prompt, route, provider, status string
		var result *string
		var created, updated string
		if err := rows.Scan(&id, &tool, &prompt, &route, &provider, &status, &result, &created, &updated); err != nil {
			writeJSON(w, 500, map[string]string{"error": "db failed"})
			return
		}
		jobs = append(jobs, map[string]any{"id": id, "tool": tool, "prompt": prompt, "route": route, "provider": provider, "status": status, "result": result, "created_at": created, "updated_at": updated})
	}
	writeJSON(w, 200, map[string]any{"jobs": jobs})
}

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if !h.Limiter.allow("jobs:"+clientIP(r), 30, 10_000_000_000) {
		writeJSON(w, 429, map[string]string{"error": "rate limited"})
		return
	}
	if err := decodeJSON(r, &req); err != nil || !validEmail(req.Email) || req.Tool == "" || req.Prompt == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	route := modelrouter.ResolveModelRoute(req.Tool)
	var id, status string
	if err := h.DB.QueryRow(`INSERT INTO generation_jobs (email, tool, prompt, route, provider, status) VALUES ($1,$2,$3,$4,$5,'queued') RETURNING id, status`, req.Email, req.Tool, req.Prompt, route.Route, route.Provider).Scan(&id, &status); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db failed"})
		return
	}
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs (email, tool, route, provider, prompt, status) VALUES ($1,$2,$3,$4,$5,$6)`, req.Email, req.Tool, route.Route, route.Provider, req.Prompt, route.Status)
	writeJSON(w, 201, map[string]any{"id": id, "tool": req.Tool, "route": route.Route, "provider": route.Provider, "status": status})
}

type runtimeRouteRequest struct {
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

func (h *Handler) RuntimeRoute(w http.ResponseWriter, r *http.Request) {
	var req runtimeRouteRequest
	if err := decodeJSON(r, &req); err != nil || req.Tool == "" {
		writeJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	_ = req.Prompt
	route := modelrouter.ResolveModelRoute(req.Tool)
	_, _ = h.DB.Exec(`INSERT INTO model_route_logs (tool, route, provider, prompt, status) VALUES ($1,$2,$3,$4,$5)`, req.Tool, route.Route, route.Provider, req.Prompt, route.Status)
	writeJSON(w, 200, route)
}
