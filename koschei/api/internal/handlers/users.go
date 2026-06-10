package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// UsersHandler returns all user profiles
func UsersHandler(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Yetkisiz", http.StatusUnauthorized)
		return
	}

	rows, err := dbConn.Query(`
		SELECT id, email, role, plan_id, credits, created_at
		FROM app_user_profiles
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, "DB hatası", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type User struct {
		ID        string    `json:"id"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		Plan      string    `json:"plan"`
		Credits   int       `json:"credits"`
		CreatedAt time.Time `json:"created_at"`
	}
	var users []User

	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Email, &u.Role, &u.Plan, &u.Credits, &u.CreatedAt)
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
