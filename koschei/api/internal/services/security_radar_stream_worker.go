package services

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	SecurityRadarStreamProvider = "solana_wss"
	SecurityRadarStreamModeLogs = "logs_subscribe"
)

type SecurityRadarStreamEventRecord struct {
	Provider        string
	StreamMode      string
	Network         string
	ModuleID        string
	EventType       string
	Target          string
	TargetType      string
	Signature       string
	Slot            int64
	ProgramID       string
	EvidenceQuality string
	Decoded         map[string]any
	RawEvent        map[string]any
}

type SecurityRadarStreamWorker struct {
	Store      *SecurityRadarStore
	WSSURL     string
	RPCURL     string
	Network    string
	BufferSize int
	Queue      chan SecurityRadarStreamEventRecord
}

func StartSecurityRadarStreamIfEnabled(ctx context.Context, db *sql.DB) func() {
	if db == nil || !securityRadarStreamEnabled() {
		return func() {}
	}
	wssURL := resolveSecurityRadarWSSURL()
	if wssURL == "" {
		log.Printf("security radar SBX-1 stream not started: no WSS URL could be resolved from SOLANA_WSS_URL, provider WSS env, SOLANA_RPC_URL, or ALCHEMY_API_KEY")
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	worker := NewSecurityRadarStreamWorker(NewSecurityRadarStore(db), wssURL, resolveSecurityRadarRPCURL())
	go worker.Start(ctx)
	return cancel
}

func securityRadarStreamEnabled() bool {
	return envBool("RADAR_STREAM_ENABLED") || envBool("KOSCHEI_AUTO_RADAR_ENABLED") || strings.EqualFold(strings.TrimSpace(os.Getenv("KOSCHEI_SOLANA_WATCH_MODE")), "stream")
}

func resolveSecurityRadarWSSURL() string {
	if v := firstSecurityRadarEnv("SOLANA_WSS_URL", "ALCHEMY_SOLANA_WSS_URL", "HELIUS_SOLANA_WSS_URL", "QUICKNODE_SOLANA_WSS_URL"); v != "" {
		return v
	}
	if rpc := strings.TrimSpace(os.Getenv("SOLANA_RPC_URL")); rpc != "" {
		if strings.HasPrefix(rpc, "https://") {
			return "wss://" + strings.TrimPrefix(rpc, "https://")
		}
		if strings.HasPrefix(rpc, "http://") {
			return "ws://" + strings.TrimPrefix(rpc, "http://")
		}
	}
	if key := strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")); key != "" {
		return "wss://solana-mainnet.g.alchemy.com/v2/" + key
	}
	return ""
}

func resolveSecurityRadarRPCURL() string {
	if v := firstSecurityRadarEnv("SOLANA_RPC_URL", "ALCHEMY_SOLANA_RPC_URL", "HELIUS_SOLANA_RPC_URL", "QUICKNODE_SOLANA_RPC_URL"); v != "" {
		return v
	}
	if wss := strings.TrimSpace(os.Getenv("SOLANA_WSS_URL")); wss != "" {
		if strings.HasPrefix(wss, "wss://") {
			return "https://" + strings.TrimPrefix(wss, "wss://")
		}
		if strings.HasPrefix(wss, "ws://") {
			return "http://" + strings.TrimPrefix(wss, "ws://")
		}
	}
	if key := strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")); key != "" {
		return "https://solana-mainnet.g.alchemy.com/v2/" + key
	}
	return ""
}

func envBool(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") || strings.EqualFold(v, "on")
}

func NewSecurityRadarStreamWorker(store *SecurityRadarStore, wssURL string, rpcURL string) *SecurityRadarStreamWorker {
	bufferSize := 5000
	if raw := strings.TrimSpace(os.Getenv("RADAR_EVENT_BUFFER_SIZE")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100000 {
			bufferSize = n
		}
	}
	return &SecurityRadarStreamWorker{Store: store, WSSURL: strings.TrimSpace(wssURL), RPCURL: strings.TrimSpace(rpcURL), Network: firstRadarValue(os.Getenv("RADAR_STREAM_NETWORK"), "solana-mainnet"), BufferSize: bufferSize, Queue: make(chan SecurityRadarStreamEventRecord, bufferSize)}
}

