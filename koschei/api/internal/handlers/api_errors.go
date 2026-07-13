package handlers

import "net/http"

const (
	APICodeOK                 = "OK"
	APICodePackageRequired    = "PACKAGE_REQUIRED"
	APICodeInvalidInput       = "INVALID_INPUT"
	APICodeInvalidCategory    = "INVALID_CATEGORY"
	APICodeNotFound           = "NOT_FOUND"
	APICodeIntegrationError   = "INTEGRATION_ERROR"
	APICodeInternalError      = "INTERNAL_ERROR"
	APICodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	APICodeUnauthorized       = "UNAUTHORIZED"
	APICodeForbidden          = "FORBIDDEN"
)

type apiEnvelope struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func writeAPIError(w http.ResponseWriter, status int, code, message string, data ...any) {
	payload := any(nil)
	if len(data) > 0 {
		payload = data[0]
	}
	writeJSON(w, status, apiEnvelope{Success: false, Code: code, Message: message, Data: payload})
}

func writeAPISuccess(w http.ResponseWriter, message string, data any) {
	writeJSON(w, http.StatusOK, apiEnvelope{Success: true, Code: APICodeOK, Message: message, Data: data})
}
