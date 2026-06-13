package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"koschei/api/internal/web3"
)

const (
	RealDataUnavailableMessage = "Real data unavailable. Analysis could not be completed."
	ModuleTXDecoder            = "tx_decoder"
	ModuleTokenScanner         = "token_scanner"
	ModuleWalletScore          = "wallet_score"
	ModuleRiskScanner          = "risk_scanner"
	ModuleSybilGraph           = "sybil_graph"
	ModuleProjectRadar         = "project_radar"
	moduleTimeout              = 10 * time.Second
	defaultSolanaNet           = "solana-mainnet"
)

type UnifiedEngine struct {
	RPC        *web3.SolanaRPC
	HTTPClient *http.Client
}

type UnifiedAnalyzeRequest struct {
	RequestID  string `json:"request_id,omitempty"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Network    string `json:"network,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type UnifiedAnalyzeResult struct {
	RequestID       string                  `json:"request_id"`
	TargetType      string                  `json:"target_type"`
	TargetID        string                  `json:"target_id"`
	Network         string                  `json:"network"`
	OverallScore    int                     `json:"overall_score"`
	RiskLevel       string                  `json:"risk_level"`
	Recommendations []string                `json:"recommendations"`
	ModuleResults   map[string]ModuleResult `json:"module_results"`
	PartialSuccess  bool                    `json:"partial_success"`
	CompletedAt     time.Time               `json:"completed_at"`
}

type ModuleResult struct {
	Module      string         `json:"module"`
	Status      string         `json:"status"`
	Score       int            `json:"score,omitempty"`
	RiskLevel   string         `json:"risk_level,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Findings    []string       `json:"findings,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Error       string         `json:"error,omitempty"`
	DurationMS  int64          `json:"duration_ms"`
	CompletedAt time.Time      `json:"completed_at"`
}

type txRPCResult struct {
	Slot      uint64 `json:"slot"`
	BlockTime *int64 `json:"blockTime"`
	Meta      *struct {
		Err interface{} `json:"err"`
		Fee uint64      `json:"fee"`
	} `json:"meta"`
	Transaction struct {
		Message struct {
			AccountKeys  []accountKeyRPC `json:"accountKeys"`
			Instructions []struct {
				ProgramID string `json:"programId"`
			} `json:"instructions"`
		} `json:"message"`
	} `json:"transaction"`
}

type accountKeyRPC struct {
	Pubkey   string `json:"pubkey"`
	Signer   bool   `json:"signer"`
	Writable bool   `json:"writable"`
}

type accountInfoRPC struct {
	Value *struct {
		Lamports   uint64 `json:"lamports"`
		Executable bool   `json:"executable"`
		Owner      string `json:"owner"`
		Data       any    `json:"data"`
	} `json:"value"`
}

type signaturesRPC []struct {
	Signature string      `json:"signature"`
	Slot      uint64      `json:"slot"`
	Err       interface{} `json:"err"`
	BlockTime *int64      `json:"blockTime"`
}

type tokenSupplyRPC struct {
	Value struct {
		Amount         string  `json:"amount"`
		Decimals       int     `json:"decimals"`
		UiAmount       float64 `json:"uiAmount"`
		UiAmountString string  `json:"uiAmountString"`
	} `json:"value"`
}

type tokenLargestRPC struct {
	Value []struct {
		Address string `json:"address"`
		Amount  string `json:"amount"`
	} `json:"value"`
}

func NewUnifiedEngine(rpc *web3.SolanaRPC) *UnifiedEngine {
	if rpc == nil {
		rpc = web3.NewSolanaRPC(nil)
	}
	return &UnifiedEngine{RPC: rpc, HTTPClient: &http.Client{Timeout: moduleTimeout}}
}

