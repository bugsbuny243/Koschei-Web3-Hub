package services

import "testing"

func TestClaimDiscoveryProducesPinnedProgramEvidence(t *testing.T) {
	profile := radarEvidenceProfile{
		IsTokenMint: true,
		HolderRoles: HolderRoleAnalysis{
			Available: true,
			Accounts: []HolderRoleAccount{
				{TokenAccount: "TokenAccountFixture", OwnerProgram: "strmRqUCoQUgGUan5YhzUZa6KqdzwX5L6FpUxfmKg5m", Balance: 123.5},
			},
		},
	}
	discovery := discoverClaimFromHolderRoles(profile)
	if !discovery.Attempted || !discovery.Found ||
		discovery.Program != "streamflow" || discovery.Account != "TokenAccountFixture" {
		t.Fatalf("holder-owner program intersection lost: %+v", discovery)
	}
	req := SecurityRadarRequest{Target: "MintFixture", Network: "solana-mainnet"}
	shield, surface := buildClaimDiscoveryArms(req, profile, "2026-10-15T00:00:00Z")
	if shield.ModuleID != ModuleWalletlessClaimShield || surface.ModuleID != ModuleClaimSurfaceRisk {
		t.Fatalf("claim arms must retain their distinct module IDs: %q / %q", shield.ModuleID, surface.ModuleID)
	}
	for _, arm := range []SecurityRadarVerdict{shield, surface} {
		if got := arvisSignalString(arm.Signals, "reason_code"); got != "known_distributor_program_observed" {
			t.Fatalf("claim arm must carry the observed program relation: %q", got)
		}
		if got := arvisSignalString(arm.Signals, "execution_status"); got != ArvisExecutionCompleted {
			t.Fatalf("found claim program must complete evidence collection: %q", got)
		}
	}
}

func TestClaimDiscoveryNoMatchIsBoundedPendingNotSafe(t *testing.T) {
	profile := radarEvidenceProfile{
		IsTokenMint: true,
		HolderRoles: HolderRoleAnalysis{
			Available: true,
			Accounts: []HolderRoleAccount{
				{TokenAccount: "OtherAccountFixture", OwnerProgram: "UnrecognizedProgram"},
			},
		},
	}
	req := SecurityRadarRequest{Target: "MintFixture", Network: "solana-mainnet"}
	shield, surface := buildClaimDiscoveryArms(req, profile, "2026-10-15T00:00:00Z")
	for _, arm := range []SecurityRadarVerdict{shield, surface} {
		if got := arvisSignalString(arm.Signals, "reason_code"); got != "no_known_distributor_program_found" {
			t.Fatalf("no-match reason missing: %q", got)
		}
		if got := arvisSignalString(arm.Signals, "execution_status"); got != ArvisExecutionEvidencePending {
			t.Fatalf("incomplete creator/program scopes must not be marked completed: %q", got)
		}
		if got := arvisSignalString(arm.Signals, "limitation"); got != claimDiscoveryLimitation {
			t.Fatalf("unknown custom distributor not ruled out: %q", got)
		}
		if _, exists := arm.Signals["observed_slot"]; exists {
			t.Fatalf("holder-role collector has no exact slot; do not fabricate it: %+v", arm.Signals)
		}
		if arm.Recommendation == "allow" {
			t.Fatalf("missing distributor match must never become an approval: %+v", arm)
		}
	}
}

func TestClaimDiscoveryWithoutHolderEvidenceNeverClaimsAbsence(t *testing.T) {
	p := radarEvidenceProfile{IsTokenMint: true}
	got := discoverClaimFromHolderRoles(p)
	if got.Attempted || got.Found || got.Reason != "claim_holder_evidence_unavailable" {
		t.Fatalf("unavailable holder data is not a completed negative: %+v", got)
	}
}
