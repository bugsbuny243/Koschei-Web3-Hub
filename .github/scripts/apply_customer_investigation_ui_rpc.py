from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    file = Path(path)
    text = file.read_text()
    if old not in text:
        raise RuntimeError(f"marker not found in {path}: {old[:120]!r}")
    file.write_text(text.replace(old, new, 1))


# 1) Give each RPC endpoint its own bounded attempt so a stalled primary cannot
# consume the entire parent context before fallback is tried.
rpc_path = "koschei/api/internal/web3/solana_rpc.go"
replace_once(rpc_path, '\t"os"\n\t"strings"', '\t"os"\n\t"strconv"\n\t"strings"')
replace_once(
    rpc_path,
    '''\tvar lastErr error
\tfor _, endpoint := range uniqueRPCURLs(s.URL(network), SolanaRPCFallbackURL(network)) {
\t\tif err := callSolanaRPC(ctx, client, endpoint, method, body, target); err != nil {
\t\t\tlastErr = err
\t\t\tif ctx.Err() != nil {
\t\t\t\treturn ctx.Err()
\t\t\t}
\t\t\tcontinue
\t\t}
\t\t_ = s.Cache.SetJSON(ctx, key, target, ttl)
\t\treturn nil
\t}
''',
    '''\tvar lastErr error
\tendpointTimeout := solanaRPCEndpointTimeout()
\tfor _, endpoint := range uniqueRPCURLs(s.URL(network), SolanaRPCFallbackURL(network)) {
\t\tattemptCtx := ctx
\t\tcancel := func() {}
\t\tif endpointTimeout > 0 {
\t\t\tattemptCtx, cancel = context.WithTimeout(ctx, endpointTimeout)
\t\t}
\t\terr := callSolanaRPC(attemptCtx, client, endpoint, method, body, target)
\t\tcancel()
\t\tif err != nil {
\t\t\tlastErr = err
\t\t\tif ctx.Err() != nil {
\t\t\t\treturn ctx.Err()
\t\t\t}
\t\t\tcontinue
\t\t}
\t\t_ = s.Cache.SetJSON(ctx, key, target, ttl)
\t\treturn nil
\t}
'''
)
replace_once(
    rpc_path,
    '''func isSolanaMainnet(network string) bool {''',
    '''func solanaRPCEndpointTimeout() time.Duration {
\tif raw := strings.TrimSpace(os.Getenv("SOLANA_RPC_ENDPOINT_TIMEOUT_MS")); raw != "" {
\t\tif value, err := strconv.Atoi(raw); err == nil && value >= 50 && value <= 30000 {
\t\t\treturn time.Duration(value) * time.Millisecond
\t\t}
\t}
\treturn 6 * time.Second
}

func isSolanaMainnet(network string) bool {'''
)

rpc_test = "koschei/api/internal/web3/solana_rpc_test.go"
replace_once(
    rpc_test,
    '''func TestUniqueRPCURLsRemovesDuplicates(t *testing.T) {''',
    '''func TestSolanaRPCFallsBackAfterEndpointTimeout(t *testing.T) {
\tvar primaryCalls atomic.Int32
\tprimary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
\t\tprimaryCalls.Add(1)
\t\t<-r.Context().Done()
\t}))
\tdefer primary.Close()

\tvar fallbackCalls atomic.Int32
\tfallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
\t\tfallbackCalls.Add(1)
\t\tw.Header().Set("Content-Type", "application/json")
\t\t_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"value":"fallback-ok"}}`)
\t}))
\tdefer fallback.Close()

\tt.Setenv("SOLANA_RPC_URL", primary.URL)
\tt.Setenv("SOLANA_RPC_FALLBACK_URL", fallback.URL)
\tt.Setenv("SOLANA_RPC_ENDPOINT_TIMEOUT_MS", "75")
\trpc := &SolanaRPC{Client: fallback.Client(), Cache: cache.NewNoop(), KeyPrefix: "test"}
\tvar out struct {
\t\tValue string `json:"value"`
\t}
\tstarted := time.Now()
\tif err := rpc.Call(context.Background(), "solana-mainnet", "getVersion", []any{}, &out, time.Second); err != nil {
\t\tt.Fatalf("expected timeout fallback success, got %v", err)
\t}
\tif elapsed := time.Since(started); elapsed > time.Second {
\t\tt.Fatalf("fallback took too long: %s", elapsed)
\t}
\tif out.Value != "fallback-ok" || primaryCalls.Load() != 1 || fallbackCalls.Load() != 1 {
\t\tt.Fatalf("unexpected fallback result=%q primary=%d fallback=%d", out.Value, primaryCalls.Load(), fallbackCalls.Load())
\t}
}

func TestUniqueRPCURLsRemovesDuplicates(t *testing.T) {'''
)

