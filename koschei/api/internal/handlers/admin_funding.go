package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var grantStatuses = map[string]bool{
	"watching": true, "apply_now": true, "applied": true,
	"won": true, "rejected": true, "archived": true,
}

type grantOpportunity struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Ecosystem   string    `json:"ecosystem"`
	SourceURL   string    `json:"source_url"`
	Category    string    `json:"category"`
	RewardRange string    `json:"reward_range"`
	Deadline    string    `json:"deadline"`
	Status      string    `json:"status"`
	FitScore    int       `json:"fit_score"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type grantOpportunityInput struct {
	Title       string `json:"title"`
	Ecosystem   string `json:"ecosystem"`
	SourceURL   string `json:"source_url"`
	Category    string `json:"category"`
	RewardRange string `json:"reward_range"`
	Deadline    string `json:"deadline"`
	Status      string `json:"status"`
	FitScore    int    `json:"fit_score"`
	Notes       string `json:"notes"`
}

func (h *Handler) GrantRadar(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.listGrantOpportunities(w, r)
	case http.MethodPost:
		h.createGrantOpportunity(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) listGrantOpportunities(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(), `
		SELECT id::text, title, ecosystem, source_url, category, reward_range, deadline, status, fit_score, notes, created_at, updated_at
		FROM grant_opportunities
		ORDER BY CASE status WHEN 'apply_now' THEN 0 WHEN 'watching' THEN 1 WHEN 'applied' THEN 2 WHEN 'won' THEN 3 ELSE 4 END,
		         fit_score DESC, updated_at DESC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	defer rows.Close()

	opportunities := make([]grantOpportunity, 0)
	for rows.Next() {
		var opportunity grantOpportunity
		if err := rows.Scan(&opportunity.ID, &opportunity.Title, &opportunity.Ecosystem, &opportunity.SourceURL, &opportunity.Category, &opportunity.RewardRange, &opportunity.Deadline, &opportunity.Status, &opportunity.FitScore, &opportunity.Notes, &opportunity.CreatedAt, &opportunity.UpdatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db scan failed"})
			return
		}
		opportunities = append(opportunities, opportunity)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db query failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "opportunities": opportunities})
}

func (h *Handler) createGrantOpportunity(w http.ResponseWriter, r *http.Request) {
	var input grantOpportunityInput
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Ecosystem = strings.TrimSpace(input.Ecosystem)
	input.SourceURL = strings.TrimSpace(input.SourceURL)
	input.Category = strings.TrimSpace(input.Category)
	input.RewardRange = strings.TrimSpace(input.RewardRange)
	input.Deadline = strings.TrimSpace(input.Deadline)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Notes = strings.TrimSpace(input.Notes)
	if input.Status == "" {
		input.Status = "watching"
	}
	if input.Title == "" || input.FitScore < 0 || input.FitScore > 100 || !grantStatuses[input.Status] || !validOptionalHTTPURL(input.SourceURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid opportunity"})
		return
	}

	var opportunity grantOpportunity
	err := h.DB.QueryRowContext(r.Context(), `
		INSERT INTO grant_opportunities (title, ecosystem, source_url, category, reward_range, deadline, status, fit_score, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id::text, title, ecosystem, source_url, category, reward_range, deadline, status, fit_score, notes, created_at, updated_at`,
		input.Title, input.Ecosystem, input.SourceURL, input.Category, input.RewardRange, input.Deadline, input.Status, input.FitScore, input.Notes,
	).Scan(&opportunity.ID, &opportunity.Title, &opportunity.Ecosystem, &opportunity.SourceURL, &opportunity.Category, &opportunity.RewardRange, &opportunity.Deadline, &opportunity.Status, &opportunity.FitScore, &opportunity.Notes, &opportunity.CreatedAt, &opportunity.UpdatedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db insert failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "opportunity": opportunity})
}

func validOptionalHTTPURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

type impactOutput struct {
	ID           string    `json:"id"`
	OutputType   string    `json:"output_type"`
	Title        string    `json:"title"`
	Ecosystem    string    `json:"ecosystem"`
	UsedAI       bool      `json:"used_ai"`
	UsedFallback bool      `json:"used_fallback"`
	CreatedAt    time.Time `json:"created_at"`
}

type impactChainCheck struct {
	Chain     string    `json:"chain"`
	Network   string    `json:"network"`
	Provider  string    `json:"provider"`
	Healthy   bool      `json:"healthy"`
	CheckedAt time.Time `json:"checked_at"`
}

