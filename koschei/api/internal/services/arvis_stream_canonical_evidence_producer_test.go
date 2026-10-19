package services

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func arvisStreamCanonicalEvidenceProducerFixture(t *testing.T) (
	arvisStreamTarget, ActorDefenseRuleVerdict, UnifiedRadarBehaviorReport, GlobalRadarTrustedVerdictRegistry,
) {
	t.Helper()
	event, _, registry := arvisStreamCanonicalAcceptanceFixture(t, true, true)
	actor := ActorDefenseRuleVerdict{
		RulesetVersion: ActorDefenseRulesetVersion,
		Grade:          "B",
		Verdict:        "compounding_rule",
		TriggeredRules: []ActorDefenseRuleHit{
			{
				RuleID: ActorRuleCompoundCreatorReuse, Title: "Creator reuse",
				Tier: "compounding", EvidenceStatus: "verified",
				GradeEffect: "compounding_input", Summary: "fixture-only verified creator relation",
				Signatures:   []string{event.Signature},
				EvidenceKeys: []string{"fixture:creator"},
			},
			{
				RuleID: ActorRuleCompoundHolderReuse, Title: "Holder reuse",
				Tier: "compounding", EvidenceStatus: "verified",
				GradeEffect: "compounding_input", Summary: "fixture-only independent holder relation",
				Signatures:   []string{"other-verified-tx"},
				EvidenceKeys: []string{"fixture:holder"},
			},
		},
	}
	behavior := UnifiedRadarBehaviorReport{
		RulesetVersion: UnifiedRadarRulesetVersionV140,
		Mint:           event.Target,
		GeneratedAt:    time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
		Signals: []UnifiedRadarSignal{{
			RuleID:         UnifiedRuleCrossTokenExitEventRecurrence,
			Title:          "Transaction-referenced exit recurrence",
			EvidenceStatus: "verified", Triggered: true, GradeEffect: "evidence_only",
			Summary:      "fixture-only recurrence with verified reference",
			Signatures:   []string{"other-verified-tx"},
			EvidenceKeys: []string{"fixture:exit"},
		}},
	}
	return event, actor, behavior, registry
}

func TestArvisStreamCanonicalEvidenceProducerRequiresNativeBoundVerifiedInputs(t *testing.T) {
	event, actor, behavior, registry := arvisStreamCanonicalEvidenceProducerFixture(t)
	out, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actor, behavior, registry)
	if err != nil {
		t.Fatalf("real canonical actor/behavior evaluation and signing refused: %v", err)
	}
	if out.Grade != "B" || !out.Signed || out.RulesetVersion != UnifiedRadarRulesetVersionV140 {
		t.Fatalf("bad canonical signed v1.4 output: grade=%q signed=%v ruleset=%q", out.Grade, out.Signed, out.RulesetVersion)
	}
	if err := validateArvisStreamCanonicalVerdict(event, out, registry); err != nil {
		t.Fatalf("canonical signed verdict did not cite original trusted stream event: %v", err)
	}
	if out.Target != event.Target || out.Network != event.Network {
		t.Fatal("canonical decision was not bound to the stream target/network")
	}

	t.Run("wrong_mint", func(t *testing.T) {
		other := behavior
		other.Mint = "OtherMint111"
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actor, other, registry); !errors.Is(err, errArvisStreamInsufficientEvidence) {
			t.Fatalf("cross-token behavior accepted: %v", err)
		}
	})
	t.Run("wrong_network", func(t *testing.T) {
		other := event
		other.Network = "solana-devnet"
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(other, actor, behavior, registry); !errors.Is(err, errArvisStreamCanonicalFinalUnavailable) {
			t.Fatalf("unsupported stream source accepted: %v", err)
		}
	})
	t.Run("wrong_ruleset", func(t *testing.T) {
		old := behavior
		old.RulesetVersion = UnifiedRadarRulesetVersionV130
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actor, old, registry); !errors.Is(err, errArvisStreamInsufficientEvidence) {
			t.Fatalf("outdated canonical behavior accepted: %v", err)
		}
	})
	t.Run("missing_verified_actor", func(t *testing.T) {
		actorMissing := actor
		actorMissing.TriggeredRules = nil
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actorMissing, behavior, registry); !errors.Is(err, errArvisStreamInsufficientEvidence) {
			t.Fatalf("missing actor verification accepted: %v", err)
		}
	})
	t.Run("missing_verified_behavior", func(t *testing.T) {
		missing := behavior
		missing.Signals = nil
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actor, missing, registry); !errors.Is(err, errArvisStreamInsufficientEvidence) {
			t.Fatalf("missing behavior verification accepted: %v", err)
		}
	})
	t.Run("observed_only_behavior", func(t *testing.T) {
		observed := behavior
		observed.Signals = append([]UnifiedRadarSignal{}, behavior.Signals...)
		observed.Signals[0].EvidenceStatus = "observed"
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actor, observed, registry); !errors.Is(err, errArvisStreamInsufficientEvidence) {
			t.Fatalf("observed-only behavior promoted to verified: %v", err)
		}
	})
	t.Run("uncited_source_signature", func(t *testing.T) {
		other := event
		other.Signature = "unrelated-source-tx"
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(other, actor, behavior, registry); !errors.Is(err, errArvisStreamInsufficientEvidence) {
			t.Fatalf("uncited source was granted authority: %v", err)
		}
	})
	t.Run("fabricated_hard_grade", func(t *testing.T) {
		fabricated := actor
		fabricated.Verdict = "hard_trigger"
		fabricated.Grade = "D"
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(event, fabricated, behavior, registry); !errors.Is(err, errArvisStreamInsufficientEvidence) {
			t.Fatalf("fabricated hard ceiling accepted: %v", err)
		}
	})
	t.Run("signer_missing", func(t *testing.T) {
		t.Setenv(unifiedVerdictSigningPrivateKeyEnv, "")
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actor, behavior, registry); !errors.Is(err, errArvisStreamCanonicalFinalUnavailable) {
			t.Fatalf("unsigned canonical stream decision accepted: %v", err)
		}
	})
	t.Run("untrusted_signer", func(t *testing.T) {
		emptyRegistry, _ := NewGlobalRadarTrustedVerdictRegistry(nil)
		if _, err := evaluateArvisStreamCanonicalEvidenceFinal(event, actor, behavior, emptyRegistry); !errors.Is(err, errArvisStreamCanonicalFinalUnavailable) {
			t.Fatalf("untrusted signing key accepted: %v", err)
		}
	})
}