# 2) Route live evidence through the shared cached primary+fallback RPC client.
live_path = "koschei/api/internal/handlers/unified_live_evidence.go"
replace_once(live_path, '\t"context"\n\t"math"', '\t"context"\n\t"fmt"\n\t"math"')
replace_once(live_path, '\t"strings"\n\t"time"', '\t"strings"\n\t"sync"\n\t"time"')
replace_once(
    live_path,
    '''\trpcURL := strings.TrimSpace(creatorIntelRPCURL())
\tif rpcURL == "" {
\t\tout.Limitations = append(out.Limitations, "Solana RPC is unavailable; no live transaction rows were collected.")
\t\treturn out
\t}
\tout.RPCConfigured = true
''',
    '''\trpcURL := strings.TrimSpace(creatorIntelRPCURL())
\tif rpcURL == "" && (h == nil || h.SolanaRPC == nil) {
\t\tout.Limitations = append(out.Limitations, "Solana RPC is unavailable; no live transaction rows were collected.")
\t\treturn out
\t}
\tout.RPCConfigured = true
\tnetwork := strings.TrimSpace(core.Request.Network)
\tif network == "" {
\t\tnetwork = "solana-mainnet"
\t}
'''
)
replace_once(
    live_path,
    '''\tif creator == "" {
\t\tlaunchCtx, cancel := context.WithTimeout(ctx, time.Duration(budgets.LaunchTimeoutSeconds)*time.Second)
\t\tlaunchSigner = discoverUnifiedLaunchSigner(launchCtx, rpcURL, out.Mint)
\t\tcancel()
\t}
''',
    '''\tif creator == "" && rpcURL != "" {
\t\tlaunchCtx, cancel := context.WithTimeout(ctx, time.Duration(budgets.LaunchTimeoutSeconds)*time.Second)
\t\tlaunchSigner = discoverUnifiedLaunchSigner(launchCtx, rpcURL, out.Mint)
\t\tcancel()
\t} else if creator == "" {
\t\tlaunchSigner.Status = "source_unavailable"
\t\tlaunchSigner.Limitations = append(launchSigner.Limitations, "Launch signer discovery requires a configured direct RPC URL.")
\t}
'''
)
replace_once(
    live_path,
    '''\t\tcoverage, rows := collectUnifiedWalletTransactions(walletCtx, rpcURL, out.Mint, target)''',
    '''\t\tcoverage, rows := h.collectUnifiedWalletTransactions(walletCtx, network, rpcURL, out.Mint, target)'''
)
start_marker = 'func collectUnifiedWalletTransactions(ctx context.Context, rpcURL, mint string, target unifiedLiveWalletTarget)'
end_marker = 'func parseUnifiedLiveTransaction('
file = Path(live_path)
text = file.read_text()
start = text.index(start_marker)
end = text.index(end_marker, start)
replacement = r'''func (h *Handler) collectUnifiedWalletTransactions(ctx context.Context, network, rpcURL, mint string, target unifiedLiveWalletTarget) (unifiedLiveWalletCoverage, []unifiedLiveTransactionRow) {
	coverage := unifiedLiveWalletCoverage{Wallet: target.Wallet, Role: target.Role, Status: "rpc_failed", Limitations: []string{}}
	rows := []unifiedLiveTransactionRow{}
	signatures, err := h.unifiedLiveSignatures(ctx, network, rpcURL, target.Wallet, unifiedLiveSignatureLimit)
	if err != nil {
		coverage.RPCFailures++
		coverage.Limitations = append(coverage.Limitations, "Wallet signatures could not be read from either configured Solana RPC provider.")
		return coverage, rows
	}
	coverage.SignaturesSeen = len(signatures)
	keys := []string{}
	infoBySignature := map[string]services.SolanaSignatureInfo{}
	for _, item := range signatures {
		if item.Err != nil || strings.TrimSpace(item.Signature) == "" {
			continue
		}
		keys = append(keys, item.Signature)
		infoBySignature[item.Signature] = item
		if len(keys) >= unifiedLiveTransactionLimit {
			break
		}
	}
	if len(keys) == 0 {
		coverage.Status = "complete_no_successful_signatures"
		return coverage, rows
	}
	transactions, batchErr := h.fetchUnifiedTransactions(ctx, network, rpcURL, keys)
	if batchErr != nil {
		coverage.RPCFailures++
		coverage.Limitations = append(coverage.Limitations, "Some recent transactions could not be parsed after primary/fallback RPC attempts: "+creatorIntelCompactError(batchErr))
	}
	for _, signature := range keys {
		tx, ok := transactions[signature]
		if !ok {
			continue
		}
		coverage.TransactionsParsed++
		if row, relevant := parseUnifiedLiveTransaction(mint, target, infoBySignature[signature], tx); relevant {
			rows = append(rows, row)
		}
	}
	coverage.RelevantTransactions = len(rows)
	switch {
	case len(transactions) < len(keys):
		coverage.Status = "partial_rpc"
	case len(rows) == 0:
		coverage.Status = "complete_no_relevant_token_delta"
	default:
		coverage.Status = "complete"
	}
	return coverage, rows
}

func (h *Handler) unifiedLiveSignatures(ctx context.Context, network, rpcURL, wallet string, limit int) ([]services.SolanaSignatureInfo, error) {
	if h != nil && h.SolanaRPC != nil {
		var out []services.SolanaSignatureInfo
		err := h.SolanaRPC.Call(ctx, network, "getSignaturesForAddress", []any{wallet, map[string]any{"limit": limit}}, &out, time.Minute)
		return out, err
	}
	return services.SolanaGetSignaturesForAddress(ctx, rpcURL, wallet, limit)
}

type unifiedTransactionFetchResult struct {
	signature string
	tx        services.SolanaTransactionResult
	err       error
}

func (h *Handler) fetchUnifiedTransactions(ctx context.Context, network, rpcURL string, signatures []string) (map[string]services.SolanaTransactionResult, error) {
	if h == nil || h.SolanaRPC == nil {
		return fetchUnifiedTransactionsLegacy(ctx, rpcURL, signatures)
	}
	out := map[string]services.SolanaTransactionResult{}
	if len(signatures) == 0 {
		return out, nil
	}
	workers := 3
	if len(signatures) < workers {
		workers = len(signatures)
	}
	jobs := make(chan string)
	results := make(chan unifiedTransactionFetchResult, len(signatures))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for signature := range jobs {
				var tx services.SolanaTransactionResult
				err := h.SolanaRPC.Call(ctx, network, "getTransaction", []any{signature, map[string]any{"encoding": "jsonParsed", "commitment": "confirmed", "maxSupportedTransactionVersion": 0}}, &tx, 24*time.Hour)
				select {
				case results <- unifiedTransactionFetchResult{signature: signature, tx: tx, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, signature := range signatures {
			select {
			case jobs <- signature:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	failures := 0
	var lastErr error
	for result := range results {
		if result.err != nil {
			failures++
			lastErr = result.err
			continue
		}
		if result.tx != nil {
			out[result.signature] = result.tx
		}
	}
	if failures > 0 {
		if lastErr == nil {
			lastErr = ctx.Err()
		}
		return out, fmt.Errorf("%d of %d transaction RPC calls failed: %w", failures, len(signatures), lastErr)
	}
	if ctx.Err() != nil && len(out) < len(signatures) {
		return out, ctx.Err()
	}
	return out, nil
}

func fetchUnifiedTransactionsLegacy(ctx context.Context, rpcURL string, signatures []string) (map[string]services.SolanaTransactionResult, error) {
	out, err := services.SolanaGetTransactionsJSONParsedBatch(ctx, rpcURL, signatures)
	if err == nil || len(out) > 0 {
		return out, err
	}
	out = map[string]services.SolanaTransactionResult{}
	var lastErr error
	for _, signature := range signatures {
		if ctx.Err() != nil {
			break
		}
		tx, singleErr := services.SolanaGetTransactionJSONParsed(ctx, rpcURL, signature)
		if singleErr != nil {
			lastErr = singleErr
			continue
		}
		if tx != nil {
			out[signature] = tx
		}
	}
	return out, lastErr
}

'''
file.write_text(text[:start] + replacement + text[end:])