type impactAnalytics struct {
	EventName string    `json:"event_name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) ProofOfImpact(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	counts := map[string]int64{}
	queries := map[string]string{
		"users_count":             `SELECT count(*) FROM app_user_profiles`,
		"active_users_estimate":   `SELECT count(DISTINCT lower(email)) FROM analytics_events WHERE email IS NOT NULL AND created_at >= now() - interval '30 days'`,
		"metadata_outputs_count":  `SELECT count(*) FROM web3_outputs WHERE lower(output_type) = 'metadata'`,
		"risk_outputs_count":      `SELECT count(*) FROM web3_outputs WHERE lower(output_type) IN ('risk', 'risk_scan')`,
		"watchlist_sources_count": `SELECT count(*) FROM web3_event_sources`,
		"web3_events_count":       `SELECT count(*) FROM web3_events`,
		"chain_checks_count":      `SELECT count(*) FROM chain_health_logs`,
		"payment_requests_count":  `SELECT count(*) FROM payment_requests`,
	}
	for key, query := range queries {
		count, err := safeImpactCount(r.Context(), h.DB, query)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "impact metrics unavailable"})
			return
		}
		counts[key] = count
	}

	outputs, err := latestImpactOutputs(r.Context(), h.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "impact outputs unavailable"})
		return
	}
	chainChecks, err := latestImpactChainChecks(r.Context(), h.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "impact chain checks unavailable"})
		return
	}
	analytics, err := latestImpactAnalytics(r.Context(), h.DB)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "impact analytics unavailable"})
		return
	}

	response := map[string]any{"ok": true, "latest_outputs": outputs, "latest_chain_checks": chainChecks, "latest_analytics": analytics, "live_demo_urls": []string{"https://tradepigloball.co/hub", "https://tradepigloball.co/metadata", "https://tradepigloball.co/risk", "https://tradepigloball.co/chains", "https://tradepigloball.co/watchlist"}}
	for key, value := range counts {
		response[key] = value
	}
	writeJSON(w, http.StatusOK, response)
}

func safeImpactCount(ctx context.Context, db *sql.DB, query string) (int64, error) {
	var count int64
	err := db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "does not exist") {
		return 0, nil
	}
	return count, err
}

func latestImpactOutputs(ctx context.Context, db *sql.DB) ([]impactOutput, error) {
	rows, err := db.QueryContext(ctx, `SELECT id::text, COALESCE(output_type,''), COALESCE(title,''), COALESCE(ecosystem,''), COALESCE(used_ai,false), COALESCE(used_fallback,false), created_at FROM web3_outputs ORDER BY created_at DESC LIMIT 10`)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "does not exist") {
		return []impactOutput{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]impactOutput, 0)
	for rows.Next() {
		var item impactOutput
		if err := rows.Scan(&item.ID, &item.OutputType, &item.Title, &item.Ecosystem, &item.UsedAI, &item.UsedFallback, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func latestImpactChainChecks(ctx context.Context, db *sql.DB) ([]impactChainCheck, error) {
	columns, err := chainHealthLogColumns(ctx, db)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unavailable") {
		return []impactChainCheck{}, nil
	}
	if err != nil {
		return nil, err
	}
	okColumn := firstAvailableColumn(columns, "ok", "healthy")
	checkedAtColumn := firstAvailableColumn(columns, "checked_at", "created_at")
	if okColumn == "" || checkedAtColumn == "" {
		return nil, errors.New("unsupported chain health schema")
	}
	query := fmt.Sprintf(`SELECT chain, network, provider, %s, %s FROM chain_health_logs ORDER BY %s DESC LIMIT 10`, okColumn, checkedAtColumn, checkedAtColumn)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]impactChainCheck, 0)
	for rows.Next() {
		var item impactChainCheck
		if err := rows.Scan(&item.Chain, &item.Network, &item.Provider, &item.Healthy, &item.CheckedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func latestImpactAnalytics(ctx context.Context, db *sql.DB) ([]impactAnalytics, error) {
	rows, err := db.QueryContext(ctx, `SELECT event_name, COALESCE(path,''), created_at FROM analytics_events ORDER BY created_at DESC LIMIT 10`)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "does not exist") {
		return []impactAnalytics{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]impactAnalytics, 0)
	for rows.Next() {
		var item impactAnalytics
		if err := rows.Scan(&item.EventName, &item.Path, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
