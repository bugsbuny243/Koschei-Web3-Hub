package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

type generateMetadataRequest struct {
	AssetName   string `json:"asset_name"`
	Description string `json:"description"`
	AssetType   string `json:"asset_type"`
	Traits      string `json:"traits"`
}

type metadataAttribute struct {
	TraitType string `json:"trait_type"`
	Value     string `json:"value"`
}

type generatedMetadata struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Type        string              `json:"type"`
	Attributes  []metadataAttribute `json:"attributes"`
	Properties  metadataProperties  `json:"properties"`
}

type metadataProperties struct {
	Category    string `json:"category"`
	GeneratedBy string `json:"generated_by"`
}

func (h *Handler) GenerateMetadata(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req generateMetadataRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	assetName := strings.TrimSpace(req.AssetName)
	description := strings.TrimSpace(req.Description)
	assetType := strings.TrimSpace(req.AssetType)
	if email == "" || assetName == "" || assetType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}
	if description == "" {
		description = "A unique digital asset"
	}

	metadata := buildGeneratedMetadata(assetName, description, assetType, req.Traits)
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "metadata_generation_failed"})
		return
	}
	prettyJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "metadata_generation_failed"})
		return
	}

	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(r.Context(), `
		UPDATE entitlements
		SET outputs_remaining = GREATEST(outputs_remaining - 1, 0),
			updated_at = now()
		WHERE id = (
			SELECT id
			FROM entitlements
			WHERE lower(email) = lower($1)
				AND status = 'active'
				AND outputs_remaining > 0
			ORDER BY outputs_remaining DESC, created_at DESC
			LIMIT 1
		)`, email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if rowsAffected == 0 {
		writeJSON(w, http.StatusPaymentRequired, map[string]string{
			"error":   "no_outputs_remaining",
			"message": "No outputs remaining. Please upgrade your plan.",
		})
		return
	}

	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO web3_outputs (email, output_type, title, ecosystem, content_json, content_text, used_ai, used_fallback)
		VALUES ($1, 'metadata', $2, 'web3', $3::jsonb, $4, false, true)`, email, assetName, string(metadataJSON), string(prettyJSON)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"metadata":      metadata,
		"used_ai":       false,
		"used_fallback": true,
	})
}

func buildGeneratedMetadata(assetName string, description string, assetType string, traits string) generatedMetadata {
	return generatedMetadata{
		Name:        assetName,
		Description: description,
		Type:        assetType,
		Attributes:  metadataAttributes(traits),
		Properties: metadataProperties{
			Category:    "game_asset",
			GeneratedBy: "Koschei Web3 Hub",
		},
	}
}

func metadataAttributes(traits string) []metadataAttribute {
	attributes := make([]metadataAttribute, 0)
	for _, trait := range strings.Split(traits, ",") {
		if value := strings.TrimSpace(trait); value != "" {
			attributes = append(attributes, metadataAttribute{TraitType: "Trait", Value: value})
		}
	}
	return attributes
}
