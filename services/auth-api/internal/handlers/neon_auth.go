package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (handler *Handler) requestNeonAuth(ctx context.Context, authPath string, input credentials) (string, int, error) {
	body, _ := json.Marshal(input)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, handler.authBaseURL+"/"+authPath, strings.NewReader(string(body)))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := handler.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	var payload any
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		return "", res.StatusCode, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", res.StatusCode, fmt.Errorf("Neon Auth returned %d", res.StatusCode)
	}
	if token := bearerToken(res.Header.Get("set-auth-jwt")); token != "" {
		return token, res.StatusCode, nil
	}
	if token := findToken(payload); token != "" {
		return token, res.StatusCode, nil
	}
	return "", res.StatusCode, errors.New("Neon Auth JWT was not found")
}

func findToken(payload any) string {
	object, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"token", "access_token", "accessToken", "jwt", "auth_token", "authToken"} {
		if token, ok := object[key].(string); ok && strings.TrimSpace(token) != "" {
			return bearerToken(token)
		}
	}
	for _, key := range []string{"data", "session", "user"} {
		if token := findToken(object[key]); token != "" {
			return token
		}
	}
	return ""
}

func bearerToken(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "Bearer "))
}
