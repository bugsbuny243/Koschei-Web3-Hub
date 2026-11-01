-- Bounded, opt-in Solana authority-change observations. Not SOC incidents,
-- signed verdict receipts, or a general-purpose raw blockchain journal.
CREATE TABLE IF NOT EXISTS security_soc_pb02_watch_cursors (
    network text NOT NULL CHECK (network = 'solana-mainnet'),
    watch_address text NOT NULL,
    watermark_signature text NOT NULL,
    watermark_slot bigint NOT NULL CHECK (watermark_slot > 0),
    last_checked_at timestamptz NOT NULL DEFAULT now(),
    bootstrap_only boolean NOT NULL DEFAULT true,
    PRIMARY KEY (network, watch_address)
);

CREATE TABLE IF NOT EXISTS security_soc_pb02_authority_observations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    network text NOT NULL CHECK (network = 'solana-mainnet'),
    watch_address text NOT NULL,
    signature text NOT NULL,
    slot bigint NOT NULL CHECK (slot > 0),
    instruction_source text NOT NULL CHECK (instruction_source IN ('outer','inner')),
    instruction_index integer NOT NULL CHECK (instruction_index >= 0),
    inner_sequence integer NOT NULL CHECK (inner_sequence >= 0),
    program_id text NOT NULL,
    event_kind text NOT NULL,
    authority_type text NOT NULL DEFAULT '',
    new_authority text NOT NULL DEFAULT '',
    evidence_state text NOT NULL DEFAULT 'confirmed_json_parsed_unpinned',
    finding_state text NOT NULL DEFAULT 'authority_mutation_observed_not_attack_proof',
    severity text NOT NULL DEFAULT 'high',
    recommended_action text NOT NULL DEFAULT 'owner_review_authorization',
    enforcement_state text NOT NULL DEFAULT 'not_connected',
    report_integrity text NOT NULL DEFAULT 'unsigned_rpc_observation',
    observed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(network, watch_address, signature, instruction_source, instruction_index, inner_sequence)
);

CREATE INDEX IF NOT EXISTS security_soc_pb02_authority_observations_recent_idx
ON security_soc_pb02_authority_observations (observed_at DESC);
