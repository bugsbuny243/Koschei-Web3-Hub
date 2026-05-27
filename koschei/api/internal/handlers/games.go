package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
)

type createGameRequest struct {
	Title          string `json:"title"`
	Prompt         string `json:"prompt"`
	TargetPlatform string `json:"target_platform"`
}

type gameSpec struct {
	GameType          string   `json:"game_type"`
	Theme             string   `json:"theme"`
	Player            string   `json:"player"`
	Enemies           []string `json:"enemies"`
	Collectibles      []string `json:"collectibles"`
	Levels            []string `json:"levels"`
	Controls          []string `json:"controls"`
	WinCondition      string   `json:"win_condition"`
	MonetizationNotes string   `json:"monetization_notes"`
	TargetPlatforms   []string `json:"target_platforms"`
	RequiredAssets    []string `json:"required_assets"`
	TechnicalPlan     []string `json:"technical_plan"`
}

const gameSpecSystemPrompt = `You are Koschei Engine backend game designer.
Return ONLY strict JSON and no markdown.
Build a production-ready game specification from customer intent.
The JSON must include exactly these keys:
- game_type (string)
- theme (string)
- player (string)
- enemies (array of strings)
- collectibles (array of strings)
- levels (array of strings)
- controls (array of strings)
- win_condition (string)
- monetization_notes (string)
- target_platforms (array of strings)
- required_assets (array of strings)
- technical_plan (array of strings)
Keep arrays concise (2-8 items).`

func (h *Handler) CreateGameProject(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createGameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	title := strings.TrimSpace(req.Title)
	prompt := strings.TrimSpace(req.Prompt)
	platform := strings.ToLower(strings.TrimSpace(req.TargetPlatform))
	if len(title) < 3 || len(title) > 120 || len(prompt) < 10 || len(prompt) > 5000 || !validTargetPlatform(platform) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	model := strings.TrimSpace(os.Getenv("TOGETHER_MODEL_GAME_DESIGN"))
	if strings.TrimSpace(os.Getenv("TOGETHER_API_KEY")) == "" || model == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ai_provider_not_configured"})
		return
	}

	targetPlatforms := targetPlatformsFor(platform)
	userPrompt := "Project title: " + title + "\nTarget platform: " + platform + "\nCustomer prompt:\n" + prompt
	resp, err := h.callTogetherWithSystemTimeoutAndMaxTokens(model, gameSpecSystemPrompt, userPrompt, 45_000_000_000, 900)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "generation_failed", "detail": shortError(err.Error())})
		return
	}

	spec, err := parseGameSpec(resp, targetPlatforms)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid_spec_response"})
		return
	}

	specJSON, _ := json.Marshal(spec)
	projectID := newID()
	status := "spec_generated"
	summary := buildSpecSummary(spec)
	userID := strings.TrimSpace(claims.Sub)
	if userID == "" {
		userID = strings.TrimSpace(claims.Email)
	}

	tx, err := h.DB.Begin()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO game_projects (id, user_id, title, prompt, game_type, target_platform, ownership_status, status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, projectID, userID, title, prompt, spec.GameType, platform, "customer_owned", status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if _, err := tx.Exec(`INSERT INTO game_specs (id, game_project_id, spec_json, generated_by_model, status) VALUES ($1,$2,$3::jsonb,$4,$5)`, newID(), projectID, string(specJSON), model, status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"game_project_id": projectID,
		"title":           title,
		"target_platform": platform,
		"spec_summary":    summary,
		"status":          status,
	})
}

func validTargetPlatform(v string) bool {
	switch v {
	case "web_game", "android_game", "web_and_android":
		return true
	default:
		return false
	}
}

func targetPlatformsFor(v string) []string {
	switch v {
	case "web_game":
		return []string{"web"}
	case "android_game":
		return []string{"android"}
	case "web_and_android":
		return []string{"web", "android"}
	default:
		return []string{"web"}
	}
}

func parseGameSpec(raw string, targetPlatforms []string) (gameSpec, error) {
	var spec gameSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return spec, err
	}
	if strings.TrimSpace(spec.GameType) == "" || strings.TrimSpace(spec.Theme) == "" || strings.TrimSpace(spec.Player) == "" || strings.TrimSpace(spec.WinCondition) == "" {
		return spec, errors.New("missing required fields")
	}
	if len(spec.TargetPlatforms) == 0 {
		spec.TargetPlatforms = targetPlatforms
	}
	return spec, nil
}

func buildSpecSummary(spec gameSpec) string {
	levels := ""
	if len(spec.Levels) > 0 {
		levels = spec.Levels[0]
	}
	return "Type: " + spec.GameType + ", Theme: " + spec.Theme + ", Core Win: " + spec.WinCondition + ", First Level: " + levels
}