func (e *UnifiedEngine) Analyze(ctx context.Context, req UnifiedAnalyzeRequest) (UnifiedAnalyzeResult, error) {
	req.TargetType = normalizeTargetType(req.TargetType)
	req.TargetID = strings.TrimSpace(req.TargetID)
	if req.Network == "" {
		req.Network = defaultSolanaNet
	}
	if req.TargetType == "" || req.TargetID == "" {
		return UnifiedAnalyzeResult{}, errors.New("target_type and target_id are required")
	}

	modules := e.applicableModules(req.TargetType)
	out := make(map[string]ModuleResult, len(modules))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, fn := range modules {
		wg.Add(1)
		go func(name string, fn func(context.Context, UnifiedAnalyzeRequest) ModuleResult) {
			defer wg.Done()
			started := time.Now()
			moduleCtx, cancel := context.WithTimeout(ctx, moduleTimeout)
			defer cancel()
			result := fn(moduleCtx, req)
			if result.Module == "" {
				result.Module = name
			}
			if result.DurationMS == 0 {
				result.DurationMS = time.Since(started).Milliseconds()
			}
			if result.CompletedAt.IsZero() {
				result.CompletedAt = time.Now().UTC()
			}
			mu.Lock()
			out[name] = result
			mu.Unlock()
		}(name, fn)
	}
	wg.Wait()

	score, level := aggregateScore(out)
	return UnifiedAnalyzeResult{
		RequestID:       firstNonEmpty(req.RequestID, deterministicID(req.TargetType+":"+req.TargetID+":"+time.Now().UTC().Format(time.RFC3339Nano))),
		TargetType:      req.TargetType,
		TargetID:        req.TargetID,
		Network:         req.Network,
		OverallScore:    score,
		RiskLevel:       level,
		Recommendations: recommendations(level, out),
		ModuleResults:   out,
		PartialSuccess:  partial(out),
		CompletedAt:     time.Now().UTC(),
	}, nil
}

func (e *UnifiedEngine) applicableModules(targetType string) map[string]func(context.Context, UnifiedAnalyzeRequest) ModuleResult {
	switch targetType {
	case "tx", "transaction":
		return map[string]func(context.Context, UnifiedAnalyzeRequest) ModuleResult{ModuleTXDecoder: e.AnalyzeTX, ModuleRiskScanner: e.ScanRisk}
	case "token", "mint":
		return map[string]func(context.Context, UnifiedAnalyzeRequest) ModuleResult{ModuleTokenScanner: e.ScanToken, ModuleRiskScanner: e.ScanRisk}
	case "wallet":
		return map[string]func(context.Context, UnifiedAnalyzeRequest) ModuleResult{ModuleWalletScore: e.ScoreWallet, ModuleSybilGraph: e.CheckSybil, ModuleRiskScanner: e.ScanRisk}
	case "address":
		return map[string]func(context.Context, UnifiedAnalyzeRequest) ModuleResult{ModuleWalletScore: e.ScoreWallet, ModuleSybilGraph: e.CheckSybil, ModuleTokenScanner: e.ScanToken, ModuleRiskScanner: e.ScanRisk}
	case "project", "url":
		return map[string]func(context.Context, UnifiedAnalyzeRequest) ModuleResult{ModuleProjectRadar: e.ScanProject, ModuleRiskScanner: e.ScanRisk}
	default:
		return map[string]func(context.Context, UnifiedAnalyzeRequest) ModuleResult{ModuleRiskScanner: e.ScanRisk}
	}
}

func (e *UnifiedEngine) AnalyzeTX(ctx context.Context, req UnifiedAnalyzeRequest) ModuleResult {
	started := time.Now()
	if req.TargetType != "tx" && req.TargetType != "transaction" {
		return skipped(ModuleTXDecoder, "TX decoder requires target_type=tx", started)
	}
	var tx txRPCResult
	err := e.RPC.Call(ctx, req.Network, "getTransaction", []any{req.TargetID, map[string]any{"encoding": "jsonParsed", "maxSupportedTransactionVersion": 0}}, &tx, web3.TTLFor("getTransaction", nil))
	if err != nil {
		return failed(ModuleTXDecoder, err, started)
	}
	programs := uniquePrograms(tx.Transaction.Message.Instructions)
	score := 90
	findings := []string{"Transaction was found on Solana RPC."}
	if tx.Meta != nil && tx.Meta.Err != nil {
		score -= 35
		findings = append(findings, "Transaction execution reported an on-chain error.")
	}
	if len(programs) >= 6 {
		score -= 15
		findings = append(findings, "Transaction touches many programs; inspect routed or composite execution carefully.")
	}
	return moduleOK(ModuleTXDecoder, score, findings, map[string]any{"slot": tx.Slot, "block_time": tx.BlockTime, "fee_lamports": fee(tx), "programs": programs, "signers": signers(tx.Transaction.Message.AccountKeys)}, started)
}

