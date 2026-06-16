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
	SecurityRadarStreamModeBlock = "block_subscribe"
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
	Store       *SecurityRadarStore
	WSSURL      string
	Network     string
	Mode        string
	BufferSize  int
	Provider    string
	Programs    []string
	Reconnect   time.Duration
	workerQueue chan SecurityRadarStreamEventRecord
}

func StartSecurityRadarStreamIfEnabled(ctx context.Context, db *sql.DB) func() {
	if db == nil || !securityRadarStreamEnabled() {
		return func() {}
	}
	wssURL := firstSecurityRadarEnv("SOLANA_WSS_URL", "ALCHEMY_SOLANA_WSS_URL", "HELIUS_SOLANA_WSS_URL", "QUICKNODE_SOLANA_WSS_URL")
	if wssURL == "" {
		log.Printf("security radar stream not started: SOLANA_WSS_URL is empty")
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	worker := NewSecurityRadarStreamWorker(NewSecurityRadarStore(db), wssURL)
	go worker.Start(ctx)
	return cancel
}

func securityRadarStreamEnabled() bool {
	v := strings.TrimSpace(os.Getenv("RADAR_STREAM_ENABLED"))
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func NewSecurityRadarStreamWorker(store *SecurityRadarStore, wssURL string) *SecurityRadarStreamWorker {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("RADAR_STREAM_MODE")))
	if mode == "" {
		mode = SecurityRadarStreamModeLogs
	}
	if mode != SecurityRadarStreamModeBlock {
		mode = SecurityRadarStreamModeLogs
	}
	bufferSize := 5000
	if raw := strings.TrimSpace(os.Getenv("RADAR_EVENT_BUFFER_SIZE")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100000 {
			bufferSize = n
		}
	}
	reconnect := 3 * time.Second
	if raw := strings.TrimSpace(os.Getenv("RADAR_STREAM_RECONNECT_SECONDS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 120 {
			reconnect = time.Duration(n) * time.Second
		}
	}
	programs := splitCSV(os.Getenv("RADAR_STREAM_PROGRAMS"))
	if len(programs) == 0 {
		programs = []string{"pump", "raydium", "token", "spl"}
	}
	return &SecurityRadarStreamWorker{
		Store:       store,
		WSSURL:      strings.TrimSpace(wssURL),
		Network:     firstSecurityRadarString(os.Getenv("RADAR_STREAM_NETWORK"), "solana-mainnet"),
		Mode:        mode,
		BufferSize:  bufferSize,
		Provider:    SecurityRadarStreamProvider,
		Programs:    programs,
		Reconnect:   reconnect,
		workerQueue: make(chan SecurityRadarStreamEventRecord, bufferSize),
	}
}

func (w *SecurityRadarStreamWorker) Start(ctx context.Context) {
	if w == nil || w.Store == nil || w.Store.DB == nil || strings.TrimSpace(w.WSSURL) == "" {
		return
	}
	log.Printf("security radar SBX-1 stream starting provider=%s mode=%s network=%s buffer=%d", w.Provider, w.Mode, w.Network, w.BufferSize)
	go w.persistLoop(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Printf("security radar SBX-1 stream stopped")
			return
		default:
		}
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("security radar SBX-1 stream reconnecting after error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.Reconnect):
		}
	}
}

func (w *SecurityRadarStreamWorker) persistLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-w.workerQueue:
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
	if err := conn.WriteJSON(w.subscriptionPayload()); err != nil {
		return err
	}
	deadline := time.NewTicker(25 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			_ = conn.Ping()
		default:
		}
		payload, err := conn.ReadText(ctx)
		if err != nil {
			return err
		}
		events := w.decodeStreamPayload(payload)
		for _, event := range events {
			select {
			case w.workerQueue <- event:
			default:
				log.Printf("security radar SBX-1 stream queue full; dropping event signature=%s module=%s", event.Signature, event.ModuleID)
			}
		}
	}
}

