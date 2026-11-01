-- Koschei-Web3-Hub shadow-only action ledger, STAGING MANUAL INSTALL ONLY.
-- This file is intentionally outside koschei/api/migrations and is NOT run
-- by the application or its automatic migration runner.
-- A database administrator must review/install it in an isolated database.
-- No provider execution, on-chain action or privileged network operation exists.
BEGIN;

CREATE TABLE IF NOT EXISTS koschei_active_response_shadow_plans (
    tenant_id       text NOT NULL CHECK (length(tenant_id) BETWEEN 1 AND 128),
    action_id       text NOT NULL CHECK (length(action_id) BETWEEN 1 AND 128),
    target_id       text NOT NULL CHECK (length(target_id) BETWEEN 1 AND 128),
    provider_id     text NOT NULL CHECK (length(provider_id) BETWEEN 1 AND 128),
    plan_digest     text NOT NULL CHECK (plan_digest ~ '^sha256:[0-9a-f]{64}$'),
    chain_hash      text NOT NULL CHECK (chain_hash ~ '^sha256:[0-9a-f]{64}$'),
    recorded_at     timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, action_id),
    UNIQUE (tenant_id, plan_digest),
    UNIQUE (tenant_id, chain_hash)
);

CREATE TABLE IF NOT EXISTS koschei_active_response_shadow_audit (
    sequence_id     bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       text NOT NULL CHECK (length(tenant_id) BETWEEN 1 AND 128),
    action_id       text NOT NULL CHECK (length(action_id) BETWEEN 1 AND 128),
    plan_digest     text NOT NULL CHECK (plan_digest ~ '^sha256:[0-9a-f]{64}$'),
    previous_hash   text NOT NULL CHECK (previous_hash ~ '^sha256:[0-9a-f]{64}$'),
    chain_hash      text NOT NULL CHECK (chain_hash ~ '^sha256:[0-9a-f]{64}$'),
    recorded_at     timestamptz NOT NULL,
    UNIQUE (tenant_id, action_id),
    UNIQUE (tenant_id, chain_hash),
    FOREIGN KEY (tenant_id, action_id)
        REFERENCES koschei_active_response_shadow_plans (tenant_id, action_id)
);

CREATE INDEX IF NOT EXISTS idx_koschei_active_response_shadow_audit_tenant_seq
    ON koschei_active_response_shadow_audit (tenant_id, sequence_id DESC);

-- Defense in depth: even privileged application role UPDATE/DELETE attempts
-- are blocked until an administrator explicitly changes the table's policy.
-- This is not proof against the table owner or a database superuser.
CREATE OR REPLACE FUNCTION koschei_reject_shadow_ledger_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'shadow ledger rows are append-only; mutation rejected';
END
$$;

DROP TRIGGER IF EXISTS trg_shadow_action_immutable ON koschei_active_response_shadow_plans;
CREATE TRIGGER trg_shadow_action_immutable
    BEFORE UPDATE OR DELETE ON koschei_active_response_shadow_plans
    FOR EACH ROW EXECUTE FUNCTION koschei_reject_shadow_ledger_mutation();

DROP TRIGGER IF EXISTS trg_shadow_audit_immutable ON koschei_active_response_shadow_audit;
CREATE TRIGGER trg_shadow_audit_immutable
    BEFORE UPDATE OR DELETE ON koschei_active_response_shadow_audit
    FOR EACH ROW EXECUTE FUNCTION koschei_reject_shadow_ledger_mutation();

COMMIT;
