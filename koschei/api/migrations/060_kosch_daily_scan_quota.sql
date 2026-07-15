CREATE TABLE IF NOT EXISTS kosch_daily_quota_usage (
    auth_subject text NOT NULL,
    quota_date date NOT NULL,
    tier text NOT NULL CHECK (tier IN ('basic','pro','enterprise')),
    quota_limit integer NOT NULL CHECK (quota_limit > 0),
    used_count integer NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (auth_subject, quota_date)
);

CREATE TABLE IF NOT EXISTS kosch_daily_quota_reservations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    auth_subject text NOT NULL,
    quota_date date NOT NULL,
    tier text NOT NULL CHECK (tier IN ('basic','pro','enterprise')),
    quota_limit integer NOT NULL CHECK (quota_limit > 0),
    reason text NOT NULL,
    status text NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved','consumed','refunded')),
    created_at timestamptz NOT NULL DEFAULT now(),
    finalized_at timestamptz,
    FOREIGN KEY (auth_subject, quota_date)
        REFERENCES kosch_daily_quota_usage(auth_subject, quota_date)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_kosch_daily_quota_usage_date
    ON kosch_daily_quota_usage(quota_date, auth_subject);

CREATE INDEX IF NOT EXISTS idx_kosch_daily_quota_reservations_subject_status
    ON kosch_daily_quota_reservations(auth_subject, quota_date, status, created_at DESC);
