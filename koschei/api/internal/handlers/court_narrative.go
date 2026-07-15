package handlers

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"koschei/api/internal/services"
)

type CourtNarrativeClient interface {
	ProsecutorOpinion(context.Context, CourtReadOnlyInput, string) (CourtOpinion, error)
	PanelOpinion(context.Context, CourtReadOnlyInput, []CourtOpinion) (CourtPanel, error)
	SeniorOpinion(context.Context, CourtReadOnlyInput, []CourtOpinion, *CourtPanel) (CourtPanel, error)
}

type CourtReadOnlyInput struct {
	Target        string                       `json:"target"`
	Network       string                       `json:"network"`
	SignedVerdict services.UnifiedRadarVerdict `json:"signed_verdict"`
	VerdictCard   map[string]any               `json:"verdict_card"`
}

type CourtOpinion struct {
	Model  string `json:"model"`
	Stance string `json:"stance"`
	Text   string `json:"text"`
}
type CourtPanel struct {
	Models []string `json:"models"`
	Stance string   `json:"stance"`
	Text   string   `json:"text"`
}
type courtTierOverrideKey struct{}

type CourtReport struct {
	Status       string         `json:"status"`
	TierApplied  string         `json:"tier_applied"`
	Prosecutors  []CourtOpinion `json:"prosecutors,omitempty"`
	Panel        *CourtPanel    `json:"panel,omitempty"`
	Senior       *CourtPanel    `json:"senior,omitempty"`
	Disagreement bool           `json:"disagreement"`
	Authority    string         `json:"authority"`
}

func (h *Handler) courtNarrative(ctx context.Context, in CourtReadOnlyInput, requestedExtended bool) *CourtReport {
	if !envBool("KOSCHEI_COURT_ENABLED", false) {
		return nil
	}
	tier := h.courtTier(ctx)
	status := "skipped"
	report := &CourtReport{Status: status, TierApplied: tier, Authority: "the signed deterministic verdict is final; court output is commentary/explanation"}
	if tier == "free" || tier == "basic" {
		return report
	}
	if h.CourtClient == nil {
		return report
	}
	applied, exhausted := h.applyCourtBudget(ctx, tier)
	if exhausted {
		report.Status = "budget_exhausted"
		report.TierApplied = applied
		tier = applied
	}
	if tier != "pro" && tier != "enterprise" {
		return report
	}
	if !envBool("KOSCHEI_COURT_PROSECUTORS_ENABLED", true) {
		return report
	}
	kimi, err := h.CourtClient.ProsecutorOpinion(ctx, in, "kimi-k2.6")
	if err != nil {
		report.Status = "error"
		return report
	}
	mini, err := h.CourtClient.ProsecutorOpinion(ctx, in, "minimax-m3")
	if err != nil {
		report.Status = "error"
		return report
	}
	report.Prosecutors = []CourtOpinion{normalizeCourtOpinion(kimi, "kimi-k2.6"), normalizeCourtOpinion(mini, "minimax-m3")}
	report.Disagreement = prosecutorsDisagree(report.Prosecutors)
	gradeChanging := len(in.SignedVerdict.TriggeredRules) > 0 || strings.TrimSpace(in.SignedVerdict.Grade) != "-"
	panelRan := false
	if (gradeChanging || report.Disagreement) && envBool("KOSCHEI_COURT_PANEL_ENABLED", true) {
		panel, err := h.CourtClient.PanelOpinion(ctx, in, report.Prosecutors)
		if err != nil {
			report.Status = "error"
			return report
		}
		report.Panel = &panel
		panelRan = true
	}
	if tier == "enterprise" && envBool("KOSCHEI_COURT_SENIOR_ENABLED", true) && (isDF(in.SignedVerdict.Grade) || panelRan || requestedExtended) {
		senior, err := h.CourtClient.SeniorOpinion(ctx, in, report.Prosecutors, report.Panel)
		if err != nil {
			report.Status = "error"
			return report
		}
		report.Senior = &senior
	}
	report.Status = "ready"
	return report
}

func (h *Handler) courtTier(ctx context.Context) string {
	if tier, ok := ctx.Value(courtTierOverrideKey{}).(string); ok {
		return normalizeCourtTier(tier)
	}
	if access, ok := tokenAccessRequestFromContext(ctx); ok {
		return normalizeCourtTier(access.Evaluation.Tier)
	}
	claims, ok := userFromContext(ctx)
	if !ok || h == nil {
		return "free"
	}
	ev, err := h.evaluateTokenAccess(ctx, claims.Sub)
	if err != nil || !ev.WalletVerified {
		return "free"
	}
	return normalizeCourtTier(ev.Tier)
}
func normalizeCourtTier(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "basic", "pro", "enterprise":
		return strings.ToLower(strings.TrimSpace(t))
	default:
		return "free"
	}
}
func envBool(k string, d bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(k)))
	if raw == "" {
		return d
	}
	return raw == "1" || raw == "true" || raw == "yes" || raw == "on"
}
func normalizeCourtOpinion(o CourtOpinion, model string) CourtOpinion {
	o.Model = firstNonEmptyString(o.Model, model)
	s := strings.ToLower(strings.TrimSpace(o.Stance))
	if s != "elevated" && s != "neutral" && s != "insufficient" {
		s = "insufficient"
	}
	o.Stance = s
	return o
}
func prosecutorsDisagree(p []CourtOpinion) bool {
	if len(p) < 2 {
		return false
	}
	return strings.TrimSpace(p[0].Stance) != strings.TrimSpace(p[1].Stance)
}
func isDF(g string) bool { g = strings.ToUpper(strings.TrimSpace(g)); return g == "D" || g == "F" }

func (h *Handler) applyCourtBudget(ctx context.Context, tier string) (string, bool) {
	limit := courtDailyLimit(tier)
	if limit <= 0 {
		return lowerCourtTier(tier), true
	}
	access, ok := tokenAccessRequestFromContext(ctx)
	if !ok {
		return tier, false
	}
	_, _, err := handlerScanQuotaLedger{Handler: h}.Reserve(ctx, access.AuthSubject, access.Email, "court_"+tier, limit, time.Now().UTC())
	if errors.Is(err, errScanQuotaExceeded) {
		return lowerCourtTier(tier), true
	}
	if err != nil {
		return tier, false
	}
	return tier, false
}
func courtDailyLimit(tier string) int {
	raw := os.Getenv("KOSCHEI_COURT_QUOTA_" + strings.ToUpper(tier) + "_DAILY")
	if raw == "" {
		if tier == "pro" {
			return 20
		}
		if tier == "enterprise" {
			return 100
		}
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return v
}
func lowerCourtTier(t string) string {
	if t == "enterprise" {
		return "pro"
	}
	if t == "pro" {
		return "basic"
	}
	return "free"
}

func (h *Handler) courtScheduledReport(ctx context.Context) *CourtReport {
	if !envBool("KOSCHEI_COURT_ENABLED", false) {
		return nil
	}
	tier := h.courtTier(ctx)
	report := &CourtReport{Status: "skipped", TierApplied: tier, Authority: "the signed deterministic verdict is final; court output is commentary/explanation"}
	if tier == "free" || tier == "basic" || h.CourtClient == nil {
		return report
	}
	applied, exhausted := h.applyCourtBudget(ctx, tier)
	report.TierApplied = applied
	if exhausted {
		report.Status = "budget_exhausted"
		return report
	}
	report.Status = "scheduled"
	return report
}
