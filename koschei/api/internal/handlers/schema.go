package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// SchemaHandler returns the full database schema
func SchemaHandler(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		http.Error(w, "Yetkisiz erişim", http.StatusUnauthorized)
		return
	}

	rows, err := dbConn.Query(`
		SELECT table_name, column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		ORDER BY table_name, ordinal_position
	`)
	if err != nil {
		http.Error(w, "Veritabanı hatası", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Column struct {
		TableName   string `json:"table"`
		ColumnName  string `json:"column"`
		DataType    string `json:"type"`
		IsNullable  string `json:"nullable"`
	}
	var columns []Column

	for rows.Next() {
		var col Column
		rows.Scan(&col.TableName, &col.ColumnName, &col.DataType, &col.IsNullable)
		columns = append(columns, col)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(columns)
}
