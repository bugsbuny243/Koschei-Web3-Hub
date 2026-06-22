package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed templates/owner_game_app.js
var ownerGameAppTemplate string

type ownerGameProjectView struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Prompt         string   `json:"prompt"`
	GameType       string   `json:"game_type"`
	TargetPlatform string   `json:"target_platform"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"created_at,omitempty"`
	Spec           gameSpec `json:"spec"`
	BundleURL      string   `json:"bundle_url"`
}

func (h *Handler) OwnerGameStudio(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, googlePlayReadiness())
	case http.MethodPost:
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))), "multipart/form-data") {
			h.ownerGooglePlayUploadAAB(w, r)
			return
		}
		h.ownerGameStudioCreate(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
	}
}

func (h *Handler) ownerGameStudioCreate(w http.ResponseWriter, r *http.Request) {
	var req createGameRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.TargetPlatform = strings.ToLower(strings.TrimSpace(req.TargetPlatform))
	if len(req.Title) < 3 || len(req.Title) > 120 || len(req.Prompt) < 10 || len(req.Prompt) > 5000 || !validTargetPlatform(req.TargetPlatform) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if !aiProviderConfigured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ai_provider_not_configured"})
		return
	}

	model := firstNonEmpty(strings.TrimSpace(os.Getenv("TOGETHER_MODEL_GAME_DESIGN")), strings.TrimSpace(os.Getenv("TOGETHER_MODEL")))
	userPrompt := "Project title: " + req.Title + "\nTarget platform: " + req.TargetPlatform + "\nOwner request:\n" + req.Prompt
	rawSpec, err := h.callTogetherWithSystemTimeoutAndMaxTokens(model, gameSpecSystemPrompt, userPrompt, 45*time.Second, 900)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "generation_failed", "detail": shortError(err.Error())})
		return
	}
	spec, err := parseGameSpec(rawSpec, targetPlatformsFor(req.TargetPlatform))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid_spec_response"})
		return
	}

	projectID := newID()
	specJSON, _ := json.Marshal(spec)
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO game_projects
			(id,user_id,title,prompt,game_type,target_platform,ownership_status,status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'spec_generated')`,
		projectID, ownerGameStudioUserID(), req.Title, req.Prompt, spec.GameType, req.TargetPlatform, ownerGameOwnershipStatus); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "game_project_insert_failed"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO game_specs
			(id,game_project_id,spec_json,generated_by_model,status)
		VALUES ($1,$2,$3::jsonb,$4,'spec_generated')`,
		newID(), projectID, string(specJSON), firstNonEmpty(model, "router-default")); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "game_spec_insert_failed"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_commit_failed"})
		return
	}

	project := ownerGameProjectView{
		ID: projectID, Title: req.Title, Prompt: req.Prompt, GameType: spec.GameType,
		TargetPlatform: req.TargetPlatform, Status: "spec_generated",
		CreatedAt: time.Now().UTC().Format(time.RFC3339), Spec: spec,
		BundleURL: "/api/owner/game-studio/bundle?id=" + projectID,
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"ok": true,
		"message": "Mobil oyun ve çalıştırılabilir Expo kaynak paketi hazırlandı.",
		"spec_summary": buildSpecSummary(spec),
		"play": googlePlayReadiness(),
		"project": project,
	})
}

func (h *Handler) ownerGooglePlayUploadAAB(w http.ResponseWriter, r *http.Request) {
	const maxAABSize = int64(200 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maxAABSize+1)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_multipart_upload"})
		return
	}
	file, header, err := r.FormFile("aab")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "aab_file_required"})
		return
	}
	defer file.Close()
	if !strings.EqualFold(filepath.Ext(header.Filename), ".aab") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only_aab_files_are_allowed"})
		return
	}
	aab, err := io.ReadAll(io.LimitReader(file, maxAABSize+1))
	if err != nil || int64(len(aab)) > maxAABSize {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "aab_file_too_large"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	publisher, source, serviceAccount, err := newGooglePlayPublisher(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "google_play_not_ready", "detail": shortError(err.Error())})
		return
	}
	result, err := publisher.PublishAAB(ctx, aab, r.FormValue("release_name"), r.FormValue("release_notes"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "google_play_publish_failed", "detail": shortError(err.Error())})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"message": "AAB Google Play Internal Testing düzenlemesine gönderildi.",
		"credentials_source": source,
		"service_account": serviceAccount,
		"result": result,
	})
}

