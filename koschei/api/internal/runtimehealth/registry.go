package runtimehealth

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = "koschei.runtime-health.v1"

const (
	CoverageNotApplicable = "not_applicable"
	CoverageInactive      = "inactive"
	CoverageUnknown       = "unknown"
	CoverageCurrent       = "current"
	CoverageLagging       = "lagging"
	CoverageBlindSpot     = "blind_spot"
	CoverageReorgGuard    = "reorg_guard"
)

type State string

const (
	StateDisabled    State = "disabled"
	StateConfigured  State = "configured"
	StateLive        State = "live"
	StateDegraded    State = "degraded"
	StateUnavailable State = "unavailable"
	StateStopped     State = "stopped"
)

type Entry struct {
	ID                  string          `json:"id"`
	Kind                string          `json:"kind"`
	NetworkID           string          `json:"network_id,omitempty"`
	State               State           `json:"state"`
	Configured          bool            `json:"configured"`
	SuccessfulCycles    uint64          `json:"successful_cycles"`
	FailedCycles        uint64          `json:"failed_cycles"`
	Observations        uint64          `json:"observations"`
	ConsecutiveFailures uint64          `json:"consecutive_failures"`
	LastSuccessAt       *time.Time      `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time      `json:"last_failure_at,omitempty"`
	LastError           string          `json:"last_error,omitempty"`
	UpdatedAt           time.Time       `json:"updated_at"`
	Freshness           string          `json:"freshness"`
	MaxAgeSeconds       float64         `json:"max_age_seconds,omitempty"`
	FreshUntil          *time.Time      `json:"fresh_until,omitempty"`
	Ingest              *IngestProgress `json:"ingest,omitempty"`
	CoverageStatus      string          `json:"coverage_status,omitempty"`
	CoverageReason      string          `json:"coverage_reason,omitempty"`
	CoverageAttention   bool            `json:"coverage_attention_required,omitempty"`
	registeredAt        time.Time
	maxAge              time.Duration
}

// Heights are decimal strings in JSON to preserve uint64 precision in browsers.
// Pending blocks describe cursor distance, not proven missing historical data.
type IngestProgress struct {
	ObservedHead  *uint64   `json:"observed_head,omitempty,string"`
	DurableCursor *uint64   `json:"durable_cursor,omitempty,string"`
	PendingBlocks *uint64   `json:"pending_blocks,omitempty,string"`
	Status        string    `json:"status"`
	CheckedAt     time.Time `json:"checked_at"`
}

func (r *Registry) RecordIngestProgress(id string, head, cursor *uint64, failed, reorg bool) {
	if r == nil || strings.TrimSpace(id) == "" {
		return
	}
	p := IngestProgress{ObservedHead: copyHeight(head), DurableCursor: copyHeight(cursor), Status: "unknown", CheckedAt: r.now().UTC()}
	if head != nil && cursor != nil {
		if *head < *cursor {
			p.Status = "provider_behind_cursor"
		} else {
			pending := *head - *cursor
			p.PendingBlocks = &pending
			p.Status = "at_observed_head"
			if pending > 0 {
				p.Status = "catching_up"
			}
		}
	}
	if failed && p.Status != "provider_behind_cursor" {
		p.Status = "cycle_failed"
	}
	if reorg {
		p.Status = "reorg_recheck_required"
		p.PendingBlocks = nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id = strings.TrimSpace(id)
	e := r.entries[id]
	e.ID = id
	e.Ingest = &p
	r.entries[id] = e
}

func copyHeight(v *uint64) *uint64 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

type Snapshot struct {
	SchemaVersion  string         `json:"schema_version"`
	GeneratedAt    time.Time      `json:"generated_at"`
	Entries        []Entry        `json:"entries"`
	Counts         map[State]int  `json:"counts"`
	CoverageCounts map[string]int `json:"coverage_counts"`
}

type Registry struct {
	mu      sync.RWMutex
	entries map[string]Entry
	now     func() time.Time
}

func New() *Registry {
	return &Registry{entries: map[string]Entry{}, now: func() time.Time { return time.Now().UTC() }}
}

func (r *Registry) Register(id, kind, networkID string, configured bool) {
	r.register(id, kind, networkID, configured, 0)
}

// RegisterPeriodic opts a recurring component into freshness monitoring.
// Re-registering a component never refreshes its successful-cycle deadline.
// Components without a periodic check remain explicitly not_monitored.
func (r *Registry) RegisterPeriodic(id, kind, networkID string, configured bool, maxAge time.Duration) {
	r.register(id, kind, networkID, configured, maxAge)
}

func (r *Registry) register(id, kind, networkID string, configured bool, maxAge time.Duration) {
	if r == nil {
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.entries[id]
	if entry.registeredAt.IsZero() || (configured && (!entry.Configured || entry.State == StateStopped)) {
		entry.registeredAt = now
	}
	if maxAge > 0 {
		entry.maxAge = maxAge
	}
	entry.ID = id
	entry.Kind = strings.TrimSpace(kind)
	entry.NetworkID = strings.ToLower(strings.TrimSpace(networkID))
	entry.Configured = configured
	if !configured {
		entry.State = StateDisabled
	} else if entry.State == "" || entry.State == StateDisabled || entry.State == StateStopped {
		entry.State = StateConfigured
	}
	entry.UpdatedAt = now
	r.entries[id] = entry
}

func (r *Registry) Success(id string, observations int) {
	if r == nil {
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.entries[id]
	entry.ID = id
	entry.Configured = true
	entry.State = StateLive
	entry.SuccessfulCycles++
	if observations > 0 {
		entry.Observations += uint64(observations)
	}
	entry.ConsecutiveFailures = 0
	entry.LastError = ""
	entry.LastSuccessAt = timePtr(now)
	entry.UpdatedAt = now
	r.entries[id] = entry
}

func (r *Registry) Failure(id string, err error) {
	if r == nil {
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.entries[id]
	entry.ID = id
	entry.Configured = true
	entry.FailedCycles++
	entry.ConsecutiveFailures++
	entry.LastFailureAt = timePtr(now)
	entry.LastError = ""
	if err != nil {
		entry.LastError = strings.TrimSpace(err.Error())
		if len(entry.LastError) > 512 {
			entry.LastError = entry.LastError[:512]
		}
	}
	if entry.SuccessfulCycles > 0 {
		entry.State = StateDegraded
	} else {
		entry.State = StateUnavailable
	}
	entry.UpdatedAt = now
	r.entries[id] = entry
}

func (r *Registry) Stop(id string) {
	if r == nil {
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.entries[id]
	entry.ID = id
	if entry.Configured {
		entry.State = StateStopped
	}
	entry.UpdatedAt = now
	r.entries[id] = entry
}

func (r *Registry) Snapshot() Snapshot {
	now := time.Now().UTC()
	if r == nil {
		return Snapshot{SchemaVersion: SchemaVersion, GeneratedAt: now, Entries: []Entry{}, Counts: map[State]int{}, CoverageCounts: map[string]int{}}
	}
	now = r.now().UTC()
	r.mu.RLock()
	entries := make([]Entry, 0, len(r.entries))
	for _, entry := range r.entries {
		if entry.Ingest != nil {
			p := *entry.Ingest
			p.ObservedHead = copyHeight(p.ObservedHead)
			p.DurableCursor = copyHeight(p.DurableCursor)
			p.PendingBlocks = copyHeight(p.PendingBlocks)
			entry.Ingest = &p
		}
		entry.Freshness = "not_monitored"
		if !entry.Configured || entry.State == StateStopped {
			entry.Freshness = "inactive"
		} else if entry.maxAge > 0 {
			entry.MaxAgeSeconds = entry.maxAge.Seconds()
			baseline := entry.registeredAt
			entry.Freshness = "awaiting_success"
			if entry.LastSuccessAt != nil && !entry.LastSuccessAt.Before(baseline) {
				baseline = *entry.LastSuccessAt
				entry.Freshness = "fresh"
			}
			entry.FreshUntil = timePtr(baseline.Add(entry.maxAge))
			if !now.Before(*entry.FreshUntil) {
				entry.Freshness = "stale"
				if entry.State == StateLive || entry.State == StateConfigured {
					entry.State = StateDegraded
				}
			}
		}
		entry.CoverageStatus, entry.CoverageReason, entry.CoverageAttention = assessCoverage(entry)
		// Snapshots own their timestamps; callers cannot mutate registry state.
		if entry.LastSuccessAt != nil {
			entry.LastSuccessAt = timePtr(*entry.LastSuccessAt)
		}
		if entry.LastFailureAt != nil {
			entry.LastFailureAt = timePtr(*entry.LastFailureAt)
		}
		entries = append(entries, entry)
	}
	r.mu.RUnlock()

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].NetworkID != entries[j].NetworkID {
			return entries[i].NetworkID < entries[j].NetworkID
		}
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].ID < entries[j].ID
	})
	counts := map[State]int{}
	coverageCounts := map[string]int{}
	for _, entry := range entries {
		counts[entry.State]++
		if entry.CoverageStatus != "" && entry.CoverageStatus != CoverageNotApplicable {
			coverageCounts[entry.CoverageStatus]++
		}
	}
	return Snapshot{SchemaVersion: SchemaVersion, GeneratedAt: now, Entries: entries, Counts: counts, CoverageCounts: coverageCounts}
}

// PublicSnapshot preserves operational evidence without exposing provider URLs,
// credentials, SQL errors or other details contained in wrapped errors.
func (r *Registry) PublicSnapshot() Snapshot {
	snapshot := r.Snapshot()
	for i := range snapshot.Entries {
		if snapshot.Entries[i].LastError != "" {
			snapshot.Entries[i].LastError = "component_check_failed"
		}
	}
	return snapshot
}

func assessCoverage(entry Entry) (status, reason string, attention bool) {
	if strings.TrimSpace(entry.Kind) != "head_ingest" {
		return CoverageNotApplicable, "component_is_not_head_ingest", false
	}
	if !entry.Configured || entry.State == StateDisabled || entry.State == StateStopped {
		return CoverageInactive, "head_ingest_not_active", false
	}
	if entry.Freshness == "stale" {
		return CoverageBlindSpot, "head_ingest_freshness_deadline_expired", true
	}
	if entry.State == StateUnavailable {
		return CoverageBlindSpot, "head_ingest_unavailable", true
	}
	if entry.Ingest == nil {
		return CoverageUnknown, "ingest_progress_not_observed", false
	}
	switch entry.Ingest.Status {
	case "reorg_recheck_required":
		return CoverageReorgGuard, "canonical_lineage_recheck_required", true
	case "provider_behind_cursor":
		return CoverageBlindSpot, "provider_head_is_behind_durable_cursor", true
	case "cycle_failed":
		return CoverageLagging, "latest_ingest_cycle_failed", true
	case "catching_up":
		return CoverageLagging, "durable_cursor_is_behind_observed_head", false
	case "at_observed_head":
		return CoverageCurrent, "durable_cursor_matches_observed_provider_head", false
	default:
		return CoverageUnknown, "ingest_progress_state_unknown", false
	}
}

func timePtr(value time.Time) *time.Time {
	v := value
	return &v
}
