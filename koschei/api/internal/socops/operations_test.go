package socops

import (
	"errors"
	"testing"
	"time"
)

func TestFirstResponseSLA(t *testing.T) {
	created := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	due := created.Add(15 * time.Minute)
	late := due.Add(time.Second)
	early := created.Add(-time.Second)
	future := due.Add(time.Hour)
	for _, tc := range []struct {
		name, severity, status string
		now                    time.Time
		response               *time.Time
		want                   string
	}{
		{"before deadline", "critical", "open", due.Add(-time.Second), nil, "pending"},
		{"deadline reached", "critical", "open", due, nil, "overdue"},
		{"on time remains met", "critical", "closed", due.Add(time.Hour), &due, "met"},
		{"late remains breached", "critical", "closed", due.Add(time.Hour), &late, "breached"},
		{"closed without evidence", "critical", "closed", due.Add(time.Hour), nil, "unknown"},
		{"unknown original severity", "", "open", due, nil, "unknown"},
		{"response before incident", "critical", "open", due, &early, "unknown"},
		{"future response", "critical", "open", due, &future, "unknown"},
		{"future creation", "critical", "open", created.Add(-time.Second), nil, "unknown"},
		{"reopened never resets first response", "critical", "open", due.Add(time.Hour), &late, "breached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := FirstResponse(tc.severity, tc.status, created, tc.now, tc.response)
			if got.State != tc.want {
				t.Fatalf("got %+v want %s", got, tc.want)
			}
		})
	}
	for severity, minutes := range map[string]int{"critical": 15, "high": 60, "medium": 240, "low": 1440, "info": 2880} {
		got := FirstResponse(severity, "open", created, created, nil)
		if got.DueAt == nil || !got.DueAt.Equal(created.Add(time.Duration(minutes)*time.Minute)) {
			t.Fatalf("wrong deadline for %s: %+v", severity, got)
		}
	}
	if got := FirstResponse("critical", "open", time.Time{}, created, nil); got.State != "unknown" {
		t.Fatal(got)
	}
}

func TestSOCCommandsPreserveRevisionAndAcknowledgement(t *testing.T) {
	zero := int64(0)
	one := int64(1)
	now := time.Now().UTC()
	before := State{Tier: "L1"}
	after, err := Apply(before, Command{Action: "assign", Assignee: "Analyst A", Summary: "Triage", ExpectedVersion: &zero}, "open", now)
	if err != nil || after.Version != 1 || after.Assignee != "Analyst A" || before.Assignee != "" {
		t.Fatalf("%+v %v", after, err)
	}
	if _, err := Apply(after, Command{Action: "assign", Assignee: "Analyst B", Summary: "Race", ExpectedVersion: &zero}, "open", now); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	acked, err := Apply(after, Command{Action: "acknowledge", Summary: "Reviewed", ExpectedVersion: &one}, "investigating", now)
	if err != nil || acked.AcknowledgedAt == nil || !acked.AcknowledgedAt.Equal(now) {
		t.Fatalf("%+v %v", acked, err)
	}
	two := int64(2)
	if _, err := Apply(acked, Command{Action: "acknowledge", Summary: "Reset", ExpectedVersion: &two}, "open", now.Add(time.Hour)); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := Apply(after, Command{Action: "escalate", Tier: "L1", Summary: "Same level", ExpectedVersion: &one}, "open", now); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	escalated, err := Apply(after, Command{Action: "escalate", Tier: "L3", Summary: "Specialist handoff", ExpectedVersion: &one}, "open", now)
	if err != nil || escalated.Tier != "L3" || escalated.Assignee != after.Assignee {
		t.Fatalf("%+v %v", escalated, err)
	}
	if _, err := Apply(escalated, Command{Action: "escalate", Tier: "L2", Summary: "Downgrade", ExpectedVersion: &two}, "open", now); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := Apply(after, Command{Action: "assign", Assignee: "B", Summary: "Closed", ExpectedVersion: &one}, "closed", now); !errors.Is(err, ErrInactive) {
		t.Fatal(err)
	}
	if _, err := Apply(after, Command{Action: "assign", Summary: "Unassign", ExpectedVersion: &one}, "open", now); err != nil {
		t.Fatal(err)
	}
}

func TestSOCCommandRejectsUnversionedOrUnsupportedInput(t *testing.T) {
	zero := int64(0)
	for _, c := range []Command{
		{Action: "assign", Summary: "No version"},
		{Action: "assign", ExpectedVersion: &zero},
		{Action: "pause", Summary: "No execution authority", ExpectedVersion: &zero},
		{Action: "escalate", Tier: "L4", Summary: "Invalid tier", ExpectedVersion: &zero},
		{Action: "acknowledge", Assignee: "spoof", Summary: "Irrelevant field", ExpectedVersion: &zero},
		{Action: "assign", Assignee: "bad\nlabel", Summary: "Control character", ExpectedVersion: &zero},
	} {
		if c.Validate() == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}