func (w *SecurityRadarStreamWorker) subscriptionPayload() map[string]any {
	if w.Mode == SecurityRadarStreamModeBlock {
		return map[string]any{"jsonrpc": "2.0", "id": 1, "method": "blockSubscribe", "params": []any{"all", map[string]any{"commitment": "confirmed", "encoding": "jsonParsed", "transactionDetails": "full", "maxSupportedTransactionVersion": 0}}}
	}
	filter := strings.TrimSpace(os.Getenv("RADAR_STREAM_LOG_FILTER"))
	var filterValue any = "all"
	if strings.EqualFold(filter, "allWithVotes") {
		filterValue = "allWithVotes"
	}
	return map[string]any{"jsonrpc": "2.0", "id": 1, "method": "logsSubscribe", "params": []any{filterValue, map[string]any{"commitment": "confirmed"}}}
}

func (w *SecurityRadarStreamWorker) decodeStreamPayload(payload []byte) []SecurityRadarStreamEventRecord {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil
	}
	method := strings.TrimSpace(anyString(raw["method"]))
	if method == "" {
		return nil
	}
	if strings.Contains(method, "logsNotification") {
		if event, ok := w.decodeLogsNotification(raw); ok {
			return []SecurityRadarStreamEventRecord{event}
		}
		return nil
	}
	if strings.Contains(method, "blockNotification") {
		return w.decodeBlockNotification(raw)
	}
	return nil
}

func (w *SecurityRadarStreamWorker) decodeLogsNotification(raw map[string]any) (SecurityRadarStreamEventRecord, bool) {
	params := asMap(raw["params"])
	result := asMap(params["result"])
	ctx := asMap(result["context"])
	value := asMap(result["value"])
	logs := stringSlice(value["logs"])
	joined := strings.ToLower(strings.Join(logs, "\n"))
	signature := anyString(value["signature"])
	moduleID, eventType, programID := classifyStreamText(joined)
	if moduleID == "unknown" && !w.acceptUnknownEvents() {
		return SecurityRadarStreamEventRecord{}, false
	}
	decoded := map[string]any{"logs": logs, "err": value["err"], "subscription_method": "logsSubscribe"}
	return SecurityRadarStreamEventRecord{
		Provider:        w.Provider,
		StreamMode:      SecurityRadarStreamModeLogs,
		Network:         w.Network,
		ModuleID:        moduleID,
		EventType:       eventType,
		Target:          firstSecurityRadarString(extractMintFromLogs(logs), signature),
		TargetType:      targetTypeForModule(moduleID),
		Signature:       signature,
		Slot:            int64FromAny(ctx["slot"]),
		ProgramID:       programID,
		EvidenceQuality: evidenceQualityForModule(moduleID),
		Decoded:         decoded,
		RawEvent:        raw,
	}, signature != "" || moduleID != "unknown"
}

func (w *SecurityRadarStreamWorker) decodeBlockNotification(raw map[string]any) []SecurityRadarStreamEventRecord {
	params := asMap(raw["params"])
	result := asMap(params["result"])
	ctx := asMap(result["context"])
	value := asMap(result["value"])
	block := asMap(value["block"])
	transactions, _ := block["transactions"].([]any)
	out := []SecurityRadarStreamEventRecord{}
	for _, item := range transactions {
		txWrap := asMap(item)
		signature := firstSignature(txWrap["transaction"])
		message := transactionMessage(txWrap["transaction"])
		instructions, _ := message["instructions"].([]any)
		for _, ixRaw := range instructions {
			ix := asMap(ixRaw)
			programID := firstSecurityRadarString(anyString(ix["programId"]), anyString(ix["program"]))
			parsed := asMap(ix["parsed"])
			ixType := strings.ToLower(anyString(parsed["type"]))
			info := asMap(parsed["info"])
			text := strings.ToLower(programID + " " + anyString(ix["program"]) + " " + ixType + " " + anyString(info["mint"]) + " " + anyString(info["account"]))
			moduleID, eventType, classifiedProgram := classifyStreamText(text)
			if classifiedProgram != "" && programID == "" {
				programID = classifiedProgram
			}
			if moduleID == "unknown" && !w.acceptUnknownEvents() {
				continue
			}
			target := firstSecurityRadarString(anyString(info["mint"]), anyString(info["account"]), signature)
			out = append(out, SecurityRadarStreamEventRecord{Provider: w.Provider, StreamMode: SecurityRadarStreamModeBlock, Network: w.Network, ModuleID: moduleID, EventType: firstSecurityRadarString(eventType, ixType, "block_instruction"), Target: target, TargetType: targetTypeForModule(moduleID), Signature: signature, Slot: int64FromAny(ctx["slot"]), ProgramID: programID, EvidenceQuality: evidenceQualityForModule(moduleID), Decoded: map[string]any{"instruction": ix, "parsed_type": ixType, "subscription_method": "blockSubscribe"}, RawEvent: raw})
		}
	}
	return out
}

