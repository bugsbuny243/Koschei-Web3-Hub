package handlers

import (
	"net/http"
	"strings"
)

func notImplemented(w http.ResponseWriter, name string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not_implemented", "handler": name})
}

// Kayıt Olma (Register) 
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	email := strings.TrimSpace(req.Email)
	if email == "" || len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Geçerli bir e-posta ve en az 8 karakterli şifre girin."})
		return
	}

	_, err := dbConn.Exec(`
		INSERT INTO app_user_profiles (id, email, role, plan_id, credits, created_at)
		VALUES (gen_random_uuid(), $1, 'user', 'free', 10, NOW())
		ON CONFLICT (email) DO NOTHING
	`, email)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Kayıt oluşturulurken bir veritabanı hatası oluştu."})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Kayıt başarılı",
		"email":   email,
	})
}

// Frontend kayıt sonrası otomatik giriş yapmaya çalışırsa hata vermesin diye açtık
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Giriş başarılı",
		"token":   "koschei-gecici-token", 
	})
}

// Frontend oturum açtıktan sonra kullanıcı bilgilerini (Me) sorarsa hata vermesin diye açtık
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      "gecici-id",
		"email":   "test@koschei.com",
		"role":    "user",
		"credits": 10,
	})
}

// Kalan kullanılmayan rotalar
func (h *Handler) StartOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "StartOTPLogin")
}
func (h *Handler) VerifyOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "VerifyOTPLogin")
}
func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) { UsersHandler(w, r) }
func (h *Handler) AdminUserAction(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "AdminUserAction")
}
func (h *Handler) AdminSettings(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "AdminSettings")
}
func (h *Handler) AIGenerate(w http.ResponseWriter, r *http.Request) { notImplemented(w, "AIGenerate") }
func (h *Handler) AIJobs(w http.ResponseWriter, r *http.Request)     { notImplemented(w, "AIJobs") }
