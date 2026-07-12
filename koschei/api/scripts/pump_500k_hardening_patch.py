from pathlib import Path


def replace_once(text: str, old: str, new: str, label: str) -> str:
    if old not in text:
        raise SystemExit(f"missing replacement target: {label}")
    return text.replace(old, new, 1)

# Quota-safe attempt cooling, one deep report per cycle, explicit queue state.
path = Path("internal/services/pump_high_volume_radar.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tdefaultPumpHighVolumeCooldown     = 6 * time.Hour
\tdefaultPumpHighVolumePageSize     = 900
\tdefaultPumpHighVolumeMaxReports   = 3''',
    '''\tdefaultPumpHighVolumeCooldown     = 6 * time.Hour
\tdefaultPumpHighVolumeAttemptPause = 30 * time.Minute
\tdefaultPumpHighVolumePageSize     = 900
\tdefaultPumpHighVolumeMaxReports   = 1''',
    "quota constants",
)
text = replace_once(
    text,
    '''func pumpHighVolumePageSize() int {''',
    '''func pumpHighVolumeAttemptCooldown() time.Duration {
\tif raw := strings.TrimSpace(os.Getenv("PUMP_HIGH_VOLUME_ATTEMPT_COOLDOWN_SECONDS")); raw != "" {
\t\tif value, err := strconv.Atoi(raw); err == nil && value >= 300 && value <= 21600 {
\t\t\treturn time.Duration(value) * time.Second
\t\t}
\t}
\treturn defaultPumpHighVolumeAttemptPause
}

func pumpHighVolumePageSize() int {''',
    "attempt cooldown env",
)
text = replace_once(
    text,
    '''\tReportCooldown     time.Duration
\tCandidatePageSize  int''',
    '''\tReportCooldown     time.Duration
\tAttemptCooldown    time.Duration
\tCandidatePageSize  int''',
    "attempt cooldown field",
)
text = replace_once(
    text,
    '''\t\tReportCooldown: pumpHighVolumeReportCooldown(), CandidatePageSize: pumpHighVolumePageSize(),
\t\tMaxReportsPerCycle: pumpHighVolumeMaxReportsPerCycle(),''',
    '''\t\tReportCooldown: pumpHighVolumeReportCooldown(), AttemptCooldown: pumpHighVolumeAttemptCooldown(),
\t\tCandidatePageSize: pumpHighVolumePageSize(), MaxReportsPerCycle: pumpHighVolumeMaxReportsPerCycle(),''',
    "attempt cooldown constructor",
)
old_loop = '''\tfor _, item := range qualified {
\t\teventID, eventErr := w.Store.RecordPumpHighVolumeObservation(ctx, item.Candidate, item.Market, w.ThresholdUSD)
\t\tif eventErr != nil {
\t\t\tlog.Printf("pump high-volume event store failed mint=%s: %v", item.Candidate.Mint, eventErr)
\t\t\tcontinue
\t\t}
\t\trecent, recentErr := w.Store.PumpHighVolumeReportedRecently(ctx, item.Candidate.Mint, w.ReportCooldown)
\t\tif recentErr != nil {
\t\t\tlog.Printf("pump high-volume cooldown lookup failed mint=%s: %v", item.Candidate.Mint, recentErr)
\t\t\tcontinue
\t\t}
\t\tif recent || reports >= w.MaxReportsPerCycle {
\t\t\tcontinue
\t\t}
\t\tif err := w.scanAndStore(ctx, eventID, item.Candidate, item.Market); err != nil {
\t\t\tlog.Printf("pump high-volume ARVIS report failed mint=%s volume24h=%.2f: %v", item.Candidate.Mint, item.Market.Volume24hUSD, err)
\t\t\tcontinue
\t\t}
\t\treports++
\t}'''
new_loop = '''\tfor _, item := range qualified {
\t\tattempted, attemptErr := w.Store.PumpHighVolumeAttemptedRecently(ctx, item.Candidate.Mint, w.AttemptCooldown)
\t\tif attemptErr != nil {
\t\t\tlog.Printf("pump high-volume attempt cooldown lookup failed mint=%s: %v", item.Candidate.Mint, attemptErr)
\t\t\tcontinue
\t\t}
\t\teventID, eventErr := w.Store.RecordPumpHighVolumeObservation(ctx, item.Candidate, item.Market, w.ThresholdUSD)
\t\tif eventErr != nil {
\t\t\tlog.Printf("pump high-volume event store failed mint=%s: %v", item.Candidate.Mint, eventErr)
\t\t\tcontinue
\t\t}
\t\trecent, recentErr := w.Store.PumpHighVolumeReportedRecently(ctx, item.Candidate.Mint, w.ReportCooldown)
\t\tif recentErr != nil {
\t\t\tlog.Printf("pump high-volume cooldown lookup failed mint=%s: %v", item.Candidate.Mint, recentErr)
\t\t\tcontinue
\t\t}
\t\tif recent || attempted || reports >= w.MaxReportsPerCycle {
\t\t\tcontinue
\t\t}
\t\tif markErr := w.Store.MarkPumpHighVolumeAttempted(ctx, eventID); markErr != nil {
\t\t\tlog.Printf("pump high-volume attempt marker failed mint=%s: %v", item.Candidate.Mint, markErr)
\t\t\tcontinue
\t\t}
\t\tif err := w.scanAndStore(ctx, eventID, item.Candidate, item.Market); err != nil {
\t\t\tlog.Printf("pump high-volume ARVIS report failed mint=%s volume24h=%.2f: %v", item.Candidate.Mint, item.Market.Volume24hUSD, err)
\t\t\tcontinue
\t\t}
\t\treports++
\t}'''
text = replace_once(text, old_loop, new_loop, "quota safe scan loop")
text = replace_once(
    text,
    '''func (s *SecurityRadarStore) LatestPumpHighVolumeReports(ctx context.Context, limit int) ([]PumpHighVolumeOwnerItem, error) {''',
    '''func (s *SecurityRadarStore) PumpHighVolumeAttemptedRecently(ctx context.Context, mint string, cooldown time.Duration) (bool, error) {
\tif s == nil || s.DB == nil {
\t\treturn false, nil
\t}
\tif cooldown <= 0 {
\t\tcooldown = defaultPumpHighVolumeAttemptPause
\t}
\tvar exists bool
\terr := s.DB.QueryRowContext(ctx, `
\t\tSELECT EXISTS (
\t\t\tSELECT 1 FROM security_radar_events
\t\t\tWHERE event_type=$1 AND source=$2 AND lower(target)=lower($3)
\t\t\t  AND COALESCE(signals->>'auto_scan_attempted','false')='true'
\t\t\t  AND created_at >= now()-($4 * interval '1 second')
\t\t)`, pumpHighVolumeEventType, pumpHighVolumeSource, strings.TrimSpace(mint), int64(cooldown/time.Second)).Scan(&exists)
\treturn exists, err
}

func (s *SecurityRadarStore) MarkPumpHighVolumeAttempted(ctx context.Context, eventID string) error {
\tif s == nil || s.DB == nil || strings.TrimSpace(eventID) == "" {
\t\treturn nil
\t}
\t_, err := s.DB.ExecContext(ctx, `
\t\tUPDATE security_radar_events
\t\tSET signals=jsonb_set(COALESCE(signals,'{}'::jsonb),'{auto_scan_attempted}','true'::jsonb,true), updated_at=now()
\t\tWHERE id=$1::uuid`, strings.TrimSpace(eventID))
\treturn err
}

func (s *SecurityRadarStore) LatestPumpHighVolumeReports(ctx context.Context, limit int) ([]PumpHighVolumeOwnerItem, error) {''',
    "attempt store methods",
)
text = replace_once(
    text,
    '''\tif limit <= 0 || limit > 200 {
\t\tlimit = 100
\t}''',
    '''\tif limit <= 0 || limit > 200 {
\t\tlimit = 200
\t}''',
    "owner report limit",
)
text = replace_once(
    text,
    '''\t\titem.ReportStatus = "evidence_pending"
\t\tif risk.Valid {''',
    '''\t\titem.ReportStatus = "queued"
\t\tif pumpSignalBool(item.Signals, "auto_scan_attempted") {
\t\t\titem.ReportStatus = "evidence_pending"
\t\t}
\t\tif risk.Valid {''',
    "owner queue status",
)
text = replace_once(
    text,
    '''\t\t"auto_volume_gate": true, "volume_window": "24h", "volume_currency": "USD",''',
    '''\t\t"auto_volume_gate": true, "auto_scan_attempted": false, "volume_window": "24h", "volume_currency": "USD",''',
    "initial attempt signal",
)
text += '''

func pumpSignalBool(signals map[string]any, key string) bool {
\tswitch value := signals[key].(type) {
\tcase bool:
\t\treturn value
\tcase string:
\t\tparsed, _ := strconv.ParseBool(strings.TrimSpace(value))
\t\treturn parsed
\tdefault:
\t\treturn false
\t}
}
'''
path.write_text(text)

# Automatic high-volume scans need the same bounded full-report timeout as owner/manual scans.
path = Path("internal/services/security_radars.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tif strings.Contains(strings.ToLower(req.Mode), "owner") || strings.Contains(strings.ToLower(req.Mode), "manual") {
\t\ttimeout = 18 * time.Second
\t}''',
    '''\tmodeLower := strings.ToLower(req.Mode)
\tif strings.Contains(modeLower, "owner") || strings.Contains(modeLower, "manual") || strings.Contains(modeLower, "live_stream:"+ModulePumpSybilRadar) {
\t\ttimeout = 18 * time.Second
\t}''',
    "automatic full scan timeout",
)
path.write_text(text)

# Automatic owner-only discoveries must not surface in the customer feed.
path = Path("internal/handlers/security_radar.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tfor _, item := range items {
\t\tif item.Signed && radarSignalsVerified(item.Signals) && item.ModuleID == services.ModuleFinalVerdictEngine {
\t\t\tverified = append(verified, item)
\t\t}
\t}''',
    '''\tfor _, item := range items {
\t\tif visible, exists := item.Signals["customer_detail_visible"]; exists {
\t\t\tif allowed, ok := visible.(bool); ok && !allowed {
\t\t\t\tcontinue
\t\t\t}
\t\t}
\t\tif item.Signed && radarSignalsVerified(item.Signals) && item.ModuleID == services.ModuleFinalVerdictEngine {
\t\t\tverified = append(verified, item)
\t\t}
\t}''',
    "owner-only customer feed filter",
)
path.write_text(text)

# Do not let an old volume-gate event permanently outrank newer source context.
path = Path("internal/handlers/security_radar_detail.go")
text = path.read_text()
text = replace_once(
    text,
    "ORDER BY CASE WHEN event_type='pumpportal_high_volume_24h' THEN 0 WHEN source='pumpportal' THEN 1 ELSE 2 END, created_at DESC",
    "ORDER BY CASE WHEN event_type='pumpportal_high_volume_24h' AND created_at >= now()-interval '36 hours' THEN 0 WHEN source='pumpportal' THEN 1 ELSE 2 END, created_at DESC",
    "fresh volume source priority",
)
path.write_text(text)

# Return the full owner-side qualified set allowed by the endpoint contract.
path = Path("internal/handlers/owner_operations.go")
text = path.read_text()
text = replace_once(text, "store.LatestPumpHighVolumeReports(r.Context(), 100)", "store.LatestPumpHighVolumeReports(r.Context(), 200)", "owner qualified limit")
path.write_text(text)

# Configuration defaults aligned with the production RPC budget.
path = Path("../../.env.example")
text = path.read_text()
text = replace_once(
    text,
    '''PUMP_HIGH_VOLUME_REPORT_COOLDOWN_SECONDS=21600
PUMP_HIGH_VOLUME_CANDIDATE_PAGE_SIZE=900
PUMP_HIGH_VOLUME_MAX_REPORTS_PER_CYCLE=3''',
    '''PUMP_HIGH_VOLUME_REPORT_COOLDOWN_SECONDS=21600
PUMP_HIGH_VOLUME_ATTEMPT_COOLDOWN_SECONDS=1800
PUMP_HIGH_VOLUME_CANDIDATE_PAGE_SIZE=900
PUMP_HIGH_VOLUME_MAX_REPORTS_PER_CYCLE=1''',
    "quota safe env defaults",
)
path.write_text(text)

# Expand focused tests for queue and quota defaults.
path = Path("internal/services/pump_high_volume_radar_test.go")
text = path.read_text()
text = replace_once(
    text,
    '''\tif signals["auto_volume_gate"] != true || signals["source_verified_pump_event"] != true {
\t\tt.Fatalf("missing gate evidence: %#v", signals)
\t}''',
    '''\tif signals["auto_volume_gate"] != true || signals["source_verified_pump_event"] != true {
\t\tt.Fatalf("missing gate evidence: %#v", signals)
\t}
\tif pumpSignalBool(signals, "auto_scan_attempted") {
\t\tt.Fatalf("a qualified observation must begin queued, not attempted: %#v", signals)
\t}''',
    "queued signal test",
)
text += '''

func TestPumpHighVolumeQuotaDefaults(t *testing.T) {
\tt.Setenv("PUMP_HIGH_VOLUME_MAX_REPORTS_PER_CYCLE", "")
\tt.Setenv("PUMP_HIGH_VOLUME_ATTEMPT_COOLDOWN_SECONDS", "")
\tif got := pumpHighVolumeMaxReportsPerCycle(); got != 1 {
\t\tt.Fatalf("max reports per cycle = %d", got)
\t}
\tif got := pumpHighVolumeAttemptCooldown(); got != 30*time.Minute {
\t\tt.Fatalf("attempt cooldown = %s", got)
\t}
}
'''
path.write_text(text)
