package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

var (
	ErrGlobalCampaignStoreUnavailable = errors.New("global campaign store is unavailable")
	ErrGlobalCampaignRevisionConflict = errors.New("global campaign revision conflicts with persisted state")
	ErrGlobalCampaignRevisionGap      = errors.New("global campaign revision is not contiguous")
	ErrGlobalCampaignRevisionStale    = errors.New("global campaign revision is stale")
	ErrGlobalCampaignStoredCorrupt    = errors.New("persisted global campaign is corrupt")
)

type GlobalCampaignPersistResult struct {
	Campaign   GlobalCampaign
	Inserted   bool
	Idempotent bool
}

type globalCampaignStoredRow struct {
	Revision           int64
	SchemaVersion      string
	State              string
	FirstObservedAt    sql.NullTime
	LastObservedAt     sql.NullTime
	RulesetVersion     string
	EvidenceHashSHA256 string
	CanonicalPayload   []byte
}

// PersistGlobalCampaignRevision appends one immutable revision and atomically
// advances the current projection. The database is correlation memory only; it
// does not gain verdict, grade, incident, containment, or response authority.
func PersistGlobalCampaignRevision(ctx context.Context, db *sql.DB, campaign GlobalCampaign) (GlobalCampaignPersistResult, error) {
	if db == nil {
		return GlobalCampaignPersistResult{}, ErrGlobalCampaignStoreUnavailable
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return GlobalCampaignPersistResult{}, fmt.Errorf("begin global campaign persistence: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := persistGlobalCampaignRevisionTx(ctx, tx, campaign)
	if err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return GlobalCampaignPersistResult{}, fmt.Errorf("commit global campaign revision: %w", err)
	}
	return result, nil
}

// persistGlobalCampaignRevisionTx is the transaction-scoped persistence core.
// Candidate-resolution callers are responsible for evidence-anchor locking;
// this helper serializes the stable campaign identity itself.
func persistGlobalCampaignRevisionTx(ctx context.Context, tx *sql.Tx, campaign GlobalCampaign) (GlobalCampaignPersistResult, error) {
	if tx == nil {
		return GlobalCampaignPersistResult{}, ErrGlobalCampaignStoreUnavailable
	}
	campaign, payload, err := canonicalGlobalCampaignForStore(campaign)
	if err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	if campaign.Revision > math.MaxInt64 {
		return GlobalCampaignPersistResult{}, fmt.Errorf("global campaign revision exceeds PostgreSQL bigint range")
	}
	if campaign.LastObservedAt.Before(campaign.FirstObservedAt) {
		return GlobalCampaignPersistResult{}, fmt.Errorf("global campaign last observation precedes first observation")
	}

	if err := lockGlobalCampaignRefTx(ctx, tx, campaign.CampaignRef); err != nil {
		return GlobalCampaignPersistResult{}, err
	}

	storedRow, found, err := loadGlobalCampaignRowTx(ctx, tx, campaign.CampaignRef, true)
	if err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	if !found {
		if campaign.Revision != 1 {
			return GlobalCampaignPersistResult{}, fmt.Errorf("%w: first persisted revision must be 1, got %d", ErrGlobalCampaignRevisionGap, campaign.Revision)
		}
		if err := insertGlobalCampaignRevisionTx(ctx, tx, campaign, payload); err != nil {
			return GlobalCampaignPersistResult{}, err
		}
		if err := insertGlobalCampaignCurrentTx(ctx, tx, campaign, payload); err != nil {
			return GlobalCampaignPersistResult{}, err
		}
		if err := replaceGlobalCampaignAnchorIndexTx(ctx, tx, campaign); err != nil {
			return GlobalCampaignPersistResult{}, err
		}
		return GlobalCampaignPersistResult{Campaign: campaign, Inserted: true}, nil
	}

	current, err := decodeStoredGlobalCampaign(campaign.CampaignRef, storedRow)
	if err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	if campaign.Revision == current.Revision {
		if campaign.EvidenceHashSHA256 != current.EvidenceHashSHA256 {
			return GlobalCampaignPersistResult{}, fmt.Errorf("%w: campaign=%s revision=%d", ErrGlobalCampaignRevisionConflict, campaign.CampaignRef, campaign.Revision)
		}
		if err := replaceGlobalCampaignAnchorIndexTx(ctx, tx, current); err != nil {
			return GlobalCampaignPersistResult{}, err
		}
		return GlobalCampaignPersistResult{Campaign: current, Idempotent: true}, nil
	}
	if campaign.Revision < current.Revision {
		return GlobalCampaignPersistResult{}, fmt.Errorf("%w: current=%d incoming=%d", ErrGlobalCampaignRevisionStale, current.Revision, campaign.Revision)
	}
	if campaign.Revision != current.Revision+1 {
		return GlobalCampaignPersistResult{}, fmt.Errorf("%w: current=%d incoming=%d", ErrGlobalCampaignRevisionGap, current.Revision, campaign.Revision)
	}
	if campaign.FirstObservedAt.After(current.FirstObservedAt) {
		return GlobalCampaignPersistResult{}, fmt.Errorf("%w: first_observed_at cannot move forward", ErrGlobalCampaignRevisionConflict)
	}
	if campaign.LastObservedAt.Before(current.LastObservedAt) {
		return GlobalCampaignPersistResult{}, fmt.Errorf("%w: last_observed_at cannot move backward", ErrGlobalCampaignRevisionConflict)
	}
	if campaign.State != current.State {
		transition := GlobalCampaignTransition{
			From:         current.State,
			To:           campaign.State,
			ReasonCodes:  campaign.TransitionReasonCodes,
			EvidenceRefs: campaign.TransitionEvidenceRefs,
		}
		if err := ValidateGlobalCampaignTransition(transition); err != nil {
			return GlobalCampaignPersistResult{}, fmt.Errorf("invalid persisted global campaign lifecycle change: %w", err)
		}
	}

	if err := insertGlobalCampaignRevisionTx(ctx, tx, campaign, payload); err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE global_campaign_current
		SET revision=$2,
		    schema_version=$3,
		    state=$4,
		    first_observed_at=$5,
		    last_observed_at=$6,
		    ruleset_version=$7,
		    evidence_hash_sha256=$8,
		    canonical_payload=$9::jsonb,
		    updated_at=now()
		WHERE campaign_ref=$1 AND revision=$10`,
		campaign.CampaignRef,
		int64(campaign.Revision),
		campaign.SchemaVersion,
		string(campaign.State),
		campaign.FirstObservedAt,
		campaign.LastObservedAt,
		campaign.RulesetVersion,
		campaign.EvidenceHashSHA256,
		string(payload),
		storedRow.Revision,
	)
	if err != nil {
		return GlobalCampaignPersistResult{}, fmt.Errorf("advance global campaign current projection: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return GlobalCampaignPersistResult{}, fmt.Errorf("inspect global campaign projection update: %w", err)
	}
	if rows != 1 {
		return GlobalCampaignPersistResult{}, fmt.Errorf("%w: current projection changed during write", ErrGlobalCampaignRevisionConflict)
	}
	if err := replaceGlobalCampaignAnchorIndexTx(ctx, tx, campaign); err != nil {
		return GlobalCampaignPersistResult{}, err
	}
	return GlobalCampaignPersistResult{Campaign: campaign, Inserted: true}, nil
}

func lockGlobalCampaignRefTx(ctx context.Context, tx *sql.Tx, campaignRef string) error {
	var advisoryResult any
	if err := tx.QueryRowContext(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, campaignRef,
	).Scan(&advisoryResult); err != nil {
		return fmt.Errorf("lock global campaign %q: %w", campaignRef, err)
	}
	return nil
}

// LoadCurrentGlobalCampaign returns the validated current campaign projection.
// Any metadata/payload/hash disagreement fails closed as persisted corruption.
func LoadCurrentGlobalCampaign(ctx context.Context, db *sql.DB, campaignRef string) (GlobalCampaign, bool, error) {
	if db == nil {
		return GlobalCampaign{}, false, ErrGlobalCampaignStoreUnavailable
	}
	campaignRef = strings.TrimSpace(campaignRef)
	if campaignRef == "" {
		return GlobalCampaign{}, false, fmt.Errorf("global campaign reference is required")
	}
	row := db.QueryRowContext(ctx, `
		SELECT revision,schema_version,state,first_observed_at,last_observed_at,
		       ruleset_version,evidence_hash_sha256,canonical_payload
		FROM global_campaign_current
		WHERE campaign_ref=$1`, campaignRef)
	stored, found, err := scanGlobalCampaignStoredRow(row)
	if err != nil || !found {
		return GlobalCampaign{}, found, err
	}
	campaign, err := decodeStoredGlobalCampaign(campaignRef, stored)
	if err != nil {
		return GlobalCampaign{}, false, err
	}
	return campaign, true, nil
}

func canonicalGlobalCampaignForStore(campaign GlobalCampaign) (GlobalCampaign, []byte, error) {
	if err := ValidateGlobalCampaign(campaign); err != nil {
		return GlobalCampaign{}, nil, fmt.Errorf("invalid global campaign for persistence: %w", err)
	}
	payload, err := json.Marshal(campaign)
	if err != nil {
		return GlobalCampaign{}, nil, fmt.Errorf("marshal global campaign: %w", err)
	}
	canonical := NewGlobalCampaign(campaign)
	canonicalPayload, err := json.Marshal(canonical)
	if err != nil {
		return GlobalCampaign{}, nil, fmt.Errorf("marshal canonical global campaign: %w", err)
	}
	if !bytes.Equal(payload, canonicalPayload) {
		return GlobalCampaign{}, nil, fmt.Errorf("global campaign must be canonicalized before persistence")
	}
	return campaign, payload, nil
}

func loadGlobalCampaignRowTx(ctx context.Context, tx *sql.Tx, campaignRef string, forUpdate bool) (globalCampaignStoredRow, bool, error) {
	query := `
		SELECT revision,schema_version,state,first_observed_at,last_observed_at,
		       ruleset_version,evidence_hash_sha256,canonical_payload
		FROM global_campaign_current
		WHERE campaign_ref=$1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	stored, found, err := scanGlobalCampaignStoredRow(tx.QueryRowContext(ctx, query, campaignRef))
	if err != nil {
		return globalCampaignStoredRow{}, false, fmt.Errorf("load global campaign current projection: %w", err)
	}
	return stored, found, nil
}

func scanGlobalCampaignStoredRow(row *sql.Row) (globalCampaignStoredRow, bool, error) {
	var stored globalCampaignStoredRow
	err := row.Scan(
		&stored.Revision,
		&stored.SchemaVersion,
		&stored.State,
		&stored.FirstObservedAt,
		&stored.LastObservedAt,
		&stored.RulesetVersion,
		&stored.EvidenceHashSHA256,
		&stored.CanonicalPayload,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return globalCampaignStoredRow{}, false, nil
	}
	if err != nil {
		return globalCampaignStoredRow{}, false, err
	}
	return stored, true, nil
}

func decodeStoredGlobalCampaign(campaignRef string, stored globalCampaignStoredRow) (GlobalCampaign, error) {
	if stored.Revision <= 0 || !stored.FirstObservedAt.Valid || !stored.LastObservedAt.Valid {
		return GlobalCampaign{}, fmt.Errorf("%w: invalid stored metadata for %s", ErrGlobalCampaignStoredCorrupt, campaignRef)
	}
	decoder := json.NewDecoder(bytes.NewReader(stored.CanonicalPayload))
	decoder.DisallowUnknownFields()
	var campaign GlobalCampaign
	if err := decoder.Decode(&campaign); err != nil {
		return GlobalCampaign{}, fmt.Errorf("%w: decode %s: %v", ErrGlobalCampaignStoredCorrupt, campaignRef, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return GlobalCampaign{}, fmt.Errorf("%w: decode %s: %v", ErrGlobalCampaignStoredCorrupt, campaignRef, err)
	}
	if err := ValidateGlobalCampaign(campaign); err != nil {
		return GlobalCampaign{}, fmt.Errorf("%w: validate %s: %v", ErrGlobalCampaignStoredCorrupt, campaignRef, err)
	}
	if campaign.CampaignRef != campaignRef ||
		campaign.Revision != uint64(stored.Revision) ||
		campaign.SchemaVersion != stored.SchemaVersion ||
		string(campaign.State) != stored.State ||
		!campaign.FirstObservedAt.Equal(stored.FirstObservedAt.Time) ||
		!campaign.LastObservedAt.Equal(stored.LastObservedAt.Time) ||
		campaign.RulesetVersion != stored.RulesetVersion ||
		campaign.EvidenceHashSHA256 != stored.EvidenceHashSHA256 {
		return GlobalCampaign{}, fmt.Errorf("%w: metadata/payload mismatch for %s revision %d", ErrGlobalCampaignStoredCorrupt, campaignRef, stored.Revision)
	}
	canonical, _, err := canonicalGlobalCampaignForStore(campaign)
	if err != nil {
		return GlobalCampaign{}, fmt.Errorf("%w: non-canonical payload for %s: %v", ErrGlobalCampaignStoredCorrupt, campaignRef, err)
	}
	return canonical, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("unexpected trailing JSON value")
}

func insertGlobalCampaignRevisionTx(ctx context.Context, tx *sql.Tx, campaign GlobalCampaign, payload []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO global_campaign_revisions
			(campaign_ref,revision,schema_version,state,first_observed_at,last_observed_at,
			 ruleset_version,evidence_hash_sha256,canonical_payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)`,
		campaign.CampaignRef,
		int64(campaign.Revision),
		campaign.SchemaVersion,
		string(campaign.State),
		campaign.FirstObservedAt,
		campaign.LastObservedAt,
		campaign.RulesetVersion,
		campaign.EvidenceHashSHA256,
		string(payload),
	)
	if err != nil {
		return fmt.Errorf("insert global campaign revision: %w", err)
	}
	return nil
}

func insertGlobalCampaignCurrentTx(ctx context.Context, tx *sql.Tx, campaign GlobalCampaign, payload []byte) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO global_campaign_current
			(campaign_ref,revision,schema_version,state,first_observed_at,last_observed_at,
			 ruleset_version,evidence_hash_sha256,canonical_payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb)`,
		campaign.CampaignRef,
		int64(campaign.Revision),
		campaign.SchemaVersion,
		string(campaign.State),
		campaign.FirstObservedAt,
		campaign.LastObservedAt,
		campaign.RulesetVersion,
		campaign.EvidenceHashSHA256,
		string(payload),
	)
	if err != nil {
		return fmt.Errorf("insert global campaign current projection: %w", err)
	}
	return nil
}