func (w *SecurityRadarStreamWorker) Start(ctx context.Context) {
	if w == nil || w.Store == nil || w.Store.DB == nil || strings.TrimSpace(w.WSSURL) == "" {
		return
	}
	log.Printf("security radar SBX-1 WSS collector started provider=%s mode=%s network=%s enrichment_rpc=%t", SecurityRadarStreamProvider, SecurityRadarStreamModeLogs, w.Network, strings.TrimSpace(w.RPCURL) != "")
	go w.persistLoop(ctx)
	backoff := 3 * time.Second
	for {
		select {
		case <-ctx.Done():
			log.Printf("security radar SBX-1 WSS collector stopped")
			return
		default:
		}
		startedAt := time.Now()
		err := w.runOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(startedAt) >= 45*time.Second && !isRadarRateLimitError(err) {
			backoff = 3 * time.Second
		}
		if isRadarRateLimitError(err) && backoff < radarRateLimitBaseBackoff() {
			backoff = radarRateLimitBaseBackoff()
		}
		wait := radarReconnectWait(backoff)
		if err != nil {
			log.Printf("security radar SBX-1 WSS reconnect scheduled retry_in=%s err=%s", wait.Round(time.Second), safeProviderError(err))
		}
		maxBackoff := radarMaxReconnectBackoff()
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func radarRateLimitBaseBackoff() time.Duration {
	seconds := envIntRange("RADAR_WSS_429_BACKOFF_SECONDS", 90, 30, 600)
	return time.Duration(seconds) * time.Second
}

func radarMaxReconnectBackoff() time.Duration {
	seconds := envIntRange("RADAR_WSS_MAX_BACKOFF_SECONDS", 300, 60, 1800)
	return time.Duration(seconds) * time.Second
}

func envIntRange(key string, fallback, min, max int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < min || n > max {
		return fallback
	}
	return n
}

func isRadarRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "429") || strings.Contains(message, "too many requests") || strings.Contains(message, "rate limit")
}

func radarReconnectWait(base time.Duration) time.Duration {
	if base <= 0 {
		base = 3 * time.Second
	}
	var sample [2]byte
	if _, err := rand.Read(sample[:]); err != nil {
		return base
	}
	jitterMax := base / 5
	if jitterMax <= 0 {
		return base
	}
	fraction := float64(binary.BigEndian.Uint16(sample[:])) / 65535.0
	return base + time.Duration(fraction*float64(jitterMax))
}

func (w *SecurityRadarStreamWorker) persistLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-w.Queue:
			w.persistEvent(ctx, event)
		}
	}
}

func (w *SecurityRadarStreamWorker) runOnce(ctx context.Context) error {
	conn, err := dialMinimalWebSocket(ctx, w.WSSURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	for index, source := range arvisHeartbeatSources() {
		programID := strings.TrimSpace(source.ProgramID)
		if programID == "" {
			continue
		}
		subscription := map[string]any{"jsonrpc": "2.0", "id": index + 1, "method": "logsSubscribe", "params": []any{map[string]any{"mentions": []string{programID}}, map[string]any{"commitment": "confirmed"}}}
		if err := conn.WriteJSON(subscription); err != nil {
			return err
		}
	}
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ping.C:
			_ = conn.Ping()
		default:
		}
		payload, err := conn.ReadText(ctx)
		if err != nil {
			return err
		}
		event, ok := w.decodeLogsPayload(payload)
		if !ok {
			continue
		}
		select {
		case w.Queue <- event:
		default:
			log.Printf("security radar SBX-1 queue full; dropping signature=%s module=%s", event.Signature, event.ModuleID)
		}
	}
}

func (w *SecurityRadarStreamWorker) decodeLogsPayload(payload []byte) (SecurityRadarStreamEventRecord, bool) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return SecurityRadarStreamEventRecord{}, false
	}
	if !strings.Contains(anyString(raw["method"]), "logsNotification") {
		return SecurityRadarStreamEventRecord{}, false
	}
	params := asRadarMap(raw["params"])
	result := asRadarMap(params["result"])
	contextValue := asRadarMap(result["context"])
	value := asRadarMap(result["value"])
	logs := radarStringSlice(value["logs"])
	moduleID, eventType, programID := classifyRadarStreamText(strings.ToLower(strings.Join(logs, "\n")))
	if moduleID == "unknown" && !envBool("RADAR_STREAM_STORE_UNKNOWN") {
		return SecurityRadarStreamEventRecord{}, false
	}
	signature := anyString(value["signature"])
	target := signature
	return SecurityRadarStreamEventRecord{Provider: SecurityRadarStreamProvider, StreamMode: SecurityRadarStreamModeLogs, Network: w.Network, ModuleID: moduleID, EventType: eventType, Target: target, TargetType: targetTypeForRadarModule(moduleID), Signature: signature, Slot: radarInt64(contextValue["slot"]), ProgramID: programID, EvidenceQuality: evidenceQualityForRadarModule(moduleID), Decoded: map[string]any{"logs": logs, "err": value["err"], "subscription_method": "logsSubscribe"}, RawEvent: raw}, signature != "" || moduleID != "unknown"
}