# 3) Render the exact POST investigation payload instead of launching another
# expensive detail scan. Add investor-readable LP, behavior and live evidence.
ui_path = "koschei/api/public/js/security-radar-detail.js"
replace_once(
    ui_path,
    '''  const safeJSON = value => {
    try { return JSON.stringify(value ?? {}, null, 2); } catch { return '{}'; }
  };

  const state = { cards: new Map(), access: false, currentTarget: '' };
''',
    '''  const safeJSON = value => {
    try { return JSON.stringify(value ?? {}, null, 2); } catch { return '{}'; }
  };

  function gradeTone(final = {}) {
    if (final.signed !== true) return 'unknown';
    const grade = String(final.grade || '').toUpperCase();
    if (['D', 'E', 'F'].includes(grade)) return 'bad';
    if (['C', 'C-', 'D+'].includes(grade)) return 'warn';
    return 'good';
  }

  function normalizeCustomerInvestigation(envelope, target) {
    const report = envelope?.investigation_report;
    if (!report || typeof report !== 'object' || Array.isArray(report)) return null;
    const final = report.final_verdict || envelope.final_verdict || {};
    const behaviorSignals = Array.isArray(report.behavior_signals?.signals) ? report.behavior_signals.signals : [];
    const triggered = Array.isArray(final.triggered_rules) ? final.triggered_rules : [];
    const pathways = Array.isArray(report.threat_anticipation?.pathways) ? report.threat_anticipation.pathways : [];
    const reasons = triggered.map(item => item.summary || item.title || item.rule_id).filter(Boolean);
    if (!reasons.length) reasons.push(...behaviorSignals.filter(item => item.triggered === true).map(item => item.summary || item.title).filter(Boolean));
    const positives = pathways.filter(item => item.status === 'closed').map(item => item.summary || item.label).filter(Boolean);
    return {
      ...report,
      target: report.target || target,
      final_verdict: final,
      customer_scan_status: envelope.status || (final.signed ? 'ready' : 'evidence_pending'),
      customer_scan_message: envelope.message || '',
      has_live_evidence: envelope.has_live_evidence === true,
      charged: envelope.charged === true,
      warning: report.warning || {
        label: envelope.status === 'evidence_pending' ? 'KANIT EKSİKLERİYLE TAMAMLANDI' : 'KOSCHEI UNIFIED VERDICT',
        reasons,
        positive_signals: positives,
        interpretation: envelope.message || 'Koschei kanıtı, karşı kanıtı ve eksik delilleri birlikte raporlar; niyet veya gerçek kişi kimliği iddiası üretmez.'
      }
    };
  }

  function investigationStatusPanel(data) {
    const status = String(data.customer_scan_status || '').toLowerCase();
    if (!status) return '';
    const pending = status === 'evidence_pending';
    return `<section class="creator-warning"><span class="eyebrow">KOSCHEI SORUŞTURMA DURUMU</span><h3>${pending ? 'Kanıt boşlukları var — güvenli hükmü verilmedi' : 'Birleşik soruşturma tamamlandı'}</h3><p>${esc(data.customer_scan_message || (pending ? 'Erişilemeyen kaynaklar açıkça eksik delil olarak tutuldu.' : 'İmzalı deterministik hüküm ve kanıt dosyası hazırlandı.'))}</p><div class="actions"><span class="pill ${pending ? 'amber' : 'green'}">${esc(status.toUpperCase())}</span><span class="pill ${data.has_live_evidence ? 'green' : 'amber'}">CANLI KANIT ${data.has_live_evidence ? 'VAR' : 'KISMİ/YOK'}</span><span class="pill">${data.charged ? 'HAK KULLANILDI' : 'ÜCRETLENDİRİLMEDİ'}</span></div></section>`;
  }

  function lpControlPanel(lp) {
    if (!lp || typeof lp !== 'object') return '';
    const holders = Array.isArray(lp.largest_lp_holders) ? lp.largest_lp_holders : [];
    const limitations = Array.isArray(lp.limitations) ? lp.limitations : [];
    return `<section class="panel full"><span class="eyebrow">LP CONTROL DOSYASI</span><h3>Likidite kimin kontrolünde?</h3><section class="statgrid"><article class="stat"><label>Durum</label><strong>${esc(lp.status || 'unverified')}</strong><small>${esc(lp.reason_code || 'kanıt kodu yok')}</small></article><article class="stat"><label>Havuz</label><strong>${esc(short(lp.pool_address))}</strong><small>${esc(lp.pool_type || lp.pool_program || 'program çözülemedi')}</small></article><article class="stat"><label>Creator LP payı</label><strong>%${num(lp.creator_lp_share_pct, 4)}</strong><small>owner-resolved LP</small></article><article class="stat"><label>Yakılmış LP</label><strong>%${num(lp.burned_share_pct, 4)}</strong><small>${lp.locked_until ? `kilit: ${esc(lp.locked_until)}` : 'unlock doğrulanmadı'}</small></article></section><div class="two-col"><article class="panel"><h3>Rezervler</h3><div class="mini"><span>Token rezervi</span><b>${num(lp.token_reserve, 6)}</b></div><div class="mini"><span>Quote rezervi</span><b>${num(lp.quote_reserve, 6)}</b></div><div class="mini"><span>Doğrudan rezerv değeri</span><b>${money(lp.reserve_liquidity_usd)}</b></div></article><article class="panel"><h3>En büyük LP sahipleri</h3>${holders.length ? `<div class="account-list">${holders.slice(0, 8).map((item, index) => `<div class="account-row"><b>#${index + 1}</b><code>${esc(short(item.owner_wallet || item.token_account))}</code><span>${esc(item.classification || 'holder')}</span><strong>%${num(item.share_pct, 4)}</strong></div>`).join('')}</div>` : '<div class="empty">LP holder sahipliği doğrulanamadı.</div>'}</article></div>${limitations.length ? `<details><summary>LP delil sınırları</summary><ul>${limitations.map(item => `<li>${esc(item)}</li>`).join('')}</ul></details>` : ''}</section>`;
  }

  function behaviorPanel(behavior) {
    const signals = Array.isArray(behavior?.signals) ? behavior.signals : [];
    if (!signals.length) return '';
    return `<section class="panel full"><span class="eyebrow">İDDİA VE KARŞI KANIT MATRİSİ</span><h3>Deterministik davranış kuralları</h3><div class="insights">${signals.map(item => `<div class="insight ${item.triggered ? 'bad' : item.evidence_status === 'observed' ? 'good' : ''}"><div class="actions"><span class="pill ${item.triggered ? 'red' : 'green'}">${item.triggered ? 'TETİKLENDİ' : 'TETİKLENMEDİ'}</span><span class="pill violet">${esc(item.rule_id || item.evidence_status || 'rule')}</span></div><b>${esc(item.title || item.rule_id)}</b><p>${esc(item.summary || '')}</p></div>`).join('')}</div></section>`;
  }

  function liveEvidencePanel(live) {
    if (!live || typeof live !== 'object') return '';
    const transactions = Array.isArray(live.transactions) ? live.transactions : [];
    const limitations = Array.isArray(live.limitations) ? live.limitations : [];
    return `<section class="panel full"><span class="eyebrow">CANLI İŞLEM DELİLİ</span><h3>Creator ve risk-bearing holder hareketleri</h3><section class="statgrid"><article class="stat"><label>Cüzdan</label><strong>${num(live.wallets_completed, 0)}/${num(live.wallets_requested, 0)}</strong><small>${esc(live.status || 'unknown')}</small></article><article class="stat"><label>İmza</label><strong>${num(live.signatures_seen, 0)}</strong><small>bounded recent window</small></article><article class="stat"><label>Parse edilen işlem</label><strong>${num(live.transactions_parsed, 0)}</strong><small>JSON parsed</small></article><article class="stat"><label>İlgili hareket</label><strong>${num(live.relevant_transactions, 0)}</strong><small>RPC hata: ${num(live.rpc_failures, 0)}</small></article></section>${transactions.length ? `<div class="evidence-list">${transactions.map(item => `<div class="evidence-row verified"><b>${esc(item.direction || 'transfer')}</b><span>${esc(short(item.wallet))} · ${num(item.token_delta, 6)} token · ${esc(item.role || '')}</span><small>${esc(short(item.signature))}</small></div>`).join('')}</div>` : '<div class="empty">Sınırlı pencerede ilgili token hareketi alınamadı; bu, eski hareket olmadığı anlamına gelmez.</div>'}${limitations.length ? `<details><summary>Canlı tarama sınırları</summary><ul>${limitations.map(item => `<li>${esc(item)}</li>`).join('')}</ul></details>` : ''}</section>`;
  }

  const state = { cards: new Map(), access: false, currentTarget: '' };
'''
)
replace_once(
    ui_path,
    '''    const signals = final.signals || fallbackItem.signals || {};
    const risk = Number(final.risk_index ?? fallbackItem.risk_index ?? 0);
''',
    '''    const signals = final.signals || fallbackItem.signals || {};
    const rawRisk = final.risk_index ?? fallbackItem.risk_index;
    const hasNumericRisk = rawRisk !== undefined && rawRisk !== null && rawRisk !== '' && Number.isFinite(Number(rawRisk));
    const risk = hasNumericRisk ? Number(rawRisk) : 0;
    const grade = String(final.grade || fallbackItem.grade || '—').toUpperCase();
    const tone = hasNumericRisk ? riskClass(risk) : gradeTone(final);
'''
)
replace_once(
    ui_path,
    '''      ${renderVerdictCard({...data, final_verdict: final})}
      <section class="verdict-head ${riskClass(risk)}">
        <div class="scorebox"><strong>${esc(risk)}</strong><span>RISK / 100</span></div>
        <div><span class="eyebrow">${esc(warning.label || final.risk_level || 'ARVIS VERDICT')}</span><h2>${esc(final.verdict || fallbackItem.verdict || 'İmzalı ARVIS kararı')}</h2><div class="target-full">${esc(data.target || fallbackItem.target)}</div><p class="muted">${esc(final.recommendation || fallbackItem.recommendation || 'Tüm kanıtları inceleyin.')}</p><div class="actions"><span class="pill ${risk >= 65 ? 'red' : risk >= 35 ? 'amber' : 'green'}">${esc(final.risk_level || fallbackItem.risk_level || 'unknown')}</span><span class="pill">${esc(final.grade || fallbackItem.grade || '—')}</span><span class="pill violet">${esc(source.launch_platform || 'Solana')}</span></div></div>
      </section>
''',
    '''      ${renderVerdictCard({...data, final_verdict: final})}
      ${investigationStatusPanel(data)}
      <section class="verdict-head ${tone}">
        <div class="scorebox">${hasNumericRisk ? `<strong>${esc(risk)}</strong><span>RISK / 100</span>` : `<strong>${esc(grade)}</strong><span>UNIFIED GRADE</span>`}</div>
        <div><span class="eyebrow">${esc(warning.label || final.risk_level || 'KOSCHEI UNIFIED VERDICT')}</span><h2>${esc(final.verdict || fallbackItem.verdict || 'Kanıt değerlendirmesi sürüyor')}</h2><div class="target-full">${esc(data.target || fallbackItem.target)}</div><p class="muted">${esc(final.recommendation || fallbackItem.recommendation || warning.interpretation || 'Tüm kanıtları ve eksik delilleri inceleyin.')}</p><div class="actions"><span class="pill ${tone === 'bad' ? 'red' : tone === 'good' ? 'green' : 'amber'}">${esc(hasNumericRisk ? (final.risk_level || fallbackItem.risk_level || 'unknown') : final.signed ? 'SIGNED' : 'EVIDENCE PENDING')}</span><span class="pill">${esc(grade)}</span><span class="pill violet">${esc(source.launch_platform || 'Solana')}</span></div></div>
      </section>
'''
)
replace_once(
    ui_path,
    '''      ${threatPanel(data.threat_anticipation)}

      <section class="two-col">''',
    '''      ${threatPanel(data.threat_anticipation)}
      ${lpControlPanel(data.lp_control)}
      ${behaviorPanel(data.behavior_signals)}
      ${liveEvidencePanel(data.full_scan_live_evidence)}

      <section class="two-col">'''
)
replace_once(
    ui_path,
    '''      const items = await loadFeed();
      const item = items.find(row => String(row.target || '').toLowerCase() === target.toLowerCase()) || data.final_verdict || {};
      await openDetail(target, item);
''',
    '''      const directReport = normalizeCustomerInvestigation(data, target);
      if (directReport) {
        renderDetail(directReport, data.final_verdict || {});
        notice(data.status === 'evidence_pending' ? 'Soruşturma tamamlandı; erişilemeyen kanıtlar açıkça işaretlendi.' : 'Birleşik soruşturma ve yatırım riski dosyası hazırlandı.');
        loadFeed().catch(() => {});
        return;
      }
      const items = await loadFeed();
      const item = items.find(row => String(row.target || '').toLowerCase() === target.toLowerCase()) || data.final_verdict || {};
      await openDetail(target, item);
'''
)