func (h *Handler) OwnerGameStudioBundle(w http.ResponseWriter, r *http.Request) {
	if !h.ownerAuth(w, r) {
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("id"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id_required"})
		return
	}
	var title, targetPlatform, rawSpec string
	err := h.DB.QueryRowContext(r.Context(), `
		SELECT gp.title, gp.target_platform, gs.spec_json::text
		FROM game_projects gp
		JOIN game_specs gs ON gs.game_project_id=gp.id
		WHERE gp.id=$1 AND gp.user_id=$2
		ORDER BY COALESCE(to_jsonb(gs)->>'created_at','') DESC, gs.id::text DESC
		LIMIT 1`, projectID, ownerGameStudioUserID()).Scan(&title, &targetPlatform, &rawSpec)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "game_project_not_found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "game_project_read_failed"})
		return
	}
	var spec gameSpec
	if err := json.Unmarshal([]byte(rawSpec), &spec); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "game_spec_invalid"})
		return
	}
	bundle, err := buildOwnerExpoGameBundle(title, projectID, targetPlatform, spec)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "bundle_generation_failed"})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+ownerGameSlug(title)+`-expo.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(bundle)
}

func ownerGameStudioUserID() string {
	owner := normalizeWallet(firstEnv("OWNER_WALLET", "KOSCHEI_OWNER_WALLET"))
	if owner == "" {
		owner = "control-center"
	}
	return "owner:" + owner
}

func buildOwnerExpoGameBundle(title, projectID, targetPlatform string, spec gameSpec) ([]byte, error) {
	files, err := ownerExpoGameFiles(title, projectID, targetPlatform, spec)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	root := ownerGameSlug(title) + "/"
	for _, path := range paths {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: root + path, Method: zip.Deflate})
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err := entry.Write([]byte(files[path])); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func ownerExpoGameFiles(title, projectID, targetPlatform string, spec gameSpec) (map[string]string, error) {
	slug := ownerGameSlug(title)
	fallbackPackage := "com.koschei.owner." + strings.ReplaceAll(slug, "-", "")
	packageName := firstNonEmpty(strings.TrimSpace(os.Getenv("ANDROID_PLAY_PACKAGE_NAME")), fallbackPackage)
	versionCode := int(time.Now().Unix())
	packageJSON, err := json.MarshalIndent(map[string]any{
		"name": slug, "version": "1.0.0", "private": true,
		"main": "node_modules/expo/AppEntry.js",
		"scripts": map[string]string{
			"start": "expo start", "android": "expo start --android",
			"web": "expo start --web", "build:android": "eas build --platform android --profile production",
		},
		"dependencies": map[string]string{
			"expo": "latest", "react": "latest", "react-native": "latest",
		},
		"devDependencies": map[string]string{"@babel/core": "latest"},
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	appJSON, err := json.MarshalIndent(map[string]any{"expo": map[string]any{
		"name": title, "slug": slug, "version": "1.0.0", "orientation": "portrait",
		"userInterfaceStyle": "dark",
		"android": map[string]any{"package": packageName, "versionCode": versionCode},
		"web": map[string]string{"bundler": "metro"},
	}}, "", "  ")
	if err != nil {
		return nil, err
	}
	easJSON, _ := json.MarshalIndent(map[string]any{"build": map[string]any{
		"preview": map[string]any{"distribution": "internal", "android": map[string]string{"buildType": "apk"}},
		"production": map[string]any{"autoIncrement": true},
	}}, "", "  ")
	specJSON, _ := json.MarshalIndent(spec, "", "  ")

	appJS := ownerGameAppTemplate
	values := map[string]any{
		"__TITLE__": title,
		"__THEME__": firstNonEmpty(spec.Theme, "Neon frontier"),
		"__PLAYER__": firstNonEmpty(spec.Player, "pilot"),
		"__ENEMIES__": ownerGameList(spec.Enemies, []string{"meteor", "drone"}),
		"__COLLECTIBLES__": ownerGameList(spec.Collectibles, []string{"energy", "crystal"}),
		"__WIN__": firstNonEmpty(spec.WinCondition, "Reach 100 points"),
		"__PROJECT_ID__": projectID,
	}
	for token, value := range values {
		raw, _ := json.Marshal(value)
		appJS = strings.ReplaceAll(appJS, token, string(raw))
	}
	readme := fmt.Sprintf("# %s\n\nKoschei Owner Mobile Game Studio tarafından otomatik üretilen Expo projesi.\n\n- Project ID: %s\n- Game type: %s\n- Theme: %s\n- Target: %s\n- Android package: %s\n- Version code: %d\n- Win condition: %s\n\n## Çalıştır\n\n1. npm install\n2. npx expo start\n3. Expo Go ile QR kodu okutun veya npm run android çalıştırın.\n\n## AAB üret\n\n1. npm install -g eas-cli\n2. eas login\n3. npm run build:android\n4. Oluşan .aab dosyasını Owner Panel > Mobil Oyun Studio içinden Internal Testing taslağına gönderin.\n\nPaket production secret içermez.\n", title, projectID, spec.GameType, spec.Theme, targetPlatform, packageName, versionCode, spec.WinCondition)
	return map[string]string{
		"App.js": appJS,
		"README.md": readme,
		"app.json": string(appJSON),
		"babel.config.js": "module.exports = function(api) { api.cache(true); return { presets: ['babel-preset-expo'] }; };\n",
		"eas.json": string(easJSON),
		"game-spec.json": string(specJSON),
		"package.json": string(packageJSON),
	}, nil
}

func ownerGameList(values, fallback []string) []string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			clean = append(clean, value)
		}
		if len(clean) == 8 {
			break
		}
	}
	if len(clean) == 0 {
		return fallback
	}
	return clean
}

func ownerGameSlug(value string) string {
	value = strings.NewReplacer(
		"Ç", "C", "Ğ", "G", "İ", "I", "Ö", "O", "Ş", "S", "Ü", "U",
		"ç", "c", "ğ", "g", "ı", "i", "ö", "o", "ş", "s", "ü", "u",
	).Replace(strings.TrimSpace(value))
	value = strings.ToLower(value)
	var out strings.Builder
	separator := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			separator = false
		} else if out.Len() > 0 && !separator {
			out.WriteByte('-')
			separator = true
		}
	}
	slug := strings.Trim(out.String(), "-")
	if slug == "" {
		slug = "koschei-mobile-game"
	}
	if slug[0] >= '0' && slug[0] <= '9' {
		slug = "game-" + slug
	}
	if len(slug) > 48 {
		slug = strings.Trim(slug[:48], "-")
	}
	return slug
}
