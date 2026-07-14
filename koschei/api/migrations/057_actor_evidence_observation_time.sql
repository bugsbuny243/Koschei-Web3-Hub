-- Canonical architecture: ACTOR_INVESTIGATION_ENGINE.md section 4.
-- A live evidence row must retain the transaction timestamp. Column defaults
-- must never replace it with the migration/insert wall-clock time.
CREATE OR REPLACE FUNCTION normalize_security_actor_evidence_line()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    relation_value text;
    metadata_program text;
    metadata_destination text;
BEGIN
    NEW.actor_wallet := btrim(COALESCE(NEW.actor_wallet,''));
    NEW.counterpart_kind := btrim(COALESCE(NEW.counterpart_kind,''));
    NEW.counterpart_id := btrim(COALESCE(NEW.counterpart_id,''));
    NEW.relation := btrim(COALESCE(NEW.relation,''));
    NEW.actor_role := btrim(COALESCE(NULLIF(NEW.actor_role,''),NULLIF(NEW.metadata->>'actor_role',''),'actor'));
    NEW.source_wallet := btrim(COALESCE(NEW.source_wallet,''));
    NEW.destination_wallet := btrim(COALESCE(NEW.destination_wallet,''));
    NEW.program := btrim(COALESCE(NEW.program,''));
    relation_value := lower(NEW.relation);
    metadata_program := btrim(COALESCE(NEW.metadata->>'program',''));
    metadata_destination := btrim(COALESCE(
        NULLIF(NEW.metadata->>'destination_wallet',''),
        NULLIF(NEW.metadata->>'pool_wallet',''),
        NULLIF(NEW.metadata->>'pool_account',''),
        ''
    ));

    IF relation_value IN ('direct_sol_transfer_out','direct_token_transfer_out') THEN
        NEW.source_wallet := COALESCE(NULLIF(NEW.source_wallet,''),NEW.actor_wallet);
        NEW.destination_wallet := COALESCE(NULLIF(NEW.destination_wallet,''),NEW.counterpart_id);
    ELSIF relation_value IN ('direct_sol_transfer_in','direct_token_transfer_in') THEN
        NEW.source_wallet := COALESCE(NULLIF(NEW.source_wallet,''),NEW.counterpart_id);
        NEW.destination_wallet := COALESCE(NULLIF(NEW.destination_wallet,''),NEW.actor_wallet);
    ELSIF relation_value='liquidity_remove_activity' THEN
        NEW.source_wallet := COALESCE(NULLIF(NEW.source_wallet,''),NEW.actor_wallet);
        NEW.destination_wallet := COALESCE(NULLIF(NEW.destination_wallet,''),metadata_destination);
    END IF;

    IF NEW.program='' THEN
        NEW.program := CASE
            WHEN relation_value IN ('direct_sol_transfer_in','direct_sol_transfer_out') THEN 'system'
            WHEN relation_value IN ('direct_token_transfer_in','direct_token_transfer_out','dominant_holder_of') THEN 'spl-token'
            WHEN relation_value='created_token' THEN COALESCE(NULLIF(metadata_program,''),'pump.fun')
            ELSE metadata_program
        END;
    END IF;

    IF TG_OP='INSERT' THEN
        IF COALESCE(NEW.occurrence_count,1)<=1 THEN
            NEW.first_observed_at := NEW.observed_at;
            NEW.last_observed_at := NEW.observed_at;
        ELSE
            NEW.first_observed_at := COALESCE(NEW.first_observed_at,NEW.observed_at);
            NEW.last_observed_at := COALESCE(NEW.last_observed_at,NEW.observed_at);
        END IF;
    ELSE
        NEW.first_observed_at := LEAST(
            COALESCE(OLD.first_observed_at,OLD.observed_at),
            COALESCE(NEW.first_observed_at,NEW.observed_at)
        );
        NEW.last_observed_at := GREATEST(
            COALESCE(OLD.last_observed_at,OLD.observed_at),
            COALESCE(NEW.last_observed_at,NEW.observed_at),
            NEW.observed_at
        );
    END IF;
    NEW.observed_at := NEW.last_observed_at;

    IF relation_value='liquidity_remove_activity'
       AND NEW.verification_status='verified'
       AND (
            NEW.source_wallet='' OR NEW.destination_wallet='' OR NEW.program='' OR
            NEW.signature IS NULL OR btrim(NEW.signature)='' OR NEW.slot IS NULL OR NEW.slot<=0 OR
            lower(COALESCE(NEW.metadata->>'actor_signed','false'))<>'true' OR
            lower(COALESCE(NEW.metadata->>'creator_role_observed','false'))<>'true' OR
            COALESCE(NEW.token_mint,'')=''
       ) THEN
        NEW.verification_status := 'observed';
        NEW.metadata := COALESCE(NEW.metadata,'{}'::jsonb) || jsonb_build_object(
            'verification_downgrade_reason',
            'liquidity removal lacks a complete signer, creator-linked mint, pool destination or program evidence line'
        );
    END IF;
    RETURN NEW;
END;
$$;
