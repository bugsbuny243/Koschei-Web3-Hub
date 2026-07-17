package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPumpHighVolumeThresholdUSD = 500000.0
	defaultPumpHighVolumePoll         = 5 * time.Minute
	defaultPumpHighVolumeCooldown     = 6 * time.Hour
	defaultPumpHighVolumeAttemptPause = 30 * time.Minute
	defaultPumpHighVolumePageSize     = 900
	defaultPumpHighVolumeMaxReports   = 1
	dexScreenerTokenBatchSize         = 30
	pumpHighVolumeEventType           = "pumpportal_high_volume_24h"
	pumpHighVolumeSource              = "pump_volume_gate"
)

// PumpRadarCandidate is a PumpPortal-discovered mint eligible for selective
// market-volume monitoring. Discovery metadata is evidence-scoped and does not
// claim creator identity beyond the source-reported wallet relation.
type PumpRadarCandidate struct {
	Mint       string    `json:"mint"`
	Name       string    `json:"name,omitempty"`
	Symbol     string    `json:"symbol,omitempty"`
	Creator    string    `json:"creator,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
}

// PumpTokenMarket is the aggregated 24-hour market surface for one mint.
// Volume is summed across unique Solana pairs returned for that token.
type PumpTokenMarket struct {
	Mint                 string    `json:"mint"`
	Name                 string    `json:"name,omitempty"`
	Symbol               string    `json:"symbol,omitempty"`
	Volume24hUSD         float64   `json:"volume_24h_usd"`
	PairCount            int       `json:"pair_count"`
	BestPairAddress      string    `json:"best_pair_address,omitempty"`
	BestPairDEX          string    `json:"best_pair_dex,omitempty"`
	BestPairVolume24hUSD float64   `json:"best_pair_volume_24h_usd"`
	LiquidityUSD         float64   `json:"liquidity_usd"`
	MarketCapUSD         float64   `json:"market_cap_usd"`
	FDVUSD               float64   `json:"fdv_usd"`
	Provider             string    `json:"provider"`
	ObservedAt           time.Time `json:"observed_at"`
}

type PumpHighVolumeOwnerItem struct {
	EventID        string         `json:"event_id"`
	Target         string         `json:"target"`
	Name           string         `json:"name,omitempty"`
	Symbol         string         `json:"symbol,omitempty"`
	Creator        string         `json:"creator,omitempty"`
	Volume24hUSD   float64        `json:"volume_24h_usd"`
	ThresholdUSD   float64        `json:"threshold_usd"`
	PairCount      int            `json:"pair_count"`
	LiquidityUSD   float64        `json:"liquidity_usd"`
	MarketCapUSD   float64        `json:"market_cap_usd"`
	VolumeProvider string         `json:"volume_provider"`
	ReportStatus   string         `json:"report_status"`
	RiskIndex      *int           `json:"risk_index,omitempty"`
	RiskLevel      string         `json:"risk_level,omitempty"`
	Verdict        string         `json:"verdict,omitempty"`
	ReportAt       *time.Time     `json:"report_at,omitempty"`
	ObservedAt     time.Time      `json:"observed_at"`
	Signals        map[string]any `json:"signals"`
}

type PumpVolumeProvider interface {
	Fetch24hVolumes(context.Context, []string) (map[string]PumpTokenMarket, error)
}

type DexScreenerPumpVolumeClient struct {
	Endpoint string
	Client   *http.Client
}

type dexScreenerTokenPair struct {
	ChainID     string `json:"chainId"`
	DexID       string `json:"dexId"`
	PairAddress string `json:"pairAddress"`
	BaseToken   struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
	} `json:"baseToken"`
	QuoteToken struct {
		Address string `json:"address"`
		Name    string `json:"name"`
		Symbol  string `json:"symbol"`
	} `json:"quoteToken"`
	Volume    map[string]float64 `json:"volume"`
	Liquidity *struct {
		USD float64 `json:"usd"`
	} `json:"liquidity"`
	MarketCap float64 `json:"marketCap"`
	FDV       float64 `json:"fdv"`
}

func NewDexScreenerPumpVolumeClient() *DexScreenerPumpVolumeClient {
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("PUMP_VOLUME_RADAR_DEXSCREENER_ENDPOINT")), "/")
	if endpoint == "" {
		endpoint = "https://api.dexscreener.com/tokens/v1/solana"
	}
	return &DexScreenerPumpVolumeClient{Endpoint: endpoint, Client: &http.Client{Timeout: 15 * time.Second}}
}

func (c *DexScreenerPumpVolumeClient) Fetch24hVolumes(ctx context.Context, mints []string) (map[string]PumpTokenMarket, error) {
	out := map[string]PumpTokenMarket{}
	mints = uniquePumpMints(mints)
	if len(mints) == 0 {
		return out, nil
	}
	if len(mints) > dexScreenerTokenBatchSize {
		return nil, fmt.Errorf("dexscreener token batch exceeds %d addresses", dexScreenerTokenBatchSize)
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	endpoint := strings.TrimRight(strings.TrimSpace(c.Endpoint), "/")
	if endpoint == "" {
		endpoint = "https://api.dexscreener.com/tokens/v1/solana"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/"+strings.Join(mints, ","), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Koschei-ARVIS-Pump-Volume-Radar/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dexscreener returned HTTP %d", resp.StatusCode)
	}
	pairs := []dexScreenerTokenPair{}
	if err := json.Unmarshal(body, &pairs); err != nil {
		return nil, fmt.Errorf("decode dexscreener token pairs: %w", err)
	}
	requested := map[string]bool{}
	for _, mint := range mints {
		requested[mint] = true
		out[mint] = PumpTokenMarket{Mint: mint, Provider: "dexscreener", ObservedAt: time.Now().UTC()}
	}
	seenPair := map[string]map[string]bool{}
	for _, pair := range pairs {
		if !strings.EqualFold(strings.TrimSpace(pair.ChainID), "solana") {
			continue
		}
		addresses := []struct {
			Address string
			Name    string
			Symbol  string
		}{
			{pair.BaseToken.Address, pair.BaseToken.Name, pair.BaseToken.Symbol},
			{pair.QuoteToken.Address, pair.QuoteToken.Name, pair.QuoteToken.Symbol},
		}
		for _, token := range addresses {
			mint := strings.TrimSpace(token.Address)
			if !requested[mint] {
				continue
			}
			pairKey := strings.TrimSpace(pair.PairAddress)
			if pairKey == "" {
				pairKey = strings.TrimSpace(pair.DexID) + "|" + pair.BaseToken.Address + "|" + pair.QuoteToken.Address
			}
			if seenPair[mint] == nil {
				seenPair[mint] = map[string]bool{}
			}
			if seenPair[mint][pairKey] {
				continue
			}
			seenPair[mint][pairKey] = true
			market := out[mint]
			volume := pair.Volume["h24"]
			if volume < 0 {
				volume = 0
			}
			market.Volume24hUSD += volume
			market.PairCount++
			if market.Name == "" {
				market.Name = strings.TrimSpace(token.Name)
			}
			if market.Symbol == "" {
				market.Symbol = strings.TrimSpace(token.Symbol)
			}
			if volume > market.BestPairVolume24hUSD {
				market.BestPairVolume24hUSD = volume
				market.BestPairAddress = strings.TrimSpace(pair.PairAddress)
				market.BestPairDEX = strings.TrimSpace(pair.DexID)
			}
			if pair.Liquidity != nil && pair.Liquidity.USD > 0 {
				market.LiquidityUSD += pair.Liquidity.USD
			}
			if pair.MarketCap > market.MarketCapUSD {
				market.MarketCapUSD = pair.MarketCap
			}
			if pair.FDV > market.FDVUSD {
				market.FDVUSD = pair.FDV
			}
			market.Volume24hUSD = roundPumpUSD(market.Volume24hUSD)
			market.BestPairVolume24hUSD = roundPumpUSD(market.BestPairVolume24hUSD)
			market.LiquidityUSD = roundPumpUSD(market.LiquidityUSD)
			market.MarketCapUSD = roundPumpUSD(market.MarketCapUSD)
			market.FDVUSD = roundPumpUSD(market.FDVUSD)
			out[mint] = market
		}
	}
	return out, nil
}

// PumpHighVolumeRadarEnabled defaults to the PumpPortal discovery switch. A
// dedicated override exists, but no global RPC stream has to be force-enabled.
func PumpHighVolumeRadarEnabled() bool {
	if raw := strings.TrimSpace(os.Getenv("PUMP_HIGH_VOLUME_RADAR_ENABLED")); raw != "" {
		return envBool("PUMP_HIGH_VOLUME_RADAR_ENABLED")
	}
	return LoadPumpPortalConfigFromEnv().Enabled
}

func PumpHighVolumeThresholdUSD() float64 {
	if raw := strings.TrimSpace(os.Getenv("PUMP_HIGH_VOLUME_MIN_24H_USD")); raw != "" {
		if value, err := strconv.ParseFloat(raw, 64); err == nil && value >= 1000 && value <= 1000000000000 {
			return value
		}
	}
	return defaultPumpHighVolumeThresholdUSD
}

func PumpHighVolumePollInterval() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("PUMP_HIGH_VOLUME_POLL_SECONDS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 60 && value <= 3600 {
			return time.Duration(value) * time.Second
		}
	}
	return defaultPumpHighVolumePoll
}

func pumpHighVolumeReportCooldown() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("PUMP_HIGH_VOLUME_REPORT_COOLDOWN_SECONDS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 900 && value <= 86400 {
			return time.Duration(value) * time.Second
		}
	}
	return defaultPumpHighVolumeCooldown
}

func pumpHighVolumeAttemptCooldown() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("PUMP_HIGH_VOLUME_ATTEMPT_COOLDOWN_SECONDS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 300 && value <= 21600 {
			return time.Duration(value) * time.Second
		}
	}
	return defaultPumpHighVolumeAttemptPause
}

func pumpHighVolumePageSize() int {
	if raw := strings.TrimSpace(os.Getenv("PUMP_HIGH_VOLUME_CANDIDATE_PAGE_SIZE")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 30 && value <= 3000 {
			return value
		}
	}
	return defaultPumpHighVolumePageSize
}

func pumpHighVolumeMaxReportsPerCycle() int {
	if OwnerUnlimitedAutomaticScanningEnabled() {
		return 0
	}
	if raw := strings.TrimSpace(os.Getenv("PUMP_HIGH_VOLUME_MAX_REPORTS_PER_CYCLE")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 1 && value <= 20 {
			return value
		}
	}
	return defaultPumpHighVolumeMaxReports
}

type PumpHighVolumeRadarWorker struct {
	Store              *SecurityRadarStore
	Volumes            PumpVolumeProvider
	ThresholdUSD       float64
	PollEvery          time.Duration
	ReportCooldown     time.Duration
	AttemptCooldown    time.Duration
	CandidatePageSize  int
	MaxReportsPerCycle int

	mu           sync.Mutex
	cursorAt     time.Time
	cursorTarget string
}

func NewPumpHighVolumeRadarWorker(store *SecurityRadarStore, provider PumpVolumeProvider) *PumpHighVolumeRadarWorker {
	if provider == nil {
		provider = NewDexScreenerPumpVolumeClient()
	}
	return &PumpHighVolumeRadarWorker{
		Store: store, Volumes: provider,
		ThresholdUSD: PumpHighVolumeThresholdUSD(), PollEvery: PumpHighVolumePollInterval(),
		ReportCooldown: pumpHighVolumeReportCooldown(), AttemptCooldown: pumpHighVolumeAttemptCooldown(),
		CandidatePageSize: pumpHighVolumePageSize(), MaxReportsPerCycle: pumpHighVolumeMaxReportsPerCycle(),
	}
}

func (w *PumpHighVolumeRadarWorker) Start(ctx context.Context) {
	if w == nil || w.Store == nil || w.Store.DB == nil || w.Volumes == nil {
		return
	}
	if w.PollEvery <= 0 {
		w.PollEvery = defaultPumpHighVolumePoll
	}
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
				log.Printf("pump high-volume radar cycle failed: %v", err)
			}
			timer.Reset(w.PollEvery)
		}
	}
}

func (w *PumpHighVolumeRadarWorker) RunOnce(ctx context.Context) error {
	if w == nil || w.Store == nil || w.Store.DB == nil || w.Volumes == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	pageSize := w.CandidatePageSize
	if pageSize <= 0 {
		pageSize = defaultPumpHighVolumePageSize
	}
	candidates, err := w.Store.ListPumpPortalCandidates(ctx, pageSize, w.cursorAt, w.cursorTarget)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		w.cursorAt = time.Time{}
		w.cursorTarget = ""
		return nil
	}
	last := candidates[len(candidates)-1]
	if len(candidates) < pageSize {
		w.cursorAt = time.Time{}
		w.cursorTarget = ""
	} else {
		w.cursorAt = last.ObservedAt
		w.cursorTarget = last.Mint
	}

	qualified := []struct {
		Candidate PumpRadarCandidate
		Market    PumpTokenMarket
	}{}
	candidateByMint := map[string]PumpRadarCandidate{}
	for _, candidate := range candidates {
		candidateByMint[candidate.Mint] = candidate
	}
	for start := 0; start < len(candidates); start += dexScreenerTokenBatchSize {
		end := start + dexScreenerTokenBatchSize
		if end > len(candidates) {
			end = len(candidates)
		}
		mints := make([]string, 0, end-start)
		for _, candidate := range candidates[start:end] {
			mints = append(mints, candidate.Mint)
		}
		markets, fetchErr := w.Volumes.Fetch24hVolumes(ctx, mints)
		if fetchErr != nil {
			log.Printf("pump high-volume market batch unavailable: %v", fetchErr)
			continue
		}
		for mint, market := range markets {
			if market.Volume24hUSD < w.ThresholdUSD {
				continue
			}
			candidate, ok := candidateByMint[mint]
			if !ok {
				continue
			}
			if market.Name == "" {
				market.Name = candidate.Name
			}
			if market.Symbol == "" {
				market.Symbol = candidate.Symbol
			}
			qualified = append(qualified, struct {
				Candidate PumpRadarCandidate
				Market    PumpTokenMarket
			}{candidate, market})
		}
	}
	sort.SliceStable(qualified, func(i, j int) bool { return qualified[i].Market.Volume24hUSD > qualified[j].Market.Volume24hUSD })

	reports := 0
	for _, item := range qualified {
		attempted, attemptErr := w.Store.PumpHighVolumeAttemptedRecently(ctx, item.Candidate.Mint, w.AttemptCooldown)
		if attemptErr != nil {
			log.Printf("pump high-volume attempt cooldown lookup failed mint=%s: %v", item.Candidate.Mint, attemptErr)
			continue
		}
		eventID, eventErr := w.Store.RecordPumpHighVolumeObservation(ctx, item.Candidate, item.Market, w.ThresholdUSD)
		if eventErr != nil {
			log.Printf("pump high-volume event store failed mint=%s: %v", item.Candidate.Mint, eventErr)
			continue
		}
		recent, recentErr := w.Store.PumpHighVolumeReportedRecently(ctx, item.Candidate.Mint, w.ReportCooldown)
		if recentErr != nil {
			log.Printf("pump high-volume cooldown lookup failed mint=%s: %v", item.Candidate.Mint, recentErr)
			continue
		}
		if recent || attempted || (w.MaxReportsPerCycle > 0 && reports >= w.MaxReportsPerCycle) {
			continue
		}
		if markErr := w.Store.MarkPumpHighVolumeAttempted(ctx, eventID); markErr != nil {
			log.Printf("pump high-volume attempt marker failed mint=%s: %v", item.Candidate.Mint, markErr)
			continue
		}
		if err := w.scanAndStore(ctx, eventID, item.Candidate, item.Market); err != nil {
			log.Printf("pump high-volume ARVIS report failed mint=%s volume24h=%.2f: %v", item.Candidate.Mint, item.Market.Volume24hUSD, err)
			continue
		}
		reports++
	}
	if len(qualified) > 0 {
		log.Printf("pump high-volume radar cycle: candidates=%d qualified=%d reports=%d threshold_usd=%.0f", len(candidates), len(qualified), reports, w.ThresholdUSD)
	}
	return nil
}

func (w *PumpHighVolumeRadarWorker) scanAndStore(ctx context.Context, eventID string, candidate PumpRadarCandidate, market PumpTokenMarket) error {
	req := SecurityRadarRequest{Target: candidate.Mint, Network: "solana-mainnet", Mode: "live_stream:" + ModulePumpSybilRadar}
	analysis := AnalyzeArvisRadars(req)
	bundle := EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	roles := ArvisHolderRolesFromBundle(bundle)
	cluster := ArvisHolderClusterFromBundle(bundle)
	launchBlockTime := int64(0)
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(cluster.LaunchEstimateAt)); err == nil {
		launchBlockTime = parsed.Unix()
	}
	forensicsCtx, cancelForensics := context.WithTimeout(ctx, 120*time.Second)
	forensics := AnalyzeLaunchForensics(forensicsCtx, w.Store.DB, firstSecurityRadarEnv("SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL"), candidate.Mint, candidate.Creator, roles, launchBlockTime, cluster.LaunchEstimateSlot)
	cancelForensics()
	analysis = ApplyLaunchForensicsToAnalysis(analysis, req, forensics)
	w.Store.CaptureLaunchForensicsFloor(ctx, candidate.Mint, "solana-mainnet", forensics)
	bundle = EvidenceBackedSecurityRadarBundle(analysis.Bundle)
	arms := ArvisArmsFromBundle(bundle)
	if len(arms) == 0 {
		arms = analysis.Arms
	}
	volumeSignals := pumpHighVolumeSignals(candidate, market, w.ThresholdUSD)
	inserted := 0
	var firstErr error
	for _, arm := range arms {
		if !arm.Signed || !pumpPortalArmVerified(arm) {
			continue
		}
		arm.Signals = mergePumpPortalSignals(arm.Signals, volumeSignals)
		arm.Signals["auto_volume_gate"] = true
		arm.Signals["owner_detail_visible"] = true
		arm.Signals["customer_detail_visible"] = false
		arm.Signals["source_verified_pump_event"] = true
		arm.Signals["source_module"] = ModulePumpSybilRadar
		if arm.ModuleID == ModuleFinalVerdictEngine {
			arm.Signals["warning_label"] = pumpPortalWarningLabel(arm.RiskIndex)
		}
		arm.Evidence = append(arm.Evidence,
			fmt.Sprintf("PumpPortal-discovered mint crossed the automatic 24-hour USD volume gate: $%.2f >= $%.2f.", market.Volume24hUSD, w.ThresholdUSD),
			"Market volume was obtained from DexScreener pair data; volume alone is not a safety or fraud verdict.",
		)
		signature := arvisStreamScopedVerdictSignature(arm.Signature, arm.ModuleID, eventID)
		_, err := w.Store.InsertVerdict(ctx, SecurityRadarVerdictRecord{
			EventID: eventID, ModuleID: arm.ModuleID, Target: arm.Target, TargetType: "token", Network: arm.Network,
			Grade: arm.Grade, RiskIndex: arm.RiskIndex, RiskLevel: arm.RiskLevel, Verdict: arm.Verdict,
			Recommendation: arm.Recommendation, Evidence: arm.Evidence, Signals: arm.Signals,
			RuleVersion: arm.RuleVersion, Signed: arm.Signed, Signature: signature,
			Source: pumpHighVolumeSource, EventType: pumpHighVolumeEventType, Provider: "alchemy+pumpportal+dexscreener",
		})
		if err != nil && firstErr == nil {
			firstErr = err
		} else if err == nil {
			inserted++
		}
	}
	if inserted == 0 && firstErr == nil {
		return fmt.Errorf("no signed live-evidence ARVIS arm was produced")
	}
	return firstErr
}

func (s *SecurityRadarStore) ListPumpPortalCandidates(ctx context.Context, limit int, before time.Time, beforeTarget string) ([]PumpRadarCandidate, error) {
	if s == nil || s.DB == nil {
		return []PumpRadarCandidate{}, nil
	}
	if limit <= 0 || limit > 3000 {
		limit = defaultPumpHighVolumePageSize
	}
	condition := ""
	args := []any{limit}
	if !before.IsZero() {
		condition = "WHERE observed_at < $2 OR (observed_at = $2 AND lower(mint) < lower($3))"
		args = append(args, before.UTC(), strings.TrimSpace(beforeTarget))
	}
	query := `
		WITH latest AS (
			SELECT DISTINCT ON (lower(target))
				target AS mint,
				COALESCE(NULLIF(signals->>'token_name',''),NULLIF(raw_summary->>'name',''),'') AS name,
				COALESCE(NULLIF(signals->>'token_symbol',''),NULLIF(raw_summary->>'symbol',''),'') AS symbol,
				COALESCE(NULLIF(signals->>'creator_wallet',''),NULLIF(signals->>'creator',''),NULLIF(raw_summary->>'creator',''),'') AS creator,
				created_at AS observed_at
			FROM security_radar_events
			WHERE source='pumpportal' AND target_type='token' AND btrim(target)<>''
			ORDER BY lower(target), created_at DESC, id DESC
		)
		SELECT mint,name,symbol,creator,observed_at
		FROM latest
		` + condition + `
		ORDER BY observed_at DESC, lower(mint) DESC
		LIMIT $1`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PumpRadarCandidate{}
	for rows.Next() {
		var item PumpRadarCandidate
		if err := rows.Scan(&item.Mint, &item.Name, &item.Symbol, &item.Creator, &item.ObservedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SecurityRadarStore) RecordPumpHighVolumeObservation(ctx context.Context, candidate PumpRadarCandidate, market PumpTokenMarket, threshold float64) (string, error) {
	if s == nil || s.DB == nil {
		return "", nil
	}
	signals := pumpHighVolumeSignals(candidate, market, threshold)
	bucket := market.ObservedAt.UTC().Truncate(time.Hour)
	if bucket.IsZero() {
		bucket = time.Now().UTC().Truncate(time.Hour)
	}
	signature := pumpHighVolumeObservationSignature(candidate.Mint, bucket)
	return s.InsertEvent(ctx, SecurityRadarEventRecord{
		ModuleID: ModulePumpSybilRadar, Target: candidate.Mint, TargetType: "token", Network: "solana-mainnet",
		Signature: signature, SourceAddress: candidate.Creator, EventType: pumpHighVolumeEventType,
		Signals: signals, RawSummary: map[string]any{
			"mint": candidate.Mint, "name": firstNonEmptyPumpPortal(candidate.Name, market.Name),
			"symbol": firstNonEmptyPumpPortal(candidate.Symbol, market.Symbol), "creator": candidate.Creator,
			"volume_24h_usd": market.Volume24hUSD, "threshold_usd": threshold,
			"pair_count": market.PairCount, "provider": market.Provider, "observed_at": market.ObservedAt,
		}, Source: pumpHighVolumeSource,
	})
}

func (s *SecurityRadarStore) PumpHighVolumeReportedRecently(ctx context.Context, mint string, cooldown time.Duration) (bool, error) {
	if s == nil || s.DB == nil {
		return false, nil
	}
	if cooldown <= 0 {
		cooldown = defaultPumpHighVolumeCooldown
	}
	var exists bool
	err := s.DB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM security_radar_verdicts
			WHERE module_id='final_verdict_engine' AND signed=true AND source=$1
			  AND lower(target)=lower($2) AND created_at >= now()-($3 * interval '1 second')
		)`, pumpHighVolumeSource, strings.TrimSpace(mint), int64(cooldown/time.Second)).Scan(&exists)
	return exists, err
}

