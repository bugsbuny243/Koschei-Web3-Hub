package main

import (
	"context"
	"database/sql"
	"log"
	"net/url"

	"koschei/api/internal/alerts"
	"koschei/api/internal/runtimehealth"
	"koschei/api/internal/services"
)

// startEVMFlowRadars runs one observe-only, in-memory EVM Flow Radar per
// network in KOSCHEI_EVM_FLOW_RADAR_NETWORKS when KOSCHEI_EVM_FLOW_RADAR_ENABLED
// is on. Each network reads its existing *_RPC_URL. Findings become
// deduplicated security alerts when application persistence exists;
// otherwise they are logged only. A misconfigured network is skipped without
// affecting the others or the process.
func startEVMFlowRadars(ctx context.Context, db *sql.DB, health *runtimehealth.Registry) func() {
	networks, unsupported := services.EVMFlowRadarNetworks()
	if !services.EVMFlowRadarEnabled() {
		for _, network := range networks {
			health.Register(services.EVMFlowRadarHealthID(network), "worker", network, false)
		}
		return func() {}
	}
	for _, network := range unsupported {
		log.Printf("evm flow radar: network %q is not supported and was ignored", network)
	}
	var sink services.EVMFlowSink
	if db != nil {
		sink = func(ctx context.Context, finding services.EVMFlowFinding) {
			_, err := alerts.Emit(ctx, db, alerts.Event{
				Source: "evm_flow_radar", EventType: finding.Kind, Severity: finding.Severity, Target: finding.Address,
				Title: finding.Title, Message: finding.Message, DedupeKey: finding.DedupeKey,
				Payload: map[string]any{"kind": finding.Kind, "network": finding.Network, "observed_at": finding.ObservedAt, "evidence": finding.Evidence},
			})
			if err != nil {
				log.Printf("evm flow radar alert emit failed network=%s kind=%s: %v", finding.Network, finding.Kind, err)
			}
		}
	}
	stops := []func(){}
	for _, network := range networks {
		cfg, ok := services.LoadEVMFlowRadarConfigFromEnv(network)
		if !ok {
			continue
		}
		endpoint, err := url.Parse(cfg.Endpoint)
		if cfg.Endpoint == "" || err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
			log.Printf("evm flow radar unavailable for %s: %s must be set to an https URL", network, cfg.EndpointEnv)
			health.Register(services.EVMFlowRadarHealthID(network), "worker", network, false)
			continue
		}
		stops = append(stops, services.NewEVMFlowRadar(cfg, db, sink, health).Start(ctx))
	}
	return func() {
		for _, stop := range stops {
			stop()
		}
	}
}