func (w *SecurityRadarStreamWorker) acceptUnknownEvents() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("RADAR_STREAM_STORE_UNKNOWN")), "true")
}

func (w *SecurityRadarStreamWorker) persistEvent(ctx context.Context, event SecurityRadarStreamEventRecord) {
	if event.ModuleID == "" {
		event.ModuleID = "unknown"
	}
	if event.EventType == "" {
		event.EventType = "stream_event"
	}
	if _, err := w.Store.InsertStreamEvent(ctx, event); err != nil {
		log.Printf("security radar stream event insert failed: %v", err)
		return
	}
	if event.ModuleID != ModulePumpSybilRadar && event.ModuleID != ModuleRaydiumPoolGuardian {
		return
	}
	if strings.TrimSpace(event.Target) == "" {
		return
	}
	bundle := AnalyzeSecurityRadars(SecurityRadarRequest{Target: event.Target, Network: event.Network, Mode: "sbx1_stream"})
	verdict := securityRadarModuleVerdict(bundle, event.ModuleID)
	eventID, err := w.Store.InsertEvent(ctx, SecurityRadarEventRecord{ModuleID: verdict.ModuleID, Target: verdict.Target, TargetType: event.TargetType, Network: verdict.Network, Signature: firstSecurityRadarString(event.Signature, verdict.Signature), SourceAddress: event.ProgramID, EventType: event.EventType, Slot: event.Slot, Signals: mergeRadarMaps(verdict.Signals, map[string]any{"stream_event_type": event.EventType, "stream_mode": event.StreamMode, "stream_provider": event.Provider, "stream_signature": event.Signature, "stream_evidence_quality": event.EvidenceQuality}), RawSummary: event.Decoded, Source: "sbx1_stream"})
	if err != nil {
		log.Printf("security radar stream event-to-radar insert failed: %v", err)
		return
	}
	_, err = w.Store.InsertVerdict(ctx, SecurityRadarVerdictRecord{EventID: eventID, ModuleID: verdict.ModuleID, Target: verdict.Target, TargetType: event.TargetType, Network: verdict.Network, Grade: verdict.Grade, RiskIndex: verdict.RiskIndex, RiskLevel: verdict.RiskLevel, Verdict: verdict.Verdict, Recommendation: verdict.Recommendation, Evidence: append(verdict.Evidence, "SBX-1 stream observed this target before manual dashboard analysis."), Signals: mergeRadarMaps(verdict.Signals, map[string]any{"stream_event_type": event.EventType, "stream_mode": event.StreamMode, "stream_signature": event.Signature, "stream_evidence_quality": event.EvidenceQuality}), RuleVersion: verdict.RuleVersion, Signed: verdict.Signed, Signature: verdict.Signature, Source: "sbx1_stream"})
	if err != nil {
		log.Printf("security radar stream verdict insert failed: %v", err)
	}
}