func (s *SecurityRadarStore) PumpHighVolumeAttemptedRecently(ctx context.Context, mint string, cooldown time.Duration) (bool, error) {
	if s == nil || s.DB == nil {
		return false, nil
	}
	if cooldown <= 0 {
		cooldown = defaultPumpHighVolumeAttemptPause
	}
	var exists bool
	err := s.DB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM security_radar_events
			WHERE event_type=$1 AND source=$2 AND lower(target)=lower($3)
			  AND COALESCE(signals->>'auto_scan_attempted','false')='true'
			  AND updated_at >= now()-($4 * interval '1 second')
		)`, pumpHighVolumeEventType, pumpHighVolumeSource, strings.TrimSpace(mint), int64(cooldown/time.Second)).Scan(&exists)
	return exists, err
}

func (s *SecurityRadarStore) MarkPumpHighVolumeAttempted(ctx context.Context, eventID string) error {
	if s == nil || s.DB == nil || strings.TrimSpace(eventID) == "" {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `
		UPDATE security_radar_events
		SET signals=jsonb_set(COALESCE(signals,'{}'::jsonb),'{auto_scan_attempted}','true'::jsonb,true), updated_at=now()
		WHERE id=$1::uuid`, strings.TrimSpace(eventID))
	return err
}

func (s *SecurityRadarStore) LatestPumpHighVolumeReports(ctx context.Context, limit int) ([]PumpHighVolumeOwnerItem, error) {
	if s == nil || s.DB == nil {
		return []PumpHighVolumeOwnerItem{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := s.DB.QueryContext(ctx, `
		WITH latest_events AS (
			SELECT DISTINCT ON (lower(e.target)) e.id::text, e.target, e.signals, e.created_at
			FROM security_radar_events e
			WHERE e.event_type=$1 AND e.source=$2 AND btrim(e.target)<>''
			ORDER BY lower(e.target), e.created_at DESC, e.id DESC
		)
		SELECT e.id,e.target,e.signals,e.created_at,
		       v.risk_index,v.risk_level,v.verdict,v.created_at
		FROM latest_events e
		LEFT JOIN LATERAL (
			SELECT risk_index,risk_level,verdict,created_at
			FROM security_radar_verdicts v
			WHERE lower(v.target)=lower(e.target) AND v.module_id='final_verdict_engine'
			  AND v.signed=true AND v.source=$2
			ORDER BY v.created_at DESC,v.id DESC LIMIT 1
		) v ON true
		ORDER BY COALESCE((e.signals->>'volume_24h_usd')::numeric,0) DESC,e.created_at DESC
		LIMIT $3`, pumpHighVolumeEventType, pumpHighVolumeSource, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PumpHighVolumeOwnerItem{}
	for rows.Next() {
		var item PumpHighVolumeOwnerItem
		var signalsRaw []byte
		var risk sql.NullInt64
		var level, verdict sql.NullString
		var reportAt sql.NullTime
		if err := rows.Scan(&item.EventID, &item.Target, &signalsRaw, &item.ObservedAt, &risk, &level, &verdict, &reportAt); err != nil {
			return nil, err
		}
		item.Signals = map[string]any{}
		_ = json.Unmarshal(signalsRaw, &item.Signals)
		item.Name = pumpSignalString(item.Signals, "token_name", "name")
		item.Symbol = pumpSignalString(item.Signals, "token_symbol", "symbol")
		item.Creator = pumpSignalString(item.Signals, "creator_wallet", "creator")
		item.Volume24hUSD = pumpSignalFloat(item.Signals, "volume_24h_usd")
		item.ThresholdUSD = pumpSignalFloat(item.Signals, "volume_threshold_usd")
		item.PairCount = int(pumpSignalFloat(item.Signals, "volume_pair_count"))
		item.LiquidityUSD = pumpSignalFloat(item.Signals, "liquidity_usd")
		item.MarketCapUSD = pumpSignalFloat(item.Signals, "market_cap_usd")
		item.VolumeProvider = pumpSignalString(item.Signals, "volume_provider")
		item.ReportStatus = "queued"
		if pumpSignalBool(item.Signals, "auto_scan_attempted") {
			item.ReportStatus = "evidence_pending"
		}
		if risk.Valid {
			value := int(risk.Int64)
			item.RiskIndex = &value
			item.RiskLevel = level.String
			item.Verdict = verdict.String
			item.ReportStatus = "completed"
		}
		if reportAt.Valid {
			value := reportAt.Time.UTC()
			item.ReportAt = &value
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func pumpHighVolumeSignals(candidate PumpRadarCandidate, market PumpTokenMarket, threshold float64) map[string]any {
	return map[string]any{
		"source": "pumpportal+dexscreener", "launch_platform": "pump.fun", "mint": candidate.Mint,
		"name": firstNonEmptyPumpPortal(candidate.Name, market.Name), "token_name": firstNonEmptyPumpPortal(candidate.Name, market.Name),
		"symbol": firstNonEmptyPumpPortal(candidate.Symbol, market.Symbol), "token_symbol": firstNonEmptyPumpPortal(candidate.Symbol, market.Symbol),
		"creator": candidate.Creator, "creator_wallet": candidate.Creator, "creator_relation_scope": "source-reported PumpPortal launch metadata",
		"creator_identity_claimed": false, "source_verified_pump_event": true,
		"auto_volume_gate": true, "auto_scan_attempted": false, "volume_window": "24h", "volume_currency": "USD",
		"volume_24h_usd": roundPumpUSD(market.Volume24hUSD), "volume_threshold_usd": roundPumpUSD(threshold),
		"volume_pair_count": market.PairCount, "volume_provider": firstNonEmptyPumpPortal(market.Provider, "dexscreener"),
		"best_pair_address": market.BestPairAddress, "best_pair_dex": market.BestPairDEX,
		"best_pair_volume_24h_usd": roundPumpUSD(market.BestPairVolume24hUSD),
		"liquidity_usd":            roundPumpUSD(market.LiquidityUSD), "market_cap_usd": roundPumpUSD(market.MarketCapUSD), "fdv_usd": roundPumpUSD(market.FDVUSD),
		"volume_observed_at":   market.ObservedAt.UTC().Format(time.RFC3339),
		"owner_detail_visible": true, "customer_detail_visible": false,
	}
}

func pumpHighVolumeObservationSignature(mint string, bucket time.Time) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(mint) + "|" + bucket.UTC().Format("20060102T15")))
	return "pump-volume-" + hex.EncodeToString(sum[:])
}

func uniquePumpMints(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func roundPumpUSD(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return float64(int64(value*100+0.5)) / 100
}

func pumpSignalString(signals map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(signals[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func pumpSignalFloat(signals map[string]any, key string) float64 {
	switch value := signals[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		parsed, _ := value.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed
	default:
		return 0
	}
}

func pumpSignalBool(signals map[string]any, key string) bool {
	switch value := signals[key].(type) {
	case bool:
		return value
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
		return parsed
	default:
		return false
	}
}