func (e *UnifiedEngine) ScanToken(ctx context.Context, req UnifiedAnalyzeRequest) ModuleResult {
	started := time.Now()
	if req.TargetType != "token" && req.TargetType != "mint" && req.TargetType != "address" {
		return skipped(ModuleTokenScanner, "Token scanner requires target_type=token or address", started)
	}
	var supply tokenSupplyRPC
	if err := e.RPC.Call(ctx, req.Network, "getTokenSupply", []any{req.TargetID}, &supply, time.Minute); err != nil {
		return failed(ModuleTokenScanner, err, started)
	}
	var account web3.TokenAccountInfoRPC
	if err := e.RPC.Call(ctx, req.Network, "getAccountInfo", []any{req.TargetID, map[string]string{"encoding": "jsonParsed"}}, &account, 30*time.Second); err != nil {
		return failed(ModuleTokenScanner, err, started)
	}
	var largest tokenLargestRPC
	_ = e.RPC.Call(ctx, req.Network, "getTokenLargestAccounts", []any{req.TargetID}, &largest, 5*time.Minute)
	topOne, topTen := holderConcentration(supply.Value.Amount, largest)
	score := 100
	findings := []string{"Token supply was loaded from Solana RPC."}
	if account.Value != nil && account.Value.Data.Parsed.Info.MintAuthority != nil {
		score -= 25
		findings = append(findings, "Mint authority is still active.")
	}
	if account.Value != nil && account.Value.Data.Parsed.Info.FreezeAuthority != nil {
		score -= 20
		findings = append(findings, "Freeze authority is still active.")
	}
	if topOne >= 50 {
		score -= 35
		findings = append(findings, "Largest token holder controls at least half of observed supply.")
	} else if topOne >= 20 {
		score -= 20
		findings = append(findings, "Largest token holder concentration is significant.")
	}
	if topTen >= 80 {
		score -= 20
		findings = append(findings, "Top ten token accounts control most observed supply.")
	}
	return moduleOK(ModuleTokenScanner, clamp(score), findings, map[string]any{"supply": supply.Value, "largest_holder_percent": topOne, "top_ten_percent": topTen, "mint_authority_active": account.Value != nil && account.Value.Data.Parsed.Info.MintAuthority != nil, "freeze_authority_active": account.Value != nil && account.Value.Data.Parsed.Info.FreezeAuthority != nil}, started)
}

func (e *UnifiedEngine) ScoreWallet(ctx context.Context, req UnifiedAnalyzeRequest) ModuleResult {
	started := time.Now()
	if req.TargetType != "wallet" && req.TargetType != "address" {
		return skipped(ModuleWalletScore, "Wallet score requires target_type=wallet", started)
	}
	account, sigs, err := e.loadAccountAndSignatures(ctx, req.Network, req.TargetID, 100)
	if err != nil {
		return failed(ModuleWalletScore, err, started)
	}
	score, findings := walletScore(account, sigs)
	return moduleOK(ModuleWalletScore, score, findings, map[string]any{"balance_sol": float64(account.Value.Lamports) / 1e9, "recent_tx_count": len(sigs), "recent_failed_tx_count": failedCount(sigs), "account_owner": account.Value.Owner, "executable": account.Value.Executable}, started)
}

