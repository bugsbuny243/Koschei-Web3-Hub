-- Canonical stream conclusions are separate from 14 observational ARVIS arms.
-- No legacy security_radar_verdicts signature is accepted as a unified proof.
-- One immutable, independently verifiable signed decision per raw stream event.
CREATE TABLE IF NOT EXISTS arvis_stream_canonical_finals (
    -- No cascading FK: raw stream retention may expire before signed evidence.
    -- The writer locks and authenticates the source ledger row before insert.
    stream_event_id uuid PRIMARY KEY,
    target text NOT NULL,
    network text NOT NULL,
    source_signature text NOT NULL,
    source_module text NOT NULL,
    verdict_payload jsonb NOT NULL,
    payload_hash text NOT NULL CHECK (payload_hash ~ '^sha256:[0-9a-f]{64}$'),
    key_id text NOT NULL CHECK (btrim(key_id) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT arvis_stream_canonical_final_envelope_shape CHECK (
        jsonb_typeof(verdict_payload) = 'object' AND
        jsonb_typeof(verdict_payload->'verdict') = 'object' AND
        verdict_payload->>'target' = target AND
        verdict_payload->>'network' = network AND
        verdict_payload->>'payload_hash' = payload_hash AND
        verdict_payload->>'key_id' = key_id AND
        verdict_payload->>'signature_algorithm' = 'ed25519' AND
        verdict_payload->'verdict'->>'signed' = 'true'
    )
);
CREATE INDEX IF NOT EXISTS arvis_stream_canonical_finals_target_idx
    ON arvis_stream_canonical_finals(network,target,created_at DESC);

-- Prevent rewriting/deleting authenticated conclusions. Per-event idempotent
-- retries use INSERT ... ON CONFLICT DO NOTHING and independently verify readback.
CREATE OR REPLACE FUNCTION reject_arvis_stream_canonical_final_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'ARVIS canonical stream finals are immutable';
END;
$$;
DROP TRIGGER IF EXISTS arvis_stream_canonical_finals_immutable ON arvis_stream_canonical_finals;
CREATE TRIGGER arvis_stream_canonical_finals_immutable
BEFORE UPDATE OR DELETE ON arvis_stream_canonical_finals
FOR EACH ROW EXECUTE FUNCTION reject_arvis_stream_canonical_final_mutation();
