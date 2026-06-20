package handlers

import (
	"database/sql"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var feedbackCategories = map[string]bool{
	"system_gap": true,
	"bug":        true,
	"suggestion": true,
	"usability":  true,
	"billing":    true,
	"security":   true,
	"other":      true,
}

var feedbackStatuses = map[string]bool{
	"new":       true,
	"reviewing": true,
	"planned":   true,
	"resolved":  true,
	"closed":    true,
}

type customerFeedbackRequest struct {
	Category     string `json:"category"`
	Subject      string `json:"subject"`
	Message      string `json:"message"`
	ContactEmail string `json:"contact_email"`
	PageURL      string `json:"page_url"`
	Website      string `json:"website"`
}

type ownerFeedbackUpdate struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	OwnerNote string `json:"owner_note"`
}

func (h *Handler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "database_unavailable"})
		return
	}
	if h.Limiter != nil && !h.Limiter.allow("customer-feedback:"+clientIP(r), 5, time.Hour) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"ok": false, "error": "rate_limited", "message": "Çok fazla geri bildirim gönderildi. Lütfen daha sonra tekrar deneyin."})
		return
	}

	var input customerFeedbackRequest
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_json", "message": "Form verisi okunamadı."})
		return
	}
	if strings.TrimSpace(input.Website) != "" {
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "message": "Geri bildiriminiz alındı."})
		return
	}

	category := strings.ToLower(strings.TrimSpace(input.Category))
	subject := strings.TrimSpace(input.Subject)
	message := strings.TrimSpace(input.Message)
	email := strings.ToLower(strings.TrimSpace(input.ContactEmail))
	pageURL := normalizeFeedbackPageURL(input.PageURL)
	if !feedbackCategories[category] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_category", "message": "Geçerli bir geri bildirim türü seçin."})
		return
	}
	if len([]rune(subject)) < 3 || len([]rune(subject)) > 160 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_subject", "message": "Başlık 3–160 karakter arasında olmalı."})
		return
	}
	if len([]rune(message)) < 10 || len([]rune(message)) > 5000 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_message", "message": "Açıklama 10–5000 karakter arasında olmalı."})
		return
	}
	if email != "" {
		address, err := mail.ParseAddress(email)
		if err != nil || !strings.EqualFold(address.Address, email) || len(email) > 254 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_email", "message": "E-posta adresi geçerli değil."})
			return
		}
	}

	userAgent := truncateFeedbackText(strings.TrimSpace(r.UserAgent()), 500)
	var id string
	err := h.DB.QueryRowContext(r.Context(), `
		INSERT INTO customer_feedback(category,subject,message,contact_email,page_url,user_agent)
		VALUES($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,''))
		RETURNING id::text
	`, category, subject, message, email, pageURL, userAgent).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "feedback_store_failed", "message": "Geri bildirim şu anda kaydedilemedi."})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":           true,
		"feedback_id":  id,
		"message":      "Geri bildiriminiz alındı. Teşekkür ederiz.",
	})
}

func (h *Handler) OwnerFeedback(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ownerFeedbackList(w, r)
	case http.MethodPost:
		h.ownerFeedbackUpdate(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *Handler) ownerFeedbackList(w http.ResponseWriter, r *http.Request) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !feedbackStatuses[status] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_status"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 300 {
			limit = parsed
		}
	}

	rows, err := h.DBRead.QueryContext(r.Context(), `
		SELECT id::text,category,subject,message,COALESCE(contact_email,''),COALESCE(page_url,''),status,COALESCE(owner_note,''),created_at,updated_at
		FROM customer_feedback
		WHERE ($1='' OR status=$1)
		ORDER BY created_at DESC
		LIMIT $2
	`, status, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "feedback_read_failed"})
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, category, subject, message, email, pageURL, itemStatus, note string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &category, &subject, &message, &email, &pageURL, &itemStatus, &note, &createdAt, &updatedAt); err != nil {
			continue
		}
		items = append(items, map[string]any{
			"id": id, "category": category, "subject": subject, "message": message,
			"contact_email": email, "page_url": pageURL, "status": itemStatus,
			"owner_note": note, "created_at": createdAt, "updated_at": updatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "items": items, "count": len(items)})
}

func (h *Handler) ownerFeedbackUpdate(w http.ResponseWriter, r *http.Request) {
	var input ownerFeedbackUpdate
	if err := decodeJSON(r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_json"})
		return
	}
	id := strings.TrimSpace(input.ID)
	status := strings.ToLower(strings.TrimSpace(input.Status))
	note := truncateFeedbackText(strings.TrimSpace(input.OwnerNote), 2000)
	if id == "" || !feedbackStatuses[status] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_feedback_update"})
		return
	}
	result, err := h.DB.ExecContext(r.Context(), `
		UPDATE customer_feedback
		SET status=$2, owner_note=$3, updated_at=now()
		WHERE id=$1::uuid
	`, id, status, note)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "feedback_update_failed"})
		return
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "feedback_not_found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id, "status": status})
}

func normalizeFeedbackPageURL(raw string) string {
	raw = truncateFeedbackText(strings.TrimSpace(raw), 1000)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	parsed.User = nil
	parsed.Fragment = ""
	return parsed.String()
}

func truncateFeedbackText(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func nullableFeedbackTime(value sql.NullTime) any {
	if value.Valid {
		return value.Time
	}
	return nil
}