func (s *SecurityRadarStore) InsertStreamEvent(ctx context.Context, event SecurityRadarStreamEventRecord) (string, error) {
	if s == nil || s.DB == nil {
		return "", nil
	}
	decoded, _ := json.Marshal(nonNilMap(event.Decoded))
	rawEvent, _ := json.Marshal(nonNilMap(event.RawEvent))
	var id string
	err := s.DB.QueryRowContext(ctx, `
		INSERT INTO security_radar_stream_events (provider,stream_mode,network,module_id,event_type,target,target_type,signature,slot,program_id,evidence_quality,decoded,raw_event,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7,NULLIF($8,''),NULLIF($9,0),NULLIF($10,''),$11,$12::jsonb,$13::jsonb,now(),now())
		ON CONFLICT DO NOTHING
		RETURNING id::text`, firstSecurityRadarString(event.Provider, SecurityRadarStreamProvider), firstSecurityRadarString(event.StreamMode, SecurityRadarStreamModeLogs), normalizeRadarNetwork(event.Network), firstSecurityRadarString(event.ModuleID, "unknown"), firstSecurityRadarString(event.EventType, "stream_event"), strings.TrimSpace(event.Target), firstSecurityRadarString(event.TargetType, "unknown"), strings.TrimSpace(event.Signature), event.Slot, strings.TrimSpace(event.ProgramID), firstSecurityRadarString(event.EvidenceQuality, "raw_stream"), string(decoded), string(rawEvent)).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func classifyStreamText(text string) (moduleID, eventType, programID string) {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "pump") || strings.Contains(lower, "pump.fun") || strings.Contains(lower, "pumpswap"):
		return ModulePumpSybilRadar, "pump_launch_or_trade", "pump"
	case strings.Contains(lower, "raydium") || strings.Contains(lower, "initialize2") || strings.Contains(lower, "amm"):
		return ModuleRaydiumPoolGuardian, "raydium_pool_or_liquidity", "raydium"
	case strings.Contains(lower, "initializemint") || strings.Contains(lower, "initialize mint") || strings.Contains(lower, "mintto") || strings.Contains(lower, "spl-token") || strings.Contains(lower, "token program"):
		return ModulePumpSybilRadar, "spl_token_mint_activity", "spl_token"
	default:
		return "unknown", "stream_event", ""
	}
}

func extractMintFromLogs(logs []string) string {
	for _, line := range logs {
		fields := strings.Fields(line)
		for _, f := range fields {
			candidate := strings.Trim(f, " ,.;:()[]{}<>\"'")
			if isLikelySolanaAddress(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func targetTypeForModule(moduleID string) string {
	switch moduleID {
	case ModuleRaydiumPoolGuardian:
		return "pool_or_token"
	case ModulePumpSybilRadar:
		return "token_or_launch"
	default:
		return "unknown"
	}
}

func evidenceQualityForModule(moduleID string) string {
	if moduleID == "unknown" {
		return "raw_stream"
	}
	return "decoded_stream_hint"
}

func mergeRadarMaps(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range nonNilMap(a) {
		out[k] = v
	}
	for k, v := range nonNilMap(b) {
		out[k] = v
	}
	return out
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		out = append(out, anyString(item))
	}
	return out
}

func int64FromAny(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64(); return n
	case string:
		n, _ := strconv.ParseInt(x, 10, 64); return n
	default:
		return 0
	}
}

func firstSignature(tx any) string {
	m := asMap(tx)
	arr, _ := m["signatures"].([]any)
	if len(arr) == 0 {
		return ""
	}
	return anyString(arr[0])
}

func transactionMessage(tx any) map[string]any {
	m := asMap(tx)
	if transaction := asMap(m["transaction"]); len(transaction) > 0 {
		return asMap(transaction["message"])
	}
	return asMap(m["message"])
}

func isLikelySolanaAddress(value string) bool {
	if len(value) < 32 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", r) {
			return false
		}
	}
	return true
}

type minimalWSConn struct {
	conn net.Conn
	r    *bufio.Reader
}

