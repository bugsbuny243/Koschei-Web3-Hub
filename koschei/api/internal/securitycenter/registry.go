// Package securitycenter describes the additive Koschei global security-center
// capability graph. It is repository capability metadata, not deployment health.
package securitycenter

import "koschei/api/internal/networktarget"

const SchemaVersion = "koschei.security-center-capability.v1"

type Capability struct {
	ID                string   `json:"id"`
	Layer             string   `json:"layer"`
	Domain            string   `json:"domain"`
	Mode              string   `json:"mode"`
	EvidenceAuthority string   `json:"evidence_authority"`
	Activation        string   `json:"activation"`
	NetworkIDs        []string `json:"network_ids,omitempty"`
	DependsOn         []string `json:"depends_on,omitempty"`
	BackendSurfaces   []string `json:"backend_surfaces,omitempty"`
	FrontendSurfaces  []string `json:"frontend_surfaces,omitempty"`
	Telemetry         []string `json:"telemetry,omitempty"`
	Notes             []string `json:"notes,omitempty"`
}

type AssetBinding struct {
	Asset   string `json:"asset"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Surface string `json:"surface"`
	Reason  string `json:"reason"`
}

type Snapshot struct {
	SchemaVersion     string                  `json:"schema_version"`
	Name              string                  `json:"name"`
	Purpose           string                  `json:"purpose"`
	PreserveExisting  bool                    `json:"preserve_existing"`
	BreakingChanges   bool                    `json:"breaking_changes"`
	DecisionAuthority string                  `json:"decision_authority"`
	EvidenceRule      string                  `json:"evidence_rule"`
	Networks          []networktarget.Network `json:"networks"`
	Capabilities      []Capability            `json:"capabilities"`
	AssetBindings     []AssetBinding          `json:"asset_bindings"`
}

func networkIDs() []string {
	catalog := networktarget.Catalog()
	out := make([]string, 0, len(catalog))
	for _, network := range catalog {
		out = append(out, network.ID)
	}
	return out
}

// Current returns the repository capability graph. "Implemented" here means a
// code path exists in this repository; it never means the configured deployment
// or an upstream provider is healthy. Runtime checks remain separate and
// missing evidence remains UNKNOWN/UNAVAILABLE.
func Current() Snapshot {
	allNetworks := networkIDs()
	return Snapshot{
		SchemaVersion:     SchemaVersion,
		Name:              "Koschei Global Crypto Security Center",
		Purpose:           "federated evidence collection, correlation, deterministic security decisions, defense validation, alerting and operator visibility",
		PreserveExisting:  true,
		BreakingChanges:   false,
		DecisionAuthority: "ARVIS remains authoritative only where its native evidence and verdict contract is connected; Fabric and Sentinel observe-first paths do not silently promote decisions",
		EvidenceRule:      "missing evidence stays UNKNOWN or UNAVAILABLE and can never be converted into SAFE",
		Networks:          networktarget.Catalog(),
		Capabilities: []Capability{
			{
				ID: "global-radar-event-plane", Layer: "ingest", Domain: "multi-network", Mode: "implemented", EvidenceAuthority: "observation_only", Activation: "adapter_and_runtime_config",
				NetworkIDs:      allNetworks,
				BackendSurfaces: []string{"/fabric/networks/probe/intelligence", "/fabric/radar/global"},
				Telemetry:       []string{"koschei.global-radar-event.v1", "native source SHA-256 digests"},
				Notes:           []string{"chain adapters emit evidence without manufacturing a verdict", "canonical event persistence is optional and fail-closed when configured"},
			},
			{
				ID: "runtime-health-registry", Layer: "control-plane", Domain: "runtime-observability", Mode: "implemented", EvidenceAuthority: "operational_status_only", Activation: "always available in HTTP runtime; entries reflect configured components",
				BackendSurfaces:  []string{"/fabric/security-center/runtime-health"},
				FrontendSurfaces: []string{"/fabric/security-center"},
				Telemetry:        []string{"configured", "live", "degraded", "unavailable", "disabled", "stopped", "last success/failure", "cycle and observation counters", "periodic success freshness and deadline"},
				Notes:            []string{"runtime health is not chain risk", "endpoint identities and secrets are never exposed"},
			},
			{
				ID: "continuous-multi-network-block-ingest", Layer: "ingest", Domain: "evm-utxo", Mode: "conditional", EvidenceAuthority: "observation_only", Activation: "KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_ENABLED=1 plus event/checkpoint ClickHouse and explicit network allowlist",
				NetworkIDs:       []string{"ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet", "polygon-mainnet", "bnb-mainnet", "avalanche-mainnet", "bitcoin-mainnet"},
				DependsOn:        []string{"global-radar-event-plane", "runtime-health-registry"},
				BackendSurfaces:  []string{"/api/owner/radar/global/events", "/fabric/security-center/runtime-health"},
				FrontendSurfaces: []string{"/owner-production", "/fabric/security-center"},
				Telemetry:        []string{"canonical block events", "transaction identity events", "EVM log events", "native RPC response SHA-256", "durable ingest checkpoints", "reorg observation state", "per-network worker health"},
				Notes:            []string{"cursor advances only after event persistence", "parent/hash discontinuity never silently advances", "no mempool or full transaction-body completeness claim", "ARVIS verdict authority is unchanged"},
			},
			{
				ID: "durable-ingest-gap-accounting", Layer: "recovery", Domain: "multi-network-operations", Mode: "implemented", EvidenceAuthority: "operational_replay_state_only", Activation: "Global Radar event ClickHouse plus continuous head ingest",
				NetworkIDs:       []string{"ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet", "polygon-mainnet", "bnb-mainnet", "avalanche-mainnet", "bitcoin-mainnet"},
				DependsOn:        []string{"continuous-multi-network-block-ingest", "durable-evidence-memory"},
				BackendSurfaces:  []string{"global_radar_ingest_gaps", "global_radar_ingest_checkpoints", "global_radar_events"},
				FrontendSurfaces: []string{"/fabric/security-center"},
				Telemetry:        []string{"gap episode identity", "initial failed height", "next replay height", "bounded end height", "failure reason", "open/resolved state", "detection/update/recovery timestamps"},
				Notes:            []string{"backlog alone is not a gap", "a gap opens only after sequential ingest fails at a concrete height", "replay advances next_height only after durable event and checkpoint persistence", "resolved means the recorded range was replayed; it does not prove source-chain historical completeness"},
			},
			{
				ID: "multi-provider-lineage-confirmation", Layer: "verification", Domain: "evm-utxo", Mode: "conditional", EvidenceAuthority: "operational_lineage_confirmation_only", Activation: "independent confirmation RPC plus KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_REQUIRE_CONFIRMATION=1 and/or KOSCHEI_GLOBAL_RADAR_HEAD_INGEST_AUTO_REORG_RECOVERY=1",
				NetworkIDs:       []string{"ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet", "polygon-mainnet", "bnb-mainnet", "avalanche-mainnet", "bitcoin-mainnet"},
				DependsOn:        []string{"continuous-multi-network-block-ingest", "durable-evidence-memory"},
				BackendSurfaces:  []string{"/fabric/security-center/runtime-health"},
				FrontendSurfaces: []string{"/fabric/security-center"},
				Telemetry:        []string{"independent block hash/parent confirmation", "provider disagreement", "bounded canonical-checkpoint ancestor search", "rewind checkpoint state", "reorg recovery health"},
				Notes:            []string{"two-provider agreement is not finality", "provider disagreement fails closed without event persistence", "automatic recovery rewinds only to a historically stored checkpoint that both providers independently match", "rewind depth is bounded to 64 blocks"},
			},
			{
				ID: "solana-arvis-evidence-and-verdict", Layer: "decision", Domain: "solana", Mode: "active", EvidenceAuthority: "authoritative_arvis", Activation: "FEATURE_SOLANA and evidence sources",
				NetworkIDs:       []string{"solana-mainnet"},
				BackendSurfaces:  []string{"/api/scan", "/api/token/scan", "/api/owner/radar/unified", "/api/v1/scan/token"},
				FrontendSurfaces: []string{"/scan", "/dashboard", "/owner-production"},
				Telemetry:        []string{"deterministic A-F verdict", "Ed25519 verdict signing", "evidence references"},
			},
			{
				ID: "radar-flow-observability", Layer: "observability", Domain: "solana-arvis-operations", Mode: "implemented", EvidenceAuthority: "operational_metrics_only", Activation: "APP_DATABASE_URL plus owner authentication",
				NetworkIDs:      []string{"solana-mainnet"},
				DependsOn:       []string{"solana-arvis-evidence-and-verdict", "durable-evidence-memory"},
				BackendSurfaces: []string{"/api/owner/radar/flow-metrics"},
				Telemetry:       []string{"15-minute and 24-hour collected/recognized/enriched/processed/verified-verdict counts", "evidence-quality distribution", "network event distribution", "queue backlog and staleness", "stage throughput basis points", "stage freshness age", "durable alert rows separated from verified verdicts"},
				Notes:           []string{"bounded windows avoid all-time hot-store scans", "recognized does not mean verified", "metrics describe persisted pipeline rows and do not claim chain-wide coverage"},
			},
			{
				ID: "global-radar-coverage-alert-lifecycle", Layer: "response", Domain: "multi-network-operations", Mode: "implemented", EvidenceAuthority: "operational_notification_only", Activation: "APP_DATABASE_URL plus background runtime and head-ingest health entries",
				NetworkIDs:       []string{"ethereum-mainnet", "base-mainnet", "arbitrum-mainnet", "optimism-mainnet", "polygon-mainnet", "bnb-mainnet", "avalanche-mainnet", "bitcoin-mainnet"},
				DependsOn:        []string{"runtime-health-registry", "continuous-multi-network-block-ingest", "watchlist-alert-and-webhook-plane"},
				BackendSurfaces:  []string{"global_radar_coverage_episodes", "security_alert_events", "security_alert_deliveries", "/fabric/security-center/runtime-health", "/api/owner/radar/coverage-episodes"},
				FrontendSurfaces: []string{"/fabric/security-center"},
				Telemetry:        []string{"open/recovered coverage episodes", "deduplicated attention alerts", "recovery alerts", "coverage reason transitions", "durable alert-delivery rows and delivery status"},
				Notes:            []string{"blind_spot is an operational monitoring-confidence signal, not proof of historical chain-data loss", "catching_up alone does not close or open a blind-spot episode", "recovery requires current head/cursor alignment", "ARVIS verdict authority is unchanged"},
			},
			{
				ID: "global-campaign-runtime", Layer: "correlation", Domain: "campaign-operations", Mode: "observe", EvidenceAuthority: "canonical_reference_projection_only", Activation: "KOSCHEI_GLOBAL_CAMPAIGN_RUNTIME_ENABLED=1; APP_DATABASE_URL; verified graph storage; migrations through 129",
				DependsOn:        []string{"cross-network-evidence-graph", "runtime-health-registry"},
				BackendSurfaces:  []string{"/api/owner/campaigns", "global_campaign_runtime_queue"},
				FrontendSurfaces: []string{"/owner/campaigns", "/fabric/security-center"},
				Telemetry:        []string{"worker.global-campaign-materializer", "pending/failed/processed source counts", "oldest pending and last processed timestamps"},
				Notes:            []string{"verified linked snapshots only", "threat inputs remain missing until connected", "automatic response disabled; no containment authority"},
			},
			{
				ID: "cross-process-job-wake", Layer: "orchestration", Domain: "worker-runtime", Mode: "implemented", EvidenceAuthority: "operational_status_only", Activation: "APP_DATABASE_URL required; NATS_URL is optional latency acceleration",
				DependsOn:       []string{"runtime-health-registry"},
				BackendSurfaces: []string{"web3_jobs", "/fabric/security-center/runtime-health"},
				Telemetry:       []string{"worker.cross-process-job-wake configured/live/degraded/unavailable state", "remote wake observations", "last success/failure"},
				Notes:           []string{"PostgreSQL web3_jobs remains the durable job authority", "Core NATS carries wake hints only; no JetStream or replay guarantee is claimed", "worker recovery ceiling remains the bounded fallback when remote wake is unavailable"},
			},
			{
				ID: "multi-network-native-probes", Layer: "sensor", Domain: "evm-utxo-move", Mode: "probe_ready", EvidenceAuthority: "evidence_only", Activation: "network endpoint configuration",
				NetworkIDs:       allNetworks,
				BackendSurfaces:  []string{"/fabric/networks/probe", "/fabric/networks/probe/intelligence", "/api/scan/approval"},
				FrontendSurfaces: []string{"/scan", "/fabric/networks/live"},
				Notes:            []string{"EVM and Bitcoin have native probe paths", "Move-family dispatch remains fail-closed beyond implemented identity/network evidence"},
			},
			{
				ID: "cross-network-evidence-graph", Layer: "correlation", Domain: "multi-network", Mode: "implemented", EvidenceAuthority: "correlation_only", Activation: "explicit evidence bindings",
				NetworkIDs:       allNetworks,
				DependsOn:        []string{"global-radar-event-plane", "multi-network-native-probes"},
				BackendSurfaces:  []string{"/api/owner/radar/global/records", "/api/owner/radar/global/events"},
				FrontendSurfaces: []string{"/owner-production", "/fabric/security-center"},
				Telemetry:        []string{"koschei.global-radar-relation.v1", "koschei.global-radar-bridge-link.v1", "koschei.global-radar-snapshot.v1"},
				Notes:            []string{"timing, amount or address similarity alone cannot create a verified relation"},
			},
			{
				ID: "network-and-validator-telemetry", Layer: "sensor", Domain: "network-health", Mode: "implemented", EvidenceAuthority: "telemetry_only", Activation: "explicit allowlist and background worker configuration",
				NetworkIDs:      allNetworks,
				BackendSurfaces: []string{"/fabric/networks/deployment", "/fabric/networks/radar"},
				Telemetry:       []string{"Solana validator observations", "EVM execution/beacon telemetry", "Bitcoin Core/PoW telemetry"},
			},
			{
				ID: "actor-defense-network", Layer: "correlation", Domain: "solana-actor-intelligence", Mode: "implemented", EvidenceAuthority: "evidence_bound_rules", Activation: "APP_DATABASE_URL for durable actor memory",
				NetworkIDs:       []string{"solana-mainnet"},
				DependsOn:        []string{"solana-arvis-evidence-and-verdict"},
				BackendSurfaces:  []string{"/api/owner/defense/tracks", "/api/owner/defense/investigate", "/api/owner/defense/distribution"},
				FrontendSurfaces: []string{"/owner-production"},
				Telemetry:        []string{"persistent actor dossier", "funding relations", "rule-band queue"},
			},
			{
				ID: "transaction-preflight-and-state-recheck", Layer: "pre-execution", Domain: "transaction-security", Mode: "implemented", EvidenceAuthority: "evidence_bound_decision", Activation: "network evidence plus customer/owner route gates",
				NetworkIDs:       []string{"solana-mainnet"},
				BackendSurfaces:  []string{"/api/public/transaction-simulate", "/api/v1/shield/preflight", "/api/v1/shield/transaction", "/api/v1/shield/state-recheck"},
				FrontendSurfaces: []string{"/scan"},
			},
			{
				ID: "execution-assurance", Layer: "verification", Domain: "protocol-defense", Mode: "implemented", EvidenceAuthority: "verification_contract", Activation: "authenticated metered or owner access",
				BackendSurfaces: []string{"/api/v1/defense/validation", "/api/v1/execution-assurance/safe/verify", "/api/owner/web3/defense-validation", "/api/owner/web3/execution-assurance/safe/verify"},
				Telemetry:       []string{"defense validation evidence", "Safe execution proof"},
			},
			{
				ID: "defense-os-lab", Layer: "validation", Domain: "defense-research", Mode: "conditional", EvidenceAuthority: "lab_only", Activation: "KOSCHEI_DEFENSE_OS_ENABLED=true plus owner authentication",
				DependsOn:        []string{"execution-assurance"},
				BackendSurfaces:  []string{"/api/owner/defense/artifacts", "/api/owner/defense/knowledge", "/api/owner/defense/lab", "/api/owner/defense/harness-execution", "/api/owner/defense/litesvm-execution"},
				FrontendSurfaces: []string{"/owner-production"},
				Notes:            []string{"conditional capability must not be represented as live when the feature gate is disabled"},
			},
			{
				ID: "durable-evidence-memory", Layer: "storage", Domain: "evidence", Mode: "conditional", EvidenceAuthority: "storage_only", Activation: "APP_DATABASE_URL and/or ClickHouse feature gates",
				DependsOn:       []string{"global-radar-event-plane"},
				BackendSurfaces: []string{"Postgres application store", "ClickHouse Global Radar graph", "ClickHouse canonical event ledger"},
				Telemetry:       []string{"retention", "payload SHA-256 verification", "replay-convergent event identity"},
			},
			{
				ID: "watchlist-alert-and-webhook-plane", Layer: "response", Domain: "customer-defense", Mode: "implemented", EvidenceAuthority: "notification_only", Activation: "APP_DATABASE_URL plus entitlement gates where applicable",
				DependsOn:        []string{"solana-arvis-evidence-and-verdict"},
				BackendSurfaces:  []string{"/api/watchlist", "/api/watchlist/alerts", "/api/webhooks/security-alerts", "/api/webhooks/deliveries"},
				FrontendSurfaces: []string{"/dashboard"},
			},
			{
				ID: "operator-incident-workflow", Layer: "response", Domain: "security-operations", Mode: "implemented", EvidenceAuthority: "operator_workflow_only", Activation: "owner session plus application database",
				DependsOn:        []string{"watchlist-alert-and-webhook-plane", "durable-evidence-memory", "global-radar-coverage-alert-lifecycle"},
				BackendSurfaces:  []string{"/api/owner/incidents", "/api/owner/incidents/"},
				FrontendSurfaces: []string{"/owner-production"},
				Telemetry:        []string{"incident status", "severity", "linked evidence refs", "persisted-alert-backed alert refs", "linked dossier refs", "coverage episode ownership via alert identity", "append-only operator action timeline"},
				Notes:            []string{"new incident alert links must resolve to persisted security_alert_events", "coverage episode ownership is derived from alert identity without changing episode or verdict authority", "operator response state never mutates ARVIS evidence, grades or signatures", "incident state is operational workflow metadata, not a security verdict"},
			},
			{
				ID: "sentinel-fabric-observation", Layer: "model", Domain: "sentinel", Mode: "observe", EvidenceAuthority: "no_web3_decision_authority", Activation: "versioned Fabric contracts only",
				DependsOn:       []string{"global-radar-event-plane"},
				BackendSurfaces: []string{"/fabric", "/fabric/capabilities"},
				Notes:           []string{"Sentinel remains an independent cybersecurity model", "observe-first integration cannot mutate Koschei Web3 security internals"},
			},
			{
				ID: "lang-fabric-policy-proof", Layer: "policy-proof", Domain: "koschei-lang", Mode: "observe", EvidenceAuthority: "independent_project", Activation: "versioned adapter/contracts after compatibility gates",
				BackendSurfaces: []string{"/fabric", "/fabric/capabilities"},
				Notes:           []string{"Koschei Lang owns compiler/runtime/policy/proof/identity internals", "no direct production dependency on Lang internals"},
			},
		},
		AssetBindings: preservedAssetBindings(),
	}
}

func preservedAssetBindings() []AssetBinding {
	return []AssetBinding{
		{Asset: "/js/cipher.js", Kind: "visual-runtime", State: "active", Surface: "/fabric/security-center", Reason: "non-security presentation enhancement; now mounted on the security-center surface"},
		{Asset: "/js/customer-command-universe-v2.js", Kind: "dashboard-runtime", State: "preserved_registered", Surface: "customer workspace compatibility inventory", Reason: "customer workspace v2 explicitly retires this DOM contract; preserved without co-execution so the active workspace contract remains authoritative"},
		{Asset: "/js/early-access-v1.js", Kind: "customer-intake", State: "preserved_registered", Surface: "dedicated intake surface pending", Reason: "requires its own form contract; not injected into unrelated security workflows"},
		{Asset: "/js/evm-spending-intelligence.js", Kind: "security-runtime", State: "active", Surface: "/scan", Reason: "bound to the existing /api/scan/approval EVM allowance and spender-authority evidence route"},
		{Asset: "/js/feedback-button.js", Kind: "operator-feedback", State: "active", Surface: "/fabric/security-center", Reason: "provides direct feedback entry without changing evidence behavior"},
		{Asset: "/js/full-scan-truth-contract.js", Kind: "scan-runtime", State: "preserved_registered", Surface: "scan compatibility inventory", Reason: "kept as a distinct truth-contract module until its DOM contract is promoted without duplicating active scan renderers"},
		{Asset: "/js/koschei-arvis-command-webgl-v1.js", Kind: "visual-runtime", State: "preserved_registered", Surface: "visual compatibility inventory", Reason: "preserved visual generation; not co-executed with newer visual runtimes"},
		{Asset: "/js/koschei-arvis-command-webgl-v2.js", Kind: "visual-runtime", State: "preserved_registered", Surface: "visual compatibility inventory", Reason: "preserved visual generation; activation requires the matching canvas DOM contract"},
		{Asset: "/js/koschei-auth-session-fix.js", Kind: "auth-compatibility", State: "preserved_registered", Surface: "auth compatibility inventory", Reason: "not injected beside current auth runtime until session semantics are proven equivalent"},
		{Asset: "/js/koschei-home-structural-mesh-v1.js", Kind: "visual-runtime", State: "preserved_registered", Surface: "home visual inventory", Reason: "preserved structural visual module"},
		{Asset: "/js/koschei-home-universe-v2.js", Kind: "visual-runtime", State: "preserved_registered", Surface: "home visual inventory", Reason: "preserved cinematic home runtime; current home DOM does not expose its contract"},
		{Asset: "/js/koschei-modern.js", Kind: "visual-runtime", State: "preserved_registered", Surface: "visual compatibility inventory", Reason: "preserved visual/UI runtime"},
		{Asset: "/js/koschei-security-world.js", Kind: "visual-runtime", State: "active", Surface: "/fabric/security-center", Reason: "renders the security-center network/evidence flow canvas"},
		{Asset: "/js/owner-actor-distribution.js", Kind: "owner-security-runtime", State: "active", Surface: "/owner-production", Reason: "adds explicit creator-token distribution follow-up to actor dossiers"},
		{Asset: "/js/owner-actor-intelligence.js", Kind: "owner-security-runtime", State: "preserved_disabled", Surface: "/owner-production inventory", Reason: "file intentionally declares itself disabled; retained while actor intelligence is served by the defense-network runtime"},
		{Asset: "/js/owner-command-center-v2.js", Kind: "owner-runtime", State: "active", Surface: "/owner-production", Reason: "adds command palette and threat-operations navigation"},
		{Asset: "/js/owner-defense-network.js", Kind: "owner-security-runtime", State: "active", Surface: "/owner-production", Reason: "connects persistent actor-defense queue and investigation fallback"},
		{Asset: "/js/owner-unified-radar.js", Kind: "owner-security-runtime", State: "active", Surface: "/owner-production", Reason: "connects the unified ARVIS evidence renderer to the owner radar route"},
		{Asset: "/js/public-solana-scan-tr.js", Kind: "localization-runtime", State: "preserved_registered", Surface: "Turkish scan localization inventory", Reason: "kept separate so the English production surface is not silently rewritten"},
		{Asset: "/js/public-transaction-shield.js", Kind: "scan-compatibility", State: "preserved_registered", Surface: "transaction shield compatibility inventory", Reason: "the current surface uses public-transaction-shield-v2; both submit handlers must not run together"},
	}
}
