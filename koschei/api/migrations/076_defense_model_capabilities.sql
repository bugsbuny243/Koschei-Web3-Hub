CREATE TABLE IF NOT EXISTS defense_model_capability_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    capability_ref text NOT NULL UNIQUE,
    provider text NOT NULL,
    model text NOT NULL,
    model_role text NOT NULL,
    endpoint text NOT NULL,
    available boolean NOT NULL DEFAULT false,
    structured_output_supported boolean NOT NULL DEFAULT false,
    tool_calling_supported boolean NOT NULL DEFAULT false,
    basic_latency_ms integer NOT NULL DEFAULT 0,
    structured_latency_ms integer NOT NULL DEFAULT 0,
    tool_latency_ms integer NOT NULL DEFAULT 0,
    status text NOT NULL,
    basic_result jsonb NOT NULL DEFAULT '{}'::jsonb,
    structured_result jsonb NOT NULL DEFAULT '{}'::jsonb,
    tool_result jsonb NOT NULL DEFAULT '{}'::jsonb,
    limitations jsonb NOT NULL DEFAULT '[]'::jsonb,
    capability_hash text NOT NULL,
    verdict_authority boolean NOT NULL DEFAULT false,
    observed_at timestamptz NOT NULL DEFAULT now(),
    created_by text NOT NULL DEFAULT 'owner',
    CONSTRAINT defense_model_capability_ref_format CHECK (capability_ref ~ '^KMC1-[0-9a-f]{32}$'),
    CONSTRAINT defense_model_capability_provider_check CHECK (provider IN ('together')),
    CONSTRAINT defense_model_capability_role_check CHECK (model_role IN (
        'general','lead_prosecutor','evidence_prosecutor','tribunal_qwen','tribunal_glm','defense_engineer','custom'
    )),
    CONSTRAINT defense_model_capability_status_check CHECK (status IN ('passed','partial','failed')),
    CONSTRAINT defense_model_capability_latency_check CHECK (basic_latency_ms >= 0 AND structured_latency_ms >= 0 AND tool_latency_ms >= 0),
    CONSTRAINT defense_model_capability_basic_object CHECK (jsonb_typeof(basic_result) = 'object'),
    CONSTRAINT defense_model_capability_structured_object CHECK (jsonb_typeof(structured_result) = 'object'),
    CONSTRAINT defense_model_capability_tool_object CHECK (jsonb_typeof(tool_result) = 'object'),
    CONSTRAINT defense_model_capability_limitations_array CHECK (jsonb_typeof(limitations) = 'array'),
    CONSTRAINT defense_model_capability_hash_format CHECK (capability_hash ~ '^sha256:[0-9a-f]{64}$'),
    CONSTRAINT defense_model_capability_nonempty CHECK (btrim(model) <> '' AND btrim(model_role) <> '' AND btrim(endpoint) <> ''),
    CONSTRAINT defense_model_capability_non_authoritative CHECK (verdict_authority = false)
);

CREATE INDEX IF NOT EXISTS defense_model_capability_model_idx
    ON defense_model_capability_snapshots (provider, model, observed_at DESC);
CREATE INDEX IF NOT EXISTS defense_model_capability_role_idx
    ON defense_model_capability_snapshots (model_role, observed_at DESC);
CREATE INDEX IF NOT EXISTS defense_model_capability_status_idx
    ON defense_model_capability_snapshots (status, observed_at DESC);

DROP TRIGGER IF EXISTS defense_model_capability_snapshots_immutable ON defense_model_capability_snapshots;
CREATE TRIGGER defense_model_capability_snapshots_immutable
BEFORE UPDATE OR DELETE ON defense_model_capability_snapshots
FOR EACH ROW EXECUTE FUNCTION reject_defense_runtime_mutation();
