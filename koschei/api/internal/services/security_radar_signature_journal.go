package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Signature-only rows are *observation receipts*, not validated token targets,
// ARVIS final verdicts, transaction effects, or an assertion of complete coverage.
// This path is limited to opted-in journal/gap-recovery callers. Never weaken
// InsertStreamEvent's independent verified-mint contract.
const securityRadarSignatureJournalMaxBytes = 262144

func securityRadarValidSignature(signature string) bool {
	if len(signature) < 80 || len(signature) > 88 {
		return false
	}
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, c := range signature {
		if !strings.ContainsRune(alphabet, c) {
			return false
		}
	}
	return true
}

func validateSecurityRadarSignatureJournal(event SecurityRadarStreamEventRecord) error {
	if event.Network != "solana-mainnet" || !securityRadarValidSignature(event.Signature) ||
		event.Slot <= 0 || !isLikelyRadarSolanaAddress(event.ProgramID) ||
		(event.Target != "" && event.Target != event.Signature) {
		return errors.New("signature journal requires a Solana mainnet signature, slot and program source")
	}
	switch event.Provider {
	case SecurityRadarStreamProvider:
		if event.StreamMode != SecurityRadarStreamModeLogs || event.EvidenceQuality != "raw_stream" &&
			event.EvidenceQuality != "decoded_stream_hint" {
			return errors.New("WSS journal source mode or evidence quality mismatch")
		}
		if event.RawEvent["method"] != "logsNotification" {
			return errors.New("WSS journal notification method mismatch")
		}
		parameters := asRadarMap(event.RawEvent["params"])
		result := asRadarMap(parameters["result"])
		value := asRadarMap(result["value"])
		if anyString(value["signature"]) != event.Signature ||
			radarInt64(asRadarMap(result["context"])["slot"]) != event.Slot {
			return errors.New("WSS notification signature or slot mismatch")
		}
	case "solana_rpc_gap_healer":
		if (event.StreamMode != "gap_replay" && event.StreamMode != "gap_bootstrap_anchor") ||
			(event.EvidenceQuality != "replay_signature_observed" &&
				event.EvidenceQuality != "replay_bootstrap_anchor") {
			return errors.New("RPC replay provenance or evidence quality mismatch")
		}
		if (event.StreamMode == "gap_replay") != (event.EvidenceQuality == "replay_signature_observed") {
			return errors.New("RPC replay stream mode/evidence mismatch")
		}
		if event.Decoded["rpc_method"] != "getSignaturesForAddress" ||
			anyString(event.RawEvent["signature"]) != event.Signature ||
			radarInt64(event.RawEvent["slot"]) != event.Slot {
			return errors.New("RPC replay signature or slot evidence mismatch")
		}
		if event.StreamMode == "gap_replay" && event.RawEvent["replayed"] != true {
			return errors.New("RPC replay provenance marker missing")
		}
		if event.StreamMode == "gap_bootstrap_anchor" && event.RawEvent["anchor"] != true {
			return errors.New("RPC bootstrap anchor provenance marker missing")
		}
	default:
		return errors.New("unsupported signature journal source")
	}
	if strings.TrimSpace(event.ModuleID) == "" || strings.TrimSpace(event.EventType) == "" {
		return errors.New("signature journal module/event metadata missing")
	}
	return nil
}

// InsertSignatureJournalEvent stores exactly one low-trust program-signature
// observation without forging a token mint target. For duplicates the returned
// ID is empty; callers must NOT count that as newly recovered evidence.
// The raw WSS/RPC notification is committed only as a digest; canonical
// metadata and proof origin remain in decoded for subsequent enrichment.
func (s *SecurityRadarStore) InsertSignatureJournalEvent(ctx context.Context, event SecurityRadarStreamEventRecord) (string, error) {
	if err := validateSecurityRadarSignatureJournal(event); err != nil {
		return "", err
	}
	if s == nil || s.DB == nil {
		return "", errors.New("signature journal database unavailable")
	}
	decoded, err := json.Marshal(nonNilMap(event.Decoded))
	if err != nil {
		return "", fmt.Errorf("encode signature observation: %w", err)
	}
	raw, err := json.Marshal(nonNilMap(event.RawEvent))
	if err != nil {
		return "", fmt.Errorf("encode signature notification: %w", err)
	}
	if len(raw) > securityRadarSignatureJournalMaxBytes || len(decoded) > securityRadarSignatureJournalMaxBytes {
		return "", errors.New("signature journal payload exceeds bounded size")
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	// Deliberately NULL target. An observed transaction signature is never a
	// token address, even if the caller provided its signature as Target.
	var id string
	err = s.DB.QueryRowContext(ctx, `
		INSERT INTO security_radar_stream_events
		    (provider,stream_mode,network,module_id,event_type,target,target_type,
		     signature,slot,program_id,evidence_quality,decoded,raw_event,raw_event_sha256,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,NULL,'signature',$6,$7,$8,$9,$10::jsonb,NULL,$11,now(),now())
		ON CONFLICT DO NOTHING RETURNING id::text
	`, event.Provider, event.StreamMode, event.Network, event.ModuleID, event.EventType,
		event.Signature, event.Slot, event.ProgramID, event.EvidenceQuality, string(decoded), hash).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
