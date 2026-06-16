package handlers

import (
	"net/http"
	"strings"

	"koschei/api/internal/services"
)

func (h *Handler) SecurityRadarGraph(w http.ResponseWriter, r *http.Request) {
	claims, ok := userFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, APICodeUnauthorized, "Unauthorized")
		return
	}
	claimEmail := normalizedClaimEmail(claims)
	if _, err := h.requirePremiumOutput(claims.Sub, claimEmail); err != nil {
		writeJSON(w, http.StatusPaymentRequired, insufficientOutputsResponse())
		return
	}
	if h == nil || h.DBRead == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "locked": false, "required_plan": "professional", "graph": services.SecurityRadarGraphResponse{OK: true, Empty: true, Message: "No node graph evidence is available for this verdict yet.", Nodes: []services.SecurityRadarGraphNode{}, Edges: []services.SecurityRadarGraphEdge{}}})
		return
	}
	status, _ := h.customerPackageStatus(r.Context(), claims.Sub, claimEmail)
	if !isNodeGraphPlan(status.PlanID) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "locked": true, "required_plan": "professional", "message": "Node Graph evidence is available on Professional and Enterprise plans."})
		return
	}
	store := services.NewSecurityRadarStore(h.DBRead)
	verdictID := strings.TrimSpace(r.URL.Query().Get("verdict_id"))
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	moduleID := strings.TrimSpace(r.URL.Query().Get("module_id"))
	var graph services.SecurityRadarGraphResponse
	var err error
	if verdictID != "" {
		graph, err = store.GraphByVerdictID(r.Context(), verdictID)
	} else {
		graph, err = store.LatestGraphForTarget(r.Context(), target, moduleID)
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "locked": false, "required_plan": "professional", "graph": services.SecurityRadarGraphResponse{OK: true, Empty: true, Message: "No node graph evidence is available for this verdict yet.", Nodes: []services.SecurityRadarGraphNode{}, Edges: []services.SecurityRadarGraphEdge{}}, "warning": "node graph store unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "locked": false, "required_plan": "professional", "graph": graph})
}

func isNodeGraphPlan(planID *string) bool {
	if planID == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(*planID)) {
	case "professional", "pro", "enterprise", "studio", "builder":
		return true
	default:
		return false
	}
}