func (w *SecurityRadarStreamWorker) persistEvent(ctx context.Context, event SecurityRadarStreamEventRecord) {
	event = w.enrichEventTarget(ctx, event)
	if _, err := w.Store.InsertStreamEvent(ctx, event); err != nil {
		log.Printf("security radar stream insert failed: %v", err)
		return
	}
	if event.ModuleID != ModulePumpSybilRadar && event.ModuleID != ModuleRaydiumPoolGuardian {
		return
	}
	if strings.TrimSpace(event.Target) == "" || strings.EqualFold(strings.TrimSpace(event.Target), strings.TrimSpace(event.Signature)) {
		return
	}
	bundle := AnalyzeSecurityRadars(SecurityRadarRequest{Target: event.Target, Network: event.Network, Mode: "sbx1_stream"})
	verdict := securityRadarModuleVerdict(bundle, event.ModuleID)
	signals := mergeRadarMaps(verdict.Signals, map[string]any{"stream_event_type": event.EventType, "stream_mode": event.StreamMode, "stream_provider": event.Provider, "stream_signature": event.Signature, "stream_evidence_quality": event.EvidenceQuality})
	if !shouldPublishSBX1CustomerVerdict(event, verdict, signals) {
		log.Printf("security radar SBX-1 raw-only event retained signature=%s target=%s data_quality=%s risk=%d", event.Signature, event.Target, anyString(signals["data_quality"]), verdict.RiskIndex)
		return
	}
	eventID, err := w.Store.InsertEvent(ctx, SecurityRadarEventRecord{ModuleID: verdict.ModuleID, Target: verdict.Target, TargetType: event.TargetType, Network: verdict.Network, Signature: firstRadarValue(event.Signature, verdict.Signature), SourceAddress: event.ProgramID, EventType: event.EventType, Slot: event.Slot, Signals: signals, RawSummary: event.Decoded, Source: "sbx1_stream"})
	if err != nil {
		log.Printf("security radar stream event-to-radar insert failed: %v", err)
		return
	}
	_, err = w.Store.InsertVerdict(ctx, SecurityRadarVerdictRecord{EventID: eventID, ModuleID: verdict.ModuleID, Target: verdict.Target, TargetType: event.TargetType, Network: verdict.Network, Grade: verdict.Grade, RiskIndex: verdict.RiskIndex, RiskLevel: verdict.RiskLevel, Verdict: verdict.Verdict, Recommendation: verdict.Recommendation, Evidence: append(verdict.Evidence, "SBX-1 WSS stream observed and enriched this target before manual dashboard analysis."), Signals: signals, RuleVersion: verdict.RuleVersion, Signed: verdict.Signed, Signature: verdict.Signature, Source: "sbx1_stream"})
	if err != nil {
		log.Printf("security radar stream verdict insert failed: %v", err)
	}
}

func shouldPublishSBX1CustomerVerdict(event SecurityRadarStreamEventRecord, verdict SecurityRadarVerdict, signals map[string]any) bool {
	if strings.TrimSpace(event.Target) == "" || strings.EqualFold(strings.TrimSpace(event.Target), strings.TrimSpace(event.Signature)) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(event.EvidenceQuality), "transaction_enriched_mint") {
		return true
	}
	if ok, _ := signals["real_onchain_evidence"].(bool); ok {
		if strings.EqualFold(anyString(signals["data_quality"]), "live_rpc_evidence") {
			if mint, _ := signals["is_token_mint"].(bool); mint {
				return true
			}
		}
		if verdict.RiskIndex >= 35 {
			return true
		}
	}
	return false
}

func (w *SecurityRadarStreamWorker) enrichEventTarget(ctx context.Context, event SecurityRadarStreamEventRecord) SecurityRadarStreamEventRecord {
	if strings.TrimSpace(w.RPCURL) == "" || strings.TrimSpace(event.Signature) == "" {
		return event
	}
	needsEnrichment := strings.TrimSpace(event.Target) == "" || strings.EqualFold(strings.TrimSpace(event.Target), strings.TrimSpace(event.Signature))
	if !needsEnrichment {
		return event
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	tx, err := SolanaGetTransactionJSONParsed(ctx, w.RPCURL, event.Signature)
	if err != nil {
		event.Decoded["enrichment_error"] = compactRadarError("getTransaction", err)
		return event
	}
	mints := extractMintsFromTransactionMap(map[string]any(tx))
	if len(mints) > 0 {
		mint := selectArvisTargetMint(mints)
		if mint != "" {
			event.Target = mint
			event.TargetType = "token"
			event.EvidenceQuality = "transaction_enriched_mint"
			event.Decoded["enriched_mint"] = mint
			event.Decoded["enriched_mints"] = mints
			event.Decoded["base_asset_mints_filtered"] = len(mints) > 1
		}
	}
	return event
}