func (e *UnifiedEngine) ScanRisk(ctx context.Context, req UnifiedAnalyzeRequest) ModuleResult {
	started := time.Now()
	findings := []string{}
	score := 75
	if req.TargetType == "tx" || req.TargetType == "transaction" {
		tx := e.AnalyzeTX(ctx, req)
		if tx.Status == "ok" {
			score = tx.Score
			findings = append(findings, tx.Findings...)
		} else {
			return tx
		}
	} else if req.TargetType == "token" || req.TargetType == "mint" {
		tok := e.ScanToken(ctx, req)
		if tok.Status == "ok" {
			score = tok.Score
			findings = append(findings, tok.Findings...)
		} else {
			return tok
		}
	} else if req.TargetType == "wallet" || req.TargetType == "address" {
		wallet := e.ScoreWallet(ctx, req)
		if wallet.Status == "ok" {
			score = wallet.Score
			findings = append(findings, wallet.Findings...)
		} else {
			return wallet
		}
	} else if req.TargetType == "project" || req.TargetType == "url" {
		project := e.ScanProject(ctx, req)
		if project.Status == "ok" {
			score = project.Score
			findings = append(findings, project.Findings...)
		} else {
			return project
		}
	}
	if req.Notes != "" && containsRiskWords(req.Notes) {
		score -= 15
		findings = append(findings, "User notes contain elevated-risk keywords.")
	}
	if len(findings) == 0 {
		findings = append(findings, "No critical risk flags were found by the applicable real-time checks.")
	}
	return moduleOK(ModuleRiskScanner, clamp(score), findings, map[string]any{"disclaimer": "Informational risk screening only; not financial, legal, or security advice."}, started)
}

func (e *UnifiedEngine) CheckSybil(ctx context.Context, req UnifiedAnalyzeRequest) ModuleResult {
	started := time.Now()
	if req.TargetType != "wallet" && req.TargetType != "address" {
		return skipped(ModuleSybilGraph, "Sybil graph requires target_type=wallet", started)
	}
	_, sigs, err := e.loadAccountAndSignatures(ctx, req.Network, req.TargetID, 100)
	if err != nil {
		return failed(ModuleSybilGraph, err, started)
	}
	score := 85
	findings := []string{"Loaded recent wallet activity from Solana RPC."}
	if len(sigs) <= 2 {
		score -= 25
		findings = append(findings, "Very low observed activity; reputation graph is sparse.")
	}
	if burstyActivity(sigs) {
		score -= 25
		findings = append(findings, "Recent activity is highly clustered in time, a common airdrop-farming signal.")
	}
	if failedCount(sigs) > len(sigs)/3 && len(sigs) > 0 {
		score -= 15
		findings = append(findings, "High failed-transaction ratio in recent activity.")
	}
	return moduleOK(ModuleSybilGraph, clamp(score), findings, map[string]any{"recent_transactions_checked": len(sigs), "failed_transactions": failedCount(sigs), "burst_activity_detected": burstyActivity(sigs)}, started)
}

func (e *UnifiedEngine) ScanProject(ctx context.Context, req UnifiedAnalyzeRequest) ModuleResult {
	started := time.Now()
	if req.TargetType != "project" && req.TargetType != "url" {
		return skipped(ModuleProjectRadar, "Project radar requires target_type=project or url", started)
	}
	raw := req.TargetID
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return failed(ModuleProjectRadar, fmt.Errorf("invalid project url"), started)
	}
	httpClient := e.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return failed(ModuleProjectRadar, err, started)
	}
	hreq.Header.Set("User-Agent", "Koschei-Unified-Intelligence/1.0")
	resp, err := httpClient.Do(hreq)
	if err != nil {
		return failed(ModuleProjectRadar, err, started)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	text := strings.ToLower(string(body))
	score := 80
	findings := []string{fmt.Sprintf("Project website returned HTTP %d.", resp.StatusCode)}
	if u.Scheme != "https" {
		score -= 15
		findings = append(findings, "Website is not using HTTPS.")
	}
	if resp.StatusCode >= 400 {
		score -= 30
		findings = append(findings, "Website returned an error status.")
	}
	if !strings.Contains(text, "github") && !strings.Contains(text, "docs") && !strings.Contains(text, "whitepaper") {
		score -= 10
		findings = append(findings, "Website does not expose obvious docs, GitHub, or whitepaper links in the fetched page.")
	}
	if strings.Contains(text, "guaranteed") || strings.Contains(text, "100x") || strings.Contains(text, "risk-free") {
		score -= 20
		findings = append(findings, "Website copy contains high-risk promotional language.")
	}
	return moduleOK(ModuleProjectRadar, clamp(score), findings, map[string]any{"url": u.String(), "status_code": resp.StatusCode, "content_type": resp.Header.Get("Content-Type"), "fetched_bytes": len(body)}, started)
}

