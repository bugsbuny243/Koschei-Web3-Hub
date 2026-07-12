package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

// The legacy Shopier/Paddle payment handlers were removed from the live route
// chain. These narrow compatibility boundaries keep stale owner-only call sites
// fail-closed until those large owner blocks are deleted independently.
type removedShopierPack struct {
	AmountTRY int
}

type removedPackageActivationResult struct {
	Activated bool `json:"activated"`
}

var shopierPacks = map[string]removedShopierPack{}

func normalizePackageID(packageID string) string {
	switch strings.ToLower(strings.TrimSpace(packageID)) {
	case "starter":
		return "starter"
	case "builder", "pro", "professional":
		return "professional"
	case "studio", "enterprise":
		return "enterprise"
	default:
		return ""
	}
}

func (h *Handler) OwnerPaymentRequestsList(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (h *Handler) OwnerApprovePaymentRequest(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (h *Handler) OwnerRejectPaymentRequest(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (h *Handler) activatePackageEntitlement(context.Context, string, string, string, string) (removedPackageActivationResult, error) {
	return removedPackageActivationResult{}, errors.New("legacy package activation is disabled")
}

// Applied migrations are the schema source of truth. Runtime payment DDL was
// deleted with the legacy payment subsystem.
func ensurePaymentSchema(context.Context, *sql.DB) error {
	return nil
}
