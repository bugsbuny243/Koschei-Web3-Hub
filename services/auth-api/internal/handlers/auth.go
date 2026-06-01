package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (handler *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	handler.authenticate(w, r, "sign-up/email")
}
func (handler *Handler) Login(w http.ResponseWriter, r *http.Request) {
	handler.authenticate(w, r, "sign-in/email")
}

func (handler *Handler) authenticate(w http.ResponseWriter, r *http.Request, authPath string) {
	var input credentials
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON body.")
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !strings.Contains(input.Email, "@") || len(input.Password) < 8 || len(input.Password) > 128 {
		WriteError(w, http.StatusBadRequest, "A valid email and password are required.")
		return
	}
	token, status, err := handler.requestNeonAuth(r.Context(), authPath, input)
	if err != nil {
		log.Printf("neon auth %s failed: %v", authPath, err)
		if status >= 400 && status < 500 {
			WriteError(w, http.StatusUnauthorized, "Invalid email or password.")
			return
		}
		WriteError(w, http.StatusBadGateway, "Auth provider request failed.")
		return
	}
	member, err := handler.verifyJWT(r.Context(), token)
	if err != nil {
		log.Printf("JWT verification failed: %v", err)
		WriteError(w, http.StatusBadGateway, "Auth token verification failed.")
		return
	}
	if err := handler.database.UpsertProfile(r.Context(), member); err != nil {
		log.Printf("profile upsert failed: %v", err)
		WriteError(w, http.StatusServiceUnavailable, "Could not create user profile.")
		return
	}
	handler.setMemberCookie(w, member)
	WriteJSON(w, http.StatusOK, map[string]any{"email": member.Email})
}
