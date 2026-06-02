package handlers

import "net/http"

func (h *Handler) Plans(w http.ResponseWriter, _ *http.Request) {
	rows, err := h.DB.Query(`SELECT id, name, price_try, monthly_credits, is_active FROM plans WHERE is_active=true ORDER BY price_try ASC`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "failed"})
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, name string
		var price, credits int
		var active bool
		if err := rows.Scan(&id, &name, &price, &credits, &active); err != nil {
			writeJSON(w, 500, map[string]string{"error": "failed"})
			return
		}
		out = append(out, map[string]any{"id": id, "name": name, "price_try": price, "monthly_credits": credits, "is_active": active})
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": out})
}
