package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type storedTokenAccessEvaluator func(context.Context,string)(tokenAccessEvaluation,error)

func (h *Handler) evaluateStoredTokenAccess(ctx context.Context, authSubject string) (tokenAccessEvaluation,error) {
	if h==nil||h.DB==nil{return tokenAccessEvaluation{},sql.ErrConnDone}
	var out tokenAccessEvaluation
	var expires time.Time
	err:=h.DB.QueryRowContext(ctx,`SELECT wallet_address,verified,gate_enabled,configured,tier,balance_base_units,balance_ui,threshold_base_units,threshold_ui,evaluated_at,expires_at FROM token_access_snapshots WHERE auth_subject=$1 AND expires_at>now() ORDER BY evaluated_at DESC LIMIT 1`,strings.TrimSpace(authSubject)).Scan(&out.Wallet,&out.WalletVerified,&out.GateEnabled,&out.Configured,&out.Tier,&out.BalanceBaseUnits,&out.BalanceUI,&out.ThresholdBaseUnits,&out.ThresholdUI,&out.EvaluatedAt,&expires)
	if err!=nil{return tokenAccessEvaluation{},err};out.Source="stored_snapshot";out.Eligible=out.GateEnabled&&out.Configured&&out.WalletVerified&&tokenTierRank(out.Tier)>0;return out,nil
}

func (h *Handler) RequireStoredTokenTier(required string,next http.HandlerFunc)http.HandlerFunc{return h.requireStoredTokenTierWithEvaluator(required,h.evaluateStoredTokenAccess,next)}
func (h *Handler) requireStoredTokenTierWithEvaluator(required string,evaluator storedTokenAccessEvaluator,next http.HandlerFunc)http.HandlerFunc{
	required=strings.ToLower(strings.TrimSpace(required))
	return func(w http.ResponseWriter,r *http.Request){user,ok:=userFromContext(r.Context());if !ok{writeJSON(w,http.StatusUnauthorized,map[string]string{"error":"unauthorized"});return};evaluation,err:=evaluator(r.Context(),strings.TrimSpace(user.Sub));if err!=nil{writeJSON(w,http.StatusForbidden,map[string]any{"error":"token_access_snapshot_required","required_tier":required});return};if !evaluation.GateEnabled||!evaluation.Configured||!evaluation.WalletVerified||tokenTierRank(evaluation.Tier)<tokenTierRank(required){writeJSON(w,http.StatusForbidden,map[string]any{"error":"token_tier_required","required_tier":required,"current_tier":evaluation.Tier});return};ctx:=withTokenAccessRequestContext(r.Context(),tokenAccessRequestContext{Evaluation:evaluation,AuthSubject:user.Sub,Email:user.Email});next(w,r.WithContext(ctx))}
}

func (h *Handler) RequireAPIKeyStoredTokenTier(required string,next http.HandlerFunc)http.HandlerFunc{
	required=strings.ToLower(strings.TrimSpace(required));return func(w http.ResponseWriter,r *http.Request){principal,ok:=apiPrincipalFromContext(r.Context());if !ok{writeJSON(w,http.StatusUnauthorized,map[string]string{"error":"unauthorized"});return};evaluation,err:=h.evaluateStoredTokenAccess(r.Context(),principal.AuthSubject);if err!=nil{writeJSON(w,http.StatusForbidden,map[string]any{"error":"token_access_snapshot_required","required_tier":required});return};if !evaluation.GateEnabled||!evaluation.Configured||!evaluation.WalletVerified||tokenTierRank(evaluation.Tier)<tokenTierRank(required){writeJSON(w,http.StatusForbidden,map[string]any{"error":"token_tier_required","required_tier":required,"current_tier":evaluation.Tier});return};ctx:=withTokenAccessRequestContext(r.Context(),tokenAccessRequestContext{Evaluation:evaluation,AuthSubject:principal.AuthSubject,Email:principal.Email});next(w,r.WithContext(ctx))}
}
