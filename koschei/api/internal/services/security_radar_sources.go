package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SecurityRadarSource struct {
	ID        string    `json:"id"`
	ModuleID  string    `json:"module_id"`
	Label     string    `json:"label"`
	Address   string    `json:"address"`
	Network   string    `json:"network"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var ErrInvalidRadarSource = errors.New("invalid radar source")

func (s *SecurityRadarStore) ListSources(ctx context.Context) ([]SecurityRadarSource, error) {
	if s == nil || s.DB == nil {
		return []SecurityRadarSource{}, nil
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id::text, module_id, label, address, COALESCE(NULLIF(network,''),'solana-mainnet'), COALESCE(enabled,true), created_at, updated_at
		FROM security_radar_sources
		ORDER BY updated_at DESC, created_at DESC`)
	if err != nil {
		if isSecurityRadarMissingRelation(err) {
			return []SecurityRadarSource{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	items := []SecurityRadarSource{}
	for rows.Next() {
		var item SecurityRadarSource
		if err := rows.Scan(&item.ID, &item.ModuleID, &item.Label, &item.Address, &item.Network, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SecurityRadarStore) EnabledSources(ctx context.Context) ([]SecurityRadarSource, error) {
	if s == nil || s.DB == nil {
		return []SecurityRadarSource{}, nil
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id::text, module_id, label, address, COALESCE(NULLIF(network,''),'solana-mainnet'), COALESCE(enabled,true), created_at, updated_at
		FROM security_radar_sources
		WHERE enabled = true
		ORDER BY updated_at ASC
		LIMIT 50`)
	if err != nil {
		if isSecurityRadarMissingRelation(err) {
			return []SecurityRadarSource{}, nil
		}
		return nil, err
	}
	defer rows.Close()
	items := []SecurityRadarSource{}
	for rows.Next() {
		var item SecurityRadarSource
		if err := rows.Scan(&item.ID, &item.ModuleID, &item.Label, &item.Address, &item.Network, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SecurityRadarStore) CreateSource(ctx context.Context, src SecurityRadarSource) (SecurityRadarSource, error) {
	if s == nil || s.DB == nil {
		return SecurityRadarSource{}, sql.ErrConnDone
	}
	normalized, err := normalizeSecurityRadarSource(src)
	if err != nil {
		return SecurityRadarSource{}, err
	}
	var out SecurityRadarSource
	err = s.DB.QueryRowContext(ctx, `
		INSERT INTO security_radar_sources (module_id,label,address,network,enabled,name,target,target_type,provider,watch_mode,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$2,$3,'program',$6,$7,now(),now())
		ON CONFLICT (module_id, address, network) DO UPDATE SET
			label=EXCLUDED.label,
			enabled=EXCLUDED.enabled,
			name=EXCLUDED.label,
			target=EXCLUDED.address,
			provider=EXCLUDED.provider,
			watch_mode=EXCLUDED.watch_mode,
			updated_at=now()
		RETURNING id::text, module_id, label, address, network, enabled, created_at, updated_at`, normalized.ModuleID, normalized.Label, normalized.Address, normalized.Network, normalized.Enabled, SecurityRadarProvider, SecurityRadarWatchMode).Scan(&out.ID, &out.ModuleID, &out.Label, &out.Address, &out.Network, &out.Enabled, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *SecurityRadarStore) UpdateSource(ctx context.Context, id string, src SecurityRadarSource) (SecurityRadarSource, error) {
	if s == nil || s.DB == nil {
		return SecurityRadarSource{}, sql.ErrConnDone
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return SecurityRadarSource{}, fmt.Errorf("%w: id is required", ErrInvalidRadarSource)
	}
	normalized, err := normalizeSecurityRadarSource(src)
	if err != nil {
		return SecurityRadarSource{}, err
	}
	var out SecurityRadarSource
	err = s.DB.QueryRowContext(ctx, `
		UPDATE security_radar_sources
		SET module_id=$2, label=$3, address=$4, network=$5, enabled=$6, name=$3, target=$4, provider=$7, watch_mode=$8, updated_at=now()
		WHERE id=$1
		RETURNING id::text, module_id, label, address, network, enabled, created_at, updated_at`, id, normalized.ModuleID, normalized.Label, normalized.Address, normalized.Network, normalized.Enabled, SecurityRadarProvider, SecurityRadarWatchMode).Scan(&out.ID, &out.ModuleID, &out.Label, &out.Address, &out.Network, &out.Enabled, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *SecurityRadarStore) SetSourceEnabled(ctx context.Context, id string, enabled bool) error {
	if s == nil || s.DB == nil {
		return sql.ErrConnDone
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidRadarSource)
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE security_radar_sources SET enabled=$2, updated_at=now() WHERE id=$1`, id, enabled)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SecurityRadarStore) DeleteSource(ctx context.Context, id string) error {
	if s == nil || s.DB == nil {
		return sql.ErrConnDone
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidRadarSource)
	}
	res, err := s.DB.ExecContext(ctx, `DELETE FROM security_radar_sources WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func normalizeSecurityRadarSource(src SecurityRadarSource) (SecurityRadarSource, error) {
	src.ModuleID = strings.TrimSpace(src.ModuleID)
	src.Label = strings.TrimSpace(src.Label)
	src.Address = strings.TrimSpace(src.Address)
	src.Network = normalizeRadarNetwork(src.Network)
	if src.ModuleID == "" {
		return src, fmt.Errorf("%w: module_id is required", ErrInvalidRadarSource)
	}
	if !isAllowedSecurityRadarModule(src.ModuleID) {
		return src, fmt.Errorf("%w: unsupported module_id", ErrInvalidRadarSource)
	}
	if src.Label == "" {
		return src, fmt.Errorf("%w: label is required", ErrInvalidRadarSource)
	}
	if !isLikelySolanaAddress(src.Address) {
		return src, fmt.Errorf("%w: invalid Solana source address", ErrInvalidRadarSource)
	}
	return src, nil
}

func isAllowedSecurityRadarModule(moduleID string) bool {
	switch strings.TrimSpace(moduleID) {
	case ModulePumpSybilRadar, ModuleRaydiumPoolGuardian, ModuleWalletlessClaimShield:
		return true
	default:
		return false
	}
}

func isLikelySolanaAddress(address string) bool {
	address = strings.TrimSpace(address)
	if len(address) < 32 || len(address) > 64 {
		return false
	}
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, r := range address {
		if !strings.ContainsRune(alphabet, r) {
			return false
		}
	}
	return true
}
