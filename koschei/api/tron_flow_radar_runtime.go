package main

import (
	"context"
	"database/sql"
	"log"
	"net/url"
	"strings"

	"koschei/api/internal/alerts"
	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/services"
)

// startTronFlowRadar runs the observe-only, in-memory Tron Flow Radar when
// KOSCHEI_TRON_FLOW_RADAR_ENABLED=1. Findings become deduplicated security
// alerts when application persistence exists; otherwise they are logged only.
func startTronFlowRadar(ctx context.Context, db *sql.DB, health *runtimehealth.Registry) func() {
	if !services.TronFlowRadarEnabled() {
		health.Register(services.TronFlowRadarHealthID, "worker", "tron-mainnet", false)
		return func() {}
	}
	cfg := services.LoadTronFlowRadarConfigFromEnv()
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
		log.Printf("tron flow radar not started: KOSCHEI_TRON_FLOW_RADAR_RPC_URL must be an https URL")
		health.Register(services.TronFlowRadarHealthID, "worker", "tron-mainnet", false)
		return func() {}
	}
	if cfg.APIKey == "" && strings.HasSuffix(endpoint.Hostname(), "trongrid.io") {
		log.Printf("tron flow radar not started: TRONGRID_API_KEY is required for TronGrid")
		health.Register(services.TronFlowRadarHealthID, "worker", "tron-mainnet", false)
		return func() {}
	}
	var sink services.TronFlowSink
	if db != nil {
		sink = func(ctx context.Context, finding services.TronFlowFinding) {
			_, err := alerts.Emit(ctx, db, alerts.Event{
				Source: "tron_flow_radar", EventType: finding.Kind, Severity: finding.Severity, Target: finding.Address,
				Title: finding.Title, Message: finding.Message, DedupeKey: finding.DedupeKey,
				Payload: map[string]any{"kind": finding.Kind, "observed_at": finding.ObservedAt, "evidence": finding.Evidence},
			})
			if err != nil {
				log.Printf("tron flow radar alert emit failed kind=%s: %v", finding.Kind, err)
			}
		}
	}
	return services.NewTronFlowRadar(cfg, db, sink, health).Start(ctx)
}
