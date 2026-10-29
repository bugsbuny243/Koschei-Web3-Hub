// Package socops defines operational metadata only. It has no security verdict
// or on-chain execution authority.
package socops

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

const Version = "soc-operations.v1"
const Policy = "soc-first-response.v1"

var ErrConflict = errors.New("SOC revision changed; reload the case")
var ErrInvalid = errors.New("invalid SOC action")
var ErrInactive = errors.New("resolved or closed case must be reopened before SOC changes")

type State struct {
	Tier           string     `json:"tier"`
	Assignee       string     `json:"assignee"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	Version        int64      `json:"version"`
}

type Command struct {
	Action          string `json:"action"`
	Tier            string `json:"tier"`
	Assignee        string `json:"assignee"`
	Summary         string `json:"summary"`
	ExpectedVersion *int64 `json:"expected_version"`
}

func ValidTier(tier string) bool { return tier == "L1" || tier == "L2" || tier == "L3" }

func (c *Command) Validate() error {
	c.Summary = strings.TrimSpace(c.Summary)
	c.Assignee = strings.TrimSpace(c.Assignee)
	if c.ExpectedVersion == nil || *c.ExpectedVersion < 0 || c.Summary == "" || len(c.Summary) > 4000 || len(c.Assignee) > 120 || strings.IndexFunc(c.Assignee, unicode.IsControl) >= 0 {
		return ErrInvalid
	}
	switch c.Action {
	case "assign":
		if c.Tier != "" {
			return ErrInvalid
		}
	case "escalate":
		if !ValidTier(c.Tier) || c.Assignee != "" {
			return ErrInvalid
		}
	case "acknowledge":
		if c.Tier != "" || c.Assignee != "" {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

func Apply(before State, c Command, status string, now time.Time) (State, error) {
	if err := c.Validate(); err != nil {
		return before, err
	}
	if before.Version != *c.ExpectedVersion {
		return before, ErrConflict
	}
	if status != "open" && status != "investigating" && status != "contained" {
		return before, ErrInactive
	}
	after := before
	switch c.Action {
	case "assign":
		if c.Assignee == before.Assignee {
			return before, ErrInvalid
		}
		after.Assignee = c.Assignee
	case "escalate":
		if !ValidTier(before.Tier) || c.Tier <= before.Tier {
			return before, ErrInvalid
		}
		after.Tier = c.Tier
	case "acknowledge":
		if before.AcknowledgedAt != nil {
			return before, ErrInvalid
		}
		t := now.UTC()
		after.AcknowledgedAt = &t
	}
	after.Version++
	return after, nil
}

type SLA struct {
	Policy      string     `json:"policy"`
	State       string     `json:"state"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	RespondedAt *time.Time `json:"responded_at,omitempty"`
}

// FirstResponse measures the first explicit acknowledgement OR persisted
// investigation/containment/resolution action, whichever happened first.
// The initial severity is used, so later reprioritization cannot hide a breach.
func FirstResponse(severity, status string, created, now time.Time, response *time.Time) SLA {
	minutes, known := map[string]int{"critical": 15, "high": 60, "medium": 240, "low": 1440, "info": 2880}[severity]
	s := SLA{Policy: Policy, State: "unknown"}
	if !known || created.IsZero() || now.Before(created) {
		return s
	}
	due := created.UTC().Add(time.Duration(minutes) * time.Minute)
	s.DueAt = &due
	if response != nil {
		if response.Before(created) || response.After(now) {
			return s
		}
		s.RespondedAt = response
		s.State = "met"
		if response.After(due) {
			s.State = "breached"
		}
		return s
	}
	if status == "resolved" || status == "closed" {
		return s
	}
	if status != "open" && status != "investigating" && status != "contained" {
		return s
	}
	s.State = "pending"
	if !now.Before(due) {
		s.State = "overdue"
	}
	return s
}