func (e *UnifiedEngine) loadAccountAndSignatures(ctx context.Context, network, address string, limit int) (accountInfoRPC, signaturesRPC, error) {
	var account accountInfoRPC
	if err := e.RPC.Call(ctx, network, "getAccountInfo", []any{address, map[string]string{"encoding": "jsonParsed"}}, &account, 30*time.Second); err != nil {
		return accountInfoRPC{}, nil, err
	}
	if account.Value == nil {
		return accountInfoRPC{}, nil, fmt.Errorf("account not found")
	}
	var sigs signaturesRPC
	if err := e.RPC.Call(ctx, network, "getSignaturesForAddress", []any{address, map[string]any{"limit": limit}}, &sigs, time.Minute); err != nil {
		return account, nil, err
	}
	return account, sigs, nil
}

func skipped(module, reason string, started time.Time) ModuleResult {
	return ModuleResult{Module: module, Status: "skipped", Summary: reason, DurationMS: time.Since(started).Milliseconds(), CompletedAt: time.Now().UTC()}
}

func failed(module string, err error, started time.Time) ModuleResult {
	status := "error"
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "deadline") || strings.Contains(strings.ToLower(err.Error()), "timeout") {
		status = "timeout"
	}
	return ModuleResult{Module: module, Status: status, Summary: RealDataUnavailableMessage, Error: err.Error(), RiskLevel: "UNKNOWN", DurationMS: time.Since(started).Milliseconds(), CompletedAt: time.Now().UTC()}
}

func moduleOK(module string, score int, findings []string, data map[string]any, started time.Time) ModuleResult {
	return ModuleResult{Module: module, Status: "ok", Score: clamp(score), RiskLevel: riskLevelFromScore(score), Summary: summaryFor(module, score), Findings: findings, Data: data, DurationMS: time.Since(started).Milliseconds(), CompletedAt: time.Now().UTC()}
}

func aggregateScore(results map[string]ModuleResult) (int, string) {
	total, weight := 0, 0
	weights := map[string]int{ModuleRiskScanner: 2, ModuleTokenScanner: 2, ModuleTXDecoder: 1, ModuleWalletScore: 1, ModuleSybilGraph: 1, ModuleProjectRadar: 1}
	for name, result := range results {
		if result.Status != "ok" {
			continue
		}
		w := weights[name]
		if w == 0 {
			w = 1
		}
		total += result.Score * w
		weight += w
	}
	if weight == 0 {
		return 0, "UNKNOWN"
	}
	score := total / weight
	return score, riskLevelFromScore(score)
}

func recommendations(level string, results map[string]ModuleResult) []string {
	recs := []string{}
	if level == "UNKNOWN" {
		return []string{RealDataUnavailableMessage}
	}
	if level == "CRITICAL" || level == "HIGH" {
		recs = append(recs, "Pause execution until flagged findings are manually reviewed.")
	} else if level == "MEDIUM" {
		recs = append(recs, "Proceed only with reduced exposure and additional verification.")
	} else {
		recs = append(recs, "No critical automated flags were produced by completed real-data checks.")
	}
	for _, key := range sortedKeys(results) {
		r := results[key]
		if r.Status == "timeout" || r.Status == "error" {
			recs = append(recs, fmt.Sprintf("Re-run %s or verify it manually because it returned %s.", key, r.Status))
			continue
		}
		if r.Status == "ok" && r.Score < 60 && len(r.Findings) > 0 {
			recs = append(recs, fmt.Sprintf("Review %s finding: %s", key, r.Findings[0]))
		}
	}
	return dedupe(recs)
}

