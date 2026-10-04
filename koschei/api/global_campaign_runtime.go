package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/services"
)

type campaignRadarStore struct {
	globalRadarGraphStore
	db *sql.DB
}

func (s *campaignRadarStore) InsertGlobalRadarSnapshot(ctx context.Context, snapshot services.GlobalRadarSnapshot) error {
	return services.StageGlobalRadarCampaignSnapshot(ctx, s.db, snapshot)
}

func buildGlobalCampaignRuntime(parent context.Context, db *sql.DB, sink globalRadarGraphStore, health *runtimehealth.Registry) (globalRadarGraphStore, *services.GlobalCampaignRuntimeConfig, error) {
	enabled := strings.TrimSpace(os.Getenv("KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED")) == "1"
	health.Register(services.GlobalCampaignRuntimeHealthID, "worker", "", enabled)
	if !enabled {
		return sink, nil, nil
	}
	if db == nil || sink == nil {
		return nil, nil, fmt.Errorf("campaign runtime requires APP_DATABASE_URL and verified Global Radar ClickHouse graph storage")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := services.VerifyGlobalCampaignRuntimeSchema(ctx, db); err != nil {
		return nil, nil, err
	}
	var identity [16]byte
	if _, err := rand.Read(identity[:]); err != nil {
		return nil, nil, err
	}
	cfg := &services.GlobalCampaignRuntimeConfig{DB: db, Owner: "campaign:" + hex.EncodeToString(identity[:]), Health: health, SnapshotSink: sink}
	return &campaignRadarStore{globalRadarGraphStore: sink, db: db}, cfg, nil
}
