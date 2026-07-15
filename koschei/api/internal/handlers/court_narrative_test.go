package handlers

import (
	"context"
	"errors"
	"testing"

	"koschei/api/internal/services"
)

type fakeCourtClient struct {
	prosecutors, panel, senior int
	stances                    []string
	err                        bool
}

func (f *fakeCourtClient) ProsecutorOpinion(ctx context.Context, in CourtReadOnlyInput, model string) (CourtOpinion, error) {
	f.prosecutors++
	if f.err {
		return CourtOpinion{}, errors.New("boom")
	}
	s := "neutral"
	if len(f.stances) >= f.prosecutors {
		s = f.stances[f.prosecutors-1]
	}
	return CourtOpinion{Model: model, Stance: s, Text: model + " narrative"}, nil
}
func (f *fakeCourtClient) PanelOpinion(context.Context, CourtReadOnlyInput, []CourtOpinion) (CourtPanel, error) {
	f.panel++
	if f.err {
		return CourtPanel{}, errors.New("boom")
	}
	return CourtPanel{Models: []string{"qwen", "glm"}, Stance: "neutral", Text: "panel narrative"}, nil
}
func (f *fakeCourtClient) SeniorOpinion(context.Context, CourtReadOnlyInput, []CourtOpinion, *CourtPanel) (CourtPanel, error) {
	f.senior++
	if f.err {
		return CourtPanel{}, errors.New("boom")
	}
	return CourtPanel{Models: []string{"openai", "anthropic"}, Stance: "elevated", Text: "senior commentary"}, nil
}

func courtCtx(tier string) context.Context {
	return withTokenAccessRequestContext(context.Background(), tokenAccessRequestContext{Evaluation: tokenAccessEvaluation{Tier: tier, WalletVerified: true}, AuthSubject: "sub", Email: "u@example.com"})
}
func verdict(grade string, triggered bool) services.UnifiedRadarVerdict {
	v := services.UnifiedRadarVerdict{Grade: grade, Verdict: "test", RulesetVersion: services.UnifiedRadarRulesetVersion, ActorRuleset: services.ActorDefenseRulesetVersion, Signature: "sig", Signed: true}
	if triggered {
		v.TriggeredRules = []services.ActorDefenseRuleHit{{RuleID: "R", EvidenceStatus: "verified", Tier: "compounding"}}
	}
	return v
}
func courtInput(v services.UnifiedRadarVerdict) CourtReadOnlyInput {
	return CourtReadOnlyInput{Target: "mint", Network: "solana-mainnet", SignedVerdict: v, VerdictCard: map[string]any{"grade": v.Grade, "signature": v.Signature}}
}

func TestCourtFreeAndBasicPerformZeroModelCalls(t *testing.T) {
	t.Setenv("KOSCHEI_COURT_ENABLED", "true")
	for _, tier := range []string{"free", "basic"} {
		c := &fakeCourtClient{}
		h := &Handler{CourtClient: c}
		r := h.courtNarrative(courtCtx(tier), courtInput(verdict("-", false)), false)
		if r == nil || c.prosecutors+c.panel+c.senior != 0 {
			t.Fatalf("%s calls=%d report=%#v", tier, c.prosecutors+c.panel+c.senior, r)
		}
	}
}
func TestCourtProAgreeingProsecutorsNoTriggerSkipsPanel(t *testing.T) {
	t.Setenv("KOSCHEI_COURT_ENABLED", "true")
	c := &fakeCourtClient{stances: []string{"neutral", "neutral"}}
	r := (&Handler{CourtClient: c}).courtNarrative(courtCtx("pro"), courtInput(verdict("-", false)), false)
	if r.Status != "ready" || c.prosecutors != 2 || c.panel != 0 || r.Disagreement {
		t.Fatalf("report=%#v calls=%+v", r, c)
	}
}
func TestCourtProDisagreementInvokesPanel(t *testing.T) {
	t.Setenv("KOSCHEI_COURT_ENABLED", "true")
	c := &fakeCourtClient{stances: []string{"elevated", "neutral"}}
	r := (&Handler{CourtClient: c}).courtNarrative(courtCtx("pro"), courtInput(verdict("-", false)), false)
	if !r.Disagreement || c.panel != 1 {
		t.Fatalf("report=%#v calls=%+v", r, c)
	}
}
func TestCourtEnterpriseDGradeInvokesSenior(t *testing.T) {
	t.Setenv("KOSCHEI_COURT_ENABLED", "true")
	c := &fakeCourtClient{stances: []string{"neutral", "neutral"}}
	r := (&Handler{CourtClient: c}).courtNarrative(courtCtx("enterprise"), courtInput(verdict("D", true)), false)
	if c.senior != 1 || r.Senior == nil {
		t.Fatalf("report=%#v calls=%+v", r, c)
	}
}
func TestCourtBudgetExhaustionDegradesWithoutFailure(t *testing.T) {
	t.Setenv("KOSCHEI_COURT_ENABLED", "true")
	t.Setenv("KOSCHEI_COURT_QUOTA_PRO_DAILY", "0")
	c := &fakeCourtClient{}
	r := (&Handler{CourtClient: c}).courtNarrative(courtCtx("pro"), courtInput(verdict("-", false)), false)
	if r.Status != "budget_exhausted" || r.TierApplied != "basic" || c.prosecutors != 0 {
		t.Fatalf("report=%#v calls=%+v", r, c)
	}
}
func TestCourtClientErrorPreservesInputVerdict(t *testing.T) {
	t.Setenv("KOSCHEI_COURT_ENABLED", "true")
	v := verdict("B", true)
	c := &fakeCourtClient{err: true}
	r := (&Handler{CourtClient: c}).courtNarrative(courtCtx("pro"), courtInput(v), false)
	if r.Status != "error" || v.Grade != "B" || v.Signature != "sig" {
		t.Fatalf("report=%#v verdict=%#v", r, v)
	}
}
func TestCourtDoesNotChangeDeterministicVerdict(t *testing.T) {
	t.Setenv("KOSCHEI_COURT_ENABLED", "true")
	v := verdict("D", true)
	before := v.Signature + v.Grade
	c := &fakeCourtClient{stances: []string{"elevated", "neutral"}}
	_ = (&Handler{CourtClient: c}).courtNarrative(courtCtx("enterprise"), courtInput(v), true)
	after := v.Signature + v.Grade
	if before != after {
		t.Fatalf("verdict changed before=%q after=%q", before, after)
	}
}