func partial(results map[string]ModuleResult) bool {
	for _, result := range results {
		if result.Status == "error" || result.Status == "timeout" || result.Status == "skipped" {
			return true
		}
	}
	return false
}

func riskLevelFromScore(score int) string {
	score = clamp(score)
	switch {
	case score >= 80:
		return "LOW"
	case score >= 60:
		return "MEDIUM"
	case score >= 35:
		return "HIGH"
	default:
		return "CRITICAL"
	}
}

func normalizeTargetType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "transaction", "signature", "hash":
		return "tx"
	case "mint", "token_mint":
		return "token"
	case "wallet_address":
		return "wallet"
	case "address":
		return "address"
	case "url", "website", "project_url":
		return "project"
	default:
		return t
	}
}

func summaryFor(module string, score int) string {
	return fmt.Sprintf("%s completed using retrieved production data with %s risk posture.", module, riskLevelFromScore(score))
}
func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
func fee(tx txRPCResult) uint64 {
	if tx.Meta == nil {
		return 0
	}
	return tx.Meta.Fee
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func deterministicID(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:16])
}

func uniquePrograms(instructions []struct {
	ProgramID string `json:"programId"`
}) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, ix := range instructions {
		if ix.ProgramID == "" || seen[ix.ProgramID] {
			continue
		}
		seen[ix.ProgramID] = true
		out = append(out, ix.ProgramID)
	}
	return out
}

func signers(keys []accountKeyRPC) []string {
	out := []string{}
	for _, key := range keys {
		if key.Signer {
			out = append(out, key.Pubkey)
		}
	}
	return out
}

func holderConcentration(totalRaw string, largest tokenLargestRPC) (float64, float64) {
	var total float64
	_, _ = fmt.Sscanf(totalRaw, "%f", &total)
	if total <= 0 {
		return 0, 0
	}
	topOne, topTen := 0.0, 0.0
	for i, holder := range largest.Value {
		var amount float64
		_, _ = fmt.Sscanf(holder.Amount, "%f", &amount)
		if i == 0 {
			topOne = amount / total * 100
		}
		if i < 10 {
			topTen += amount / total * 100
		}
	}
	return round2(topOne), round2(topTen)
}

func walletScore(account accountInfoRPC, sigs signaturesRPC) (int, []string) {
	score := 35
	findings := []string{"Wallet account exists on Solana RPC."}
	if account.Value.Lamports >= 1e9 {
		score += 15
	} else if account.Value.Lamports >= 1e8 {
		score += 8
	}
	if len(sigs) >= 100 {
		score += 25
	} else if len(sigs) >= 20 {
		score += 18
	} else if len(sigs) > 0 {
		score += 8
	} else {
		findings = append(findings, "No recent transactions were found in the fetched range.")
	}
	failed := failedCount(sigs)
	if len(sigs) > 0 {
		rate := float64(failed) / float64(len(sigs))
		if rate < 0.05 {
			score += 20
		} else if rate < 0.20 {
			score += 10
		} else {
			score -= 15
			findings = append(findings, "Recent transaction failure rate is elevated.")
		}
	}
	if account.Value.Executable {
		score -= 10
		findings = append(findings, "Address is an executable program account, not a standard wallet.")
	}
	return clamp(score), findings
}

func failedCount(sigs signaturesRPC) int {
	n := 0
	for _, sig := range sigs {
		if sig.Err != nil {
			n++
		}
	}
	return n
}

func burstyActivity(sigs signaturesRPC) bool {
	if len(sigs) < 5 {
		return false
	}
	var times []int64
	for _, sig := range sigs {
		if sig.BlockTime != nil {
			times = append(times, *sig.BlockTime)
		}
	}
	if len(times) < 5 {
		return false
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	return times[len(times)-1]-times[0] <= 600
}

func containsRiskWords(s string) bool {
	s = strings.ToLower(s)
	for _, word := range []string{"rug", "drain", "exploit", "hack", "freeze", "mint", "honeypot", "blacklist"} {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]ModuleResult) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range in {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}
func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }

func (r UnifiedAnalyzeResult) ModuleResultsJSON() ([]byte, error) {
	return json.Marshal(r.ModuleResults)
}
