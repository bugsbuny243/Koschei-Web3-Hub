package handlers

import (
	"net/http"
	"strings"
)

func notImplemented(w http.ResponseWriter, name string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "not_implemented", "handler": name})
}

// Kayıt Olma (Register) rotasını aktif hale getirdik
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	
	// Gelen isteği çöz
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_body"})
		return
	}

	email := strings.TrimSpace(req.Email)
	if email == "" || len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Geçerli bir e-posta ve en az 8 karakterli şifre girin."})
		return
	}

	// Kullanıcıyı PostgreSQL veritabanına ekle (Her yeni kullanıcıya başlangıç için 10 kredi veriyoruz)
	_, err := dbConn.Exec(`
		INSERT INTO app_user_profiles (id, email, role, plan_id, credits, created_at)
		VALUES (gen_random_uuid(), $1, 'user', 'free', 10, NOW())
		ON CONFLICT (email) DO NOTHING
	`, email)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Kayıt oluşturulurken bir veritabanı hatası oluştu."})
		return
	}

	// Başarılı yanıtı gönder (Frontend bu yanıtı alınca login sayfasına veya ana sayfaya yönlendirecek)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Kayıt başarılı",
		"email":   email,
	})
}

// Şimdilik diğer rotalar kapalı kalmaya devam ediyor, sırayla açarız
func (h *Handler) Login(w http.ResponseWriter, r *http.Request)    { notImplemented(w, "Login") }
func (h *Handler) StartOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "StartOTPLogin")
}
func (h *Handler) VerifyOTPLogin(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "VerifyOTPLogin")
}
func (h *Handler) Me(w http.ResponseWriter, r *http.Request)         { notImplemented(w, "Me") }
func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) { UsersHandler(w, r) }
func (h *Handler) AdminUserAction(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "AdminUserAction")
}
func (h *Handler) AdminSettings(w http.ResponseWriter, r *http.Request) {
	notImplemented(w, "AdminSettings")
}
func (h *Handler) AIGenerate(w http.ResponseWriter, r *http.Request) { notImplemented(w, "AIGenerate") }
func (h *Handler) AIJobs(w http.ResponseWriter, r *http.Request)     { notImplemented(w, "AIJobs") }
