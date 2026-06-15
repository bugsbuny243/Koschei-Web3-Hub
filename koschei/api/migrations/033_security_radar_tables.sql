CREATE TABLE IF NOT EXISTS security_radar_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT 'token',
    module_id TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (target, COALESCE(module_id, ''))
);

CREATE TABLE IF NOT EXISTS security_radar_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id TEXT NOT NULL,
    name TEXT NOT NULL,
    target TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT 'program',
    provider TEXT NOT NULL DEFAULT 'alchemy',
    watch_mode TEXT NOT NULL DEFAULT 'polling',
    last_seen_signature TEXT,
    last_seen_slot BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (module_id, target)
);

CREATE TABLE IF NOT EXISTS security_radar_seen_signatures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id TEXT NOT NULL,
    source_target TEXT NOT NULL,
    signature TEXT NOT NULL,
    slot BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (module_id, signature)
);

CREATE TABLE IF NOT EXISTS security_radar_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT 'token',
    module_id TEXT NOT NULL,
    source TEXT,
    signature TEXT,
    risk_index INTEGER,
    risk_level TEXT,
    grade TEXT,
    verdict TEXT,
    recommendation TEXT,
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    signals JSONB NOT NULL DEFAULT '{}'::jsonb,
    rule_version TEXT NOT NULL DEFAULT 'koschei-security-radar-v1',
    slot BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (module_id, signature, source)
);

CREATE TABLE IF NOT EXISTS security_radar_verdicts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT 'token',
    module_id TEXT NOT NULL,
    signature TEXT NOT NULL,
    risk_index INTEGER NOT NULL,
    risk_level TEXT NOT NULL,
    grade TEXT NOT NULL,
    verdict TEXT NOT NULL,
    recommendation TEXT NOT NULL,
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    signals JSONB NOT NULL DEFAULT '{}'::jsonb,
    rule_version TEXT NOT NULL DEFAULT 'koschei-security-radar-v1',
    user_id TEXT,
    source TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (signature, module_id)
);

CREATE INDEX IF NOT EXISTS idx_security_radar_verdicts_created_at ON security_radar_verdicts (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_radar_verdicts_risk ON security_radar_verdicts (risk_level, risk_index DESC);
CREATE INDEX IF NOT EXISTS idx_security_radar_events_module_created ON security_radar_events (module_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_security_radar_seen_signature ON security_radar_seen_signatures (signature);
