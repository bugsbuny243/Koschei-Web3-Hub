-- A deliberately bounded, independently gated SOLANA authority watch.
-- Separate from the 21M-row raw WSS journal; no existing data is deleted.
CREATE TABLE IF NOT EXISTS security_solana_authority_watch_cursors (
    watch_address text PRIMARY KEY,
    watermark_signature text NOT NULL DEFAULT '',
    watermark_slot bigint NOT NULL DEFAULT 0,
    status text NOT NULL DEFAULT 'uninitialized',
    last_error text NOT NULL DEFAULT '',
    last_polled_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT authority_watch_cursor_status CHECK (
        status IN ('uninitialized','caught_up','bootstrap_window_only','gap_unresolved','rpc_error')
    )
);

CREATE TABLE IF NOT EXISTS security_solana_authority_watch_observations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    watch_address text NOT NULL,
    signature text NOT NULL,
    slot bigint NOT NULL CHECK (slot > 0),
    instruction_scope text NOT NULL CHECK (instruction_scope IN ('outer','cpi')),
    instruction_index integer NOT NULL CHECK (instruction_index >= 0),
    inner_index integer NOT NULL CHECK (inner_index >= 0),
    program_id text NOT NULL,
    authority_account text NOT NULL,
    authority_type text NOT NULL,
    new_authority text NOT NULL,
    finding_state text NOT NULL DEFAULT 'confirmed_authority_change_observed',
    severity text NOT NULL CHECK (severity IN ('medium','high','critical')),
    evidence_state text NOT NULL DEFAULT 'confirmed_executed_transaction',
    enforcement_state text NOT NULL DEFAULT 'not_connected',
    report_integrity text NOT NULL DEFAULT 'unsigned_rpc_observation',
    transaction_sha256 text NOT NULL CHECK (length(transaction_sha256)=64),
    observed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (watch_address,signature,instruction_scope,instruction_index,inner_index)
);
CREATE INDEX IF NOT EXISTS idx_solana_authority_watch_observations_recent
    ON security_solana_authority_watch_observations (observed_at DESC, watch_address);
COMMENT ON TABLE security_solana_authority_watch_observations IS
    'Opt-in exact-account Solana confirmed transaction authority-change observations; not auto-created SOC incidents or signed final verdicts.';
