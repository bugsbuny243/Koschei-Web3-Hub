package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// PersistArvisStreamCanonicalJobFinal connects an EXISTING canonical
// actor/behavior investigation with a real queued Pump/Raydium stream source.
// It never accepts caller-supplied stream identity, a legacy evidence-arm
// final, or a signing key furnished by the job.
//
// A source row and its matching processing claim must already exist. The
// trusted public-key registry is loaded from operator configuration, while
// the original transaction is fetched again from the mainnet RPC and checked
// against the source signature, slot, mint and executed program before a
// signed immutable final can atomically complete its claim.
//
// This method does not enable or enqueue the automatic stream worker.
func PersistArvisStreamCanonicalJobFinal(
	ctx context.Context,
	db *sql.DB,
	streamEventID, investigatedTarget, network string,
	actor ActorDefenseRuleVerdict,
	behavior UnifiedRadarBehaviorReport,
	solanaMainnetRPC string,
) (UnifiedRadarVerdict, error) {
	if db == nil || ctx == nil {
		return UnifiedRadarVerdict{}, errors.New("ARVIS stream canonical job requires a database and context")
	}
	streamEventID = strings.TrimSpace(streamEventID)
	investigatedTarget = strings.TrimSpace(investigatedTarget)
	network = strings.TrimSpace(network)
	if streamEventID == "" || investigatedTarget == "" || network != "solana-mainnet" {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: invalid canonical stream job identity", errArvisStreamCanonicalFinalUnavailable)
	}

	registry, configured, err := arvisStreamRecoveryTrustRegistry()
	if err != nil {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: invalid trusted operator registry: %v", errArvisStreamCanonicalFinalUnavailable, err)
	}
	if !configured {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: trusted operator public keys unavailable", errArvisStreamCanonicalFinalUnavailable)
	}

	var event arvisStreamTarget
	var kind, claimStatus, claimTarget, claimSignature string
	err = db.QueryRowContext(ctx, `
		SELECT s.id::text, COALESCE(s.target,''), COALESCE(s.signature,''),
		       COALESCE(s.network,''), COALESCE(s.slot,0),
		       COALESCE(s.program_id,''), COALESCE(s.module_id,''),
		       COALESCE(s.target_type,''), COALESCE(p.status,''),
		       COALESCE(p.target,''), COALESCE(p.signature,'')
		FROM security_radar_stream_events s
		JOIN arvis_stream_processing p ON p.stream_event_id=s.id
		WHERE s.id=$1::uuid
	`, streamEventID).Scan(
		&event.ID, &event.Target, &event.Signature, &event.Network,
		&event.Slot, &event.ProgramID, &event.ModuleID, &kind,
		&claimStatus, &claimTarget, &claimSignature,
	)
	if err != nil {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: original stream event and processing claim unavailable: %v", errArvisStreamCanonicalFinalUnavailable, err)
	}
	if event.Target != investigatedTarget || event.Network != network ||
		kind != "token" || (claimStatus != "processing" && claimStatus != "completed") ||
		claimTarget != event.Target || claimSignature != event.Signature ||
		!arvisStreamSourceProgramPermitted(event.ModuleID, event.ProgramID) {
		return UnifiedRadarVerdict{}, fmt.Errorf("%w: stream source and canonical investigation identity mismatch", errArvisStreamCanonicalFinalUnavailable)
	}
	return persistArvisStreamNativeEvidenceFinal(ctx, db, event, actor, behavior, registry, solanaMainnetRPC)
}
