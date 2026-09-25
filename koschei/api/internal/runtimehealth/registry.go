package runtimehealth

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = "koschei.runtime-health.v1"

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
	ID                  string     `json:"id"`
	Kind                string     `json:"kind"`
	NetworkID           string     `json:"network_id,omitempty"`
	State               State      `json:"state"`
	Configured          bool       `json:"configured"`
	SuccessfulCycles    uint64     `json:"successful_cycles"`
	FailedCycles        uint64     `json:"failed_cycles"`
	Observations        uint64     `json:"observations"`
	ConsecutiveFailures uint64     `json:"consecutive_failures"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type Snapshot struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Entries       []Entry       `json:"entries"`
	Counts        map[State]int `json:"counts"`
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
		return Snapshot{SchemaVersion: SchemaVersion, GeneratedAt: now, Entries: []Entry{}, Counts: map[State]int{}}
	}
	now = r.now().UTC()
	r.mu.RLock()
	entries := make([]Entry, 0, len(r.entries))
	for _, entry := range r.entries {
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
	for _, entry := range entries {
		counts[entry.State]++
	}
	return Snapshot{SchemaVersion: SchemaVersion, GeneratedAt: now, Entries: entries, Counts: counts}
}

func timePtr(value time.Time) *time.Time {
	v := value
	return &v
}