func TestArvisStreamCanonicalEvidenceProducerPostgres17AtomicAcceptance(t *testing.T) {
	url := os.Getenv("KOSCHEI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KOSCHEI_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() // always discard test observations

	event, actor, behavior, registry := arvisStreamCanonicalEvidenceProducerFixture(t)
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO security_radar_stream_events
		(provider,stream_mode,network,module_id,event_type,target,target_type,signature,evidence_quality)
		VALUES('ci_fixture','test',$1,$2,'test_canonical_producer',$3,'token',$4,'fixture_only')
		RETURNING id::text
	`, event.Network, event.ModuleID, event.Target, event.Signature).Scan(&event.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO arvis_stream_processing(stream_event_id,target,signature,status,attempts)
		VALUES($1::uuid,$2,$3,'processing',1)
	`, event.ID, event.Target, event.Signature); err != nil {
		t.Fatal(err)
	}
	var status string
	current := func() string {
		t.Helper()
		if err := tx.QueryRowContext(ctx, `SELECT status FROM arvis_stream_processing WHERE stream_event_id=$1::uuid`, event.ID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		return status
	}
	missingEvidence := behavior
	missingEvidence.Signals = nil
	if _, err := persistArvisStreamCanonicalEvidenceFinalTx(ctx, tx, event, actor, missingEvidence, registry); !errors.Is(err, errArvisStreamInsufficientEvidence) {
		t.Fatalf("no actor/behavior agreement was persisted: %v", err)
	}
	if got := current(); got != "processing" {
		t.Fatalf("invalid evidence prematurely completed: %s", got)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM arvis_stream_canonical_finals WHERE stream_event_id=$1::uuid`, event.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid evidence generated immutable receipt: count=%d err=%v", count, err)
	}

	first, err := persistArvisStreamCanonicalEvidenceFinalTx(ctx, tx, event, actor, behavior, registry)
	if err != nil {
		t.Fatalf("real v1.4 actor/behavior canonical final could not persist: %v", err)
	}
	if !first.Signed || first.Grade != "B" || current() != "completed" {
		t.Fatalf("genuine signed final did not atomically complete: grade=%q status=%s", first.Grade, status)
	}
	second, err := persistArvisStreamCanonicalEvidenceFinalTx(ctx, tx, event, actor, behavior, registry)
	if err != nil {
		t.Fatalf("verified same-evidence duplicate processing was not idempotent: %v", err)
	}
	if second.PayloadHash != first.PayloadHash || second.Signature != first.Signature {
		t.Fatal("duplicate signed final generated different authority")
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM arvis_stream_canonical_finals WHERE stream_event_id=$1::uuid`, event.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("canonical immutable receipt not unique: count=%d err=%v", count, err)
	}

	other := event
	other.Signature = "different-chain-transaction"
	if _, err := persistArvisStreamCanonicalEvidenceFinalTx(ctx, tx, other, actor, behavior, registry); err == nil {
		t.Fatal("a different source transaction substituted signed canonical evidence")
	}
	if current() != "completed" {
		t.Fatal("rejected source substitution downgraded an accepted canonical result")
	}
	// The real signed receipt must still pass trusted registry readback.
	if err := validateArvisStreamCanonicalVerdict(event, first, registry); err != nil {
		t.Fatalf("trusted canonical signed readback violated: %v", err)
	}
	if !strings.Contains(first.NarrativeSource, "deterministic_rules") {
		t.Fatalf("canonical actor/behavior evaluator was bypassed: narrative=%q", first.NarrativeSource)
	}
}