func dialMinimalWebSocket(ctx context.Context, rawURL string) (*minimalWSConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("unsupported websocket scheme %s", u.Scheme)
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" { host += ":443" } else { host += ":80" }
	}
	d := net.Dialer{Timeout: 12 * time.Second}
	var conn net.Conn
	if u.Scheme == "wss" {
		raw, err := d.DialContext(ctx, "tcp", host)
		if err != nil { return nil, err }
		tlsConn := tls.Client(raw, &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil { _ = raw.Close(); return nil, err }
		conn = tlsConn
	} else {
		conn, err = d.DialContext(ctx, "tcp", host)
		if err != nil { return nil, err }
	}
	keyRaw := make([]byte, 16)
	_, _ = rand.Read(keyRaw)
	key := base64.StdEncoding.EncodeToString(keyRaw)
	path := u.RequestURI()
	if path == "" { path = "/" }
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nUser-Agent: Koschei-SBX1/1.0\r\n\r\n", path, u.Host, key)
	if _, err := io.WriteString(conn, req); err != nil { _ = conn.Close(); return nil, err }
	r := bufio.NewReader(conn)
	status, err := r.ReadString('\n')
	if err != nil { _ = conn.Close(); return nil, err }
	if !strings.Contains(status, " 101 ") {
		_ = conn.Close(); return nil, fmt.Errorf("websocket upgrade failed: %s", strings.TrimSpace(status))
	}
	accept := ""
	for {
		line, err := r.ReadString('\n')
		if err != nil { _ = conn.Close(); return nil, err }
		line = strings.TrimSpace(line)
		if line == "" { break }
		if strings.HasPrefix(strings.ToLower(line), "sec-websocket-accept:") {
			accept = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	if accept != websocketAccept(key) { _ = conn.Close(); return nil, fmt.Errorf("websocket accept mismatch") }
	return &minimalWSConn{conn: conn, r: r}, nil
}

func websocketAccept(key string) string {
	h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h[:])
}

func (c *minimalWSConn) Close() error { if c == nil || c.conn == nil { return nil }; return c.conn.Close() }

func (c *minimalWSConn) WriteJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil { return err }
	return c.writeFrame(0x1, payload)
}

func (c *minimalWSConn) Ping() error { return c.writeFrame(0x9, []byte("koschei")) }

func (c *minimalWSConn) pong(payload []byte) error { return c.writeFrame(0xA, payload) }

func (c *minimalWSConn) writeFrame(opcode byte, payload []byte) error {
	if c == nil || c.conn == nil { return io.ErrClosedPipe }
	var b bytes.Buffer
	b.WriteByte(0x80 | opcode)
	maskKey := make([]byte, 4)
	_, _ = rand.Read(maskKey)
	l := len(payload)
	switch {
	case l < 126:
		b.WriteByte(0x80 | byte(l))
	case l <= 65535:
		b.WriteByte(0x80 | 126)
		_ = binary.Write(&b, binary.BigEndian, uint16(l))
	default:
		b.WriteByte(0x80 | 127)
		_ = binary.Write(&b, binary.BigEndian, uint64(l))
	}
	b.Write(maskKey)
	masked := make([]byte, l)
	for i := range payload { masked[i] = payload[i] ^ maskKey[i%4] }
	b.Write(masked)
	_, err := c.conn.Write(b.Bytes())
	return err
}

func (c *minimalWSConn) ReadText(ctx context.Context) ([]byte, error) {
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(35 * time.Second))
		h, err := c.r.ReadByte()
		if err != nil { return nil, err }
		h2, err := c.r.ReadByte()
		if err != nil { return nil, err }
		opcode := h & 0x0f
		masked := h2&0x80 != 0
		length := uint64(h2 & 0x7f)
		if length == 126 {
			var x uint16; if err := binary.Read(c.r, binary.BigEndian, &x); err != nil { return nil, err }; length = uint64(x)
		} else if length == 127 {
			if err := binary.Read(c.r, binary.BigEndian, &length); err != nil { return nil, err }
		}
		mask := []byte{0,0,0,0}
		if masked { if _, err := io.ReadFull(c.r, mask); err != nil { return nil, err } }
		payload := make([]byte, length)
		if _, err := io.ReadFull(c.r, payload); err != nil { return nil, err }
		if masked { for i := range payload { payload[i] ^= mask[i%4] } }
		switch opcode {
		case 0x1:
			return payload, nil
		case 0x8:
			return nil, io.EOF
		case 0x9:
			_ = c.pong(payload)
		case 0xA:
			continue
		default:
			continue
		}
		select { case <-ctx.Done(): return nil, ctx.Err(); default: }
	}
}