verify_path = Path("koschei/api/scripts/verify-customer-investigation-ui.js")
verify_path.write_text(r'''const fs = require('fs');
const source = fs.readFileSync('public/js/security-radar-detail.js', 'utf8');
const required = [
  'normalizeCustomerInvestigation',
  'envelope?.investigation_report',
  'renderDetail(directReport',
  'lpControlPanel(data.lp_control)',
  'liveEvidencePanel(data.full_scan_live_evidence)',
  'behaviorPanel(data.behavior_signals)',
  'UNIFIED GRADE',
  'EVIDENCE PENDING'
];
for (const marker of required) {
  if (!source.includes(marker)) throw new Error(`missing customer investigation UI marker: ${marker}`);
}
const postBlock = source.slice(source.indexOf("api('/api/v1/radar/check'"), source.indexOf('async function boot'));
if (!postBlock.includes('data.investigation_report') && !postBlock.includes('normalizeCustomerInvestigation(data, target)')) {
  throw new Error('POST response is not consumed as an investigation report');
}
if (postBlock.indexOf('renderDetail(directReport') > postBlock.indexOf('await openDetail(target, item)')) {
  throw new Error('direct investigation rendering must precede legacy detail fallback');
}
console.log('customer investigation UI contract verified');
''')

# 4) Make JS CI inventory self-healing and add the new contract verifier.
ci_path = ".github/workflows/api-ci.yml"
ci = Path(ci_path).read_text()
start = ci.index('          node --check public/js/public-solana-scan.js')
end = ci.index('          node --check ../../oss/verifier/typescript/verify-dossier.mjs', start)
ci = ci[:start] + '''          for file in public/js/*.js; do
            node --check "$file"
          done
''' + ci[end:]
ci = ci.replace('          node scripts/verify-live-evidence-card.js\n', '          node scripts/verify-live-evidence-card.js\n          node scripts/verify-customer-investigation-ui.js\n', 1)
Path(ci_path).write_text(ci)

# Temporary patch machinery must not remain in the product diff.
Path('.github/scripts/apply_customer_investigation_ui_rpc.py').unlink(missing_ok=True)
Path('.github/workflows/apply-customer-investigation-ui-rpc.yml').unlink(missing_ok=True)
