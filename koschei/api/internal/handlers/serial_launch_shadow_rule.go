package handlers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"koschei/api/internal/services"
)

// ARD-C006 runs in shadow mode only. It is reported beside the signed verdict
// so its behavior can be reviewed on real scans; it never enters the verdict,
// its signature, the actor queue or the grade until explicitly promoted.
const (
	serialLaunchShadowRuleID      = "ARD-C006"
	serialLaunchShadowRuleVersion = "koschei-serial-launch-shadow-v1"
	serialLaunchMinVerifiedTokens = 10
	serialLaunchMinInactiveRatio  = 0.9
	serialLaunchBurstWindow       = time.Hour
	serialLaunchMinBurstLaunches  = 10
	serialLaunchEvidenceLimit     = 50
)

type serialLaunchShadowEvidence struct {
	Mint              string     `json:"mint"`
	CreationSignature string     `json:"creation_signature,omitempty"`
	CreationSlot      int64      `json:"creation_slot,omitempty"`
	CreatedOnChainAt  *time.Time `json:"created_on_chain_at,omitempty"`
	FateStatus        string     `json:"fate_status"`
	LifecycleStatus   string     `json:"lifecycle_status"`
}

type serialLaunchShadowRule struct {
	RuleID              string                       `json:"rule_id"`
	Title               string                       `json:"title"`
	Version             string                       `json:"version"`
	Mode                string                       `json:"mode"`
	PromotionStatus     string                       `json:"promotion_status"`
	GradeEffect         string                       `json:"grade_effect"`
	WouldBeGradeEffect  string                       `json:"would_be_grade_effect"`
	Status              string                       `json:"status"`
	Triggered           bool                         `json:"triggered"`
	CreatorWallet       string                       `json:"creator_wallet"`
	VerifiedPriorTokens int                          `json:"verified_prior_tokens"`
	KnownFateTokens     int                          `json:"known_fate_tokens"`
	InactiveOrDead      int                          `json:"inactive_or_dead"`
	Active              int                          `json:"active"`
	InactiveRatio       float64                      `json:"inactive_ratio"`
	MaxLaunchesPerHour  int                          `json:"max_launches_per_hour"`
	FirstLaunchAt       *time.Time                   `json:"first_launch_at,omitempty"`
	LastLaunchAt        *time.Time                   `json:"last_launch_at,omitempty"`
	Summary             string                       `json:"summary"`
	Thresholds          map[string]any               `json:"thresholds"`
	Evidence            []serialLaunchShadowEvidence `json:"evidence"`
	Limitations         []string                     `json:"limitations"`
	RugClaimed          bool                         `json:"rug_claimed"`
	WrongdoingClaimed   bool                         `json:"wrongdoing_claimed"`
}

// evaluateSerialLaunchShadowRule looks for a verified pattern: many tokens
// canonically created by the same wallet inside one hour, almost all of which
// are inactive now. Only verified creator-mint outcomes count; the scanned
// mint itself is excluded, and tokens without a known fate are left out of the
// ratio rather than counted either way.
func evaluateSerialLaunchShadowRule(target string, history creatorOutcomeHistoryReport) serialLaunchShadowRule {
	target = strings.TrimSpace(target)
	out := serialLaunchShadowRule{
		RuleID: serialLaunchShadowRuleID, Title: "Verified serial launch factory", Version: serialLaunchShadowRuleVersion,
		Mode: "shadow", PromotionStatus: "shadow_only_not_promoted",
		GradeEffect: "none_shadow", WouldBeGradeEffect: "compounding_input",
		Status: "insufficient_evidence", CreatorWallet: strings.TrimSpace(history.CreatorWallet),
		Thresholds: map[string]any{
			"min_verified_prior_tokens": serialLaunchMinVerifiedTokens,
			"min_inactive_ratio":        serialLaunchMinInactiveRatio,
			"burst_window_seconds":      int(serialLaunchBurstWindow / time.Second),
			"min_launches_in_window":    serialLaunchMinBurstLaunches,
		},
		Evidence: []serialLaunchShadowEvidence{},
		Limitations: []string{
			"Shadow rule: reported for review only; it does not change the signed verdict, grade, Guard action or actor queue.",
			"Inactive or zero-liquidity status is a current market snapshot and does not establish a rug pull, exit scam or wrongdoing.",
			"Creator discovery is bounded; older or unverified creator-linked tokens may exist outside this sample.",
		},
	}

	launches := []time.Time{}
	for _, outcome := range history.Outcomes {
		mint := strings.TrimSpace(outcome.Mint)
		if mint == "" || mint == target || !strings.HasPrefix(outcome.EvidenceStatus, "verified_creator_mint") {
			continue
		}
		out.VerifiedPriorTokens++
		switch outcome.FateStatus {
		case services.ActorTokenFateInactiveOrDead:
			out.InactiveOrDead++
		case services.ActorTokenFateActive:
			out.Active++
		}
		if outcome.CreatedOnChainAt != nil {
			at := outcome.CreatedOnChainAt.UTC()
			launches = append(launches, at)
			if out.FirstLaunchAt == nil || at.Before(*out.FirstLaunchAt) {
				first := at
				out.FirstLaunchAt = &first
			}
			if out.LastLaunchAt == nil || at.After(*out.LastLaunchAt) {
				last := at
				out.LastLaunchAt = &last
			}
		}
		if len(out.Evidence) < serialLaunchEvidenceLimit {
			out.Evidence = append(out.Evidence, serialLaunchShadowEvidence{
				Mint: mint, CreationSignature: strings.TrimSpace(outcome.CreationSignature), CreationSlot: outcome.CreationSlot,
				CreatedOnChainAt: outcome.CreatedOnChainAt, FateStatus: outcome.FateStatus, LifecycleStatus: outcome.LifecycleStatus,
			})
		}
	}
	out.KnownFateTokens = out.InactiveOrDead + out.Active
	if out.KnownFateTokens > 0 {
		out.InactiveRatio = float64(out.InactiveOrDead) / float64(out.KnownFateTokens)
	}
	out.MaxLaunchesPerHour = maxLaunchesWithin(launches, serialLaunchBurstWindow)

	if out.KnownFateTokens < serialLaunchMinVerifiedTokens {
		out.Summary = fmt.Sprintf("%d verified prior tokens with a known fate; at least %d are required before the pattern is evaluated.", out.KnownFateTokens, serialLaunchMinVerifiedTokens)
		return out
	}
	out.Triggered = out.InactiveRatio >= serialLaunchMinInactiveRatio && out.MaxLaunchesPerHour >= serialLaunchMinBurstLaunches
	if out.Triggered {
		out.Status = "triggered_shadow"
		out.Summary = fmt.Sprintf("The creator wallet canonically created %d verified prior tokens, up to %d within one hour; %d of %d with a known fate are inactive now.",
			out.VerifiedPriorTokens, out.MaxLaunchesPerHour, out.InactiveOrDead, out.KnownFateTokens)
	} else {
		out.Status = "not_triggered"
		out.Summary = fmt.Sprintf("%d of %d verified prior tokens are inactive and at most %d launched within one hour; the serial-launch thresholds were not met.",
			out.InactiveOrDead, out.KnownFateTokens, out.MaxLaunchesPerHour)
	}
	return out
}

func maxLaunchesWithin(launches []time.Time, window time.Duration) int {
	sorted := append([]time.Time(nil), launches...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	best, start := 0, 0
	for end := range sorted {
		for sorted[end].Sub(sorted[start]) > window {
			start++
		}
		if count := end - start + 1; count > best {
			best = count
		}
	}
	return best
}
