BEGIN;

CREATE ROLE rich_ratelimit NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
GRANT USAGE ON SCHEMA rich_private TO rich_ratelimit;

CREATE TABLE rich_private.rate_policies (
    scope TEXT PRIMARY KEY CHECK (scope ~ '^[a-z][a-z0-9-]{0,63}$'),
    capacity INTEGER NOT NULL CHECK (capacity BETWEEN 1 AND 10000),
    refill_seconds NUMERIC NOT NULL CHECK (refill_seconds BETWEEN 0.001 AND 86400)
);

INSERT INTO rich_private.rate_policies (scope, capacity, refill_seconds) VALUES
    ('auth-login-ip', 10, 6),
    ('auth-login-account', 5, 12),
    ('auth-signup-ip', 5, 720),
    ('auth-reset-ip', 5, 720),
    ('auth-reset-account', 3, 1200),
    ('ai-user', 3, 240),
    ('ai-ip', 10, 120),
    ('ai-global', 20, 60),
    ('sync-user', 10, 2);

CREATE TABLE rich_private.rate_buckets (
    scope TEXT NOT NULL REFERENCES rich_private.rate_policies(scope),
    subject_hash TEXT NOT NULL CHECK (subject_hash ~ '^[0-9a-f]{64}$'),
    tokens NUMERIC NOT NULL CHECK (tokens >= 0),
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (scope, subject_hash)
);
CREATE INDEX rate_buckets_updated ON rich_private.rate_buckets (updated_at);
REVOKE ALL ON rich_private.rate_policies, rich_private.rate_buckets FROM PUBLIC;

CREATE FUNCTION rich_private.consume_rate_limit(request_scope TEXT, request_subject_hash TEXT)
RETURNS TABLE (allowed BOOLEAN, remaining INTEGER, retry_after INTEGER)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog
AS $$
DECLARE
    policy_capacity INTEGER;
    policy_refill NUMERIC;
    current_tokens NUMERIC;
    previous_update TIMESTAMPTZ;
    current_time_value TIMESTAMPTZ;
BEGIN
    IF request_scope IS NULL OR request_subject_hash IS NULL OR request_subject_hash !~ '^[0-9a-f]{64}$' THEN
        RAISE EXCEPTION 'identificação de limite inválida' USING ERRCODE = '22023';
    END IF;
    SELECT capacity, refill_seconds INTO policy_capacity, policy_refill
    FROM rich_private.rate_policies WHERE scope = request_scope FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'política de limite desconhecida' USING ERRCODE = '22023';
    END IF;
    INSERT INTO rich_private.rate_buckets (scope, subject_hash, tokens, updated_at)
    VALUES (request_scope, request_subject_hash, policy_capacity, clock_timestamp())
    ON CONFLICT DO NOTHING;
    SELECT tokens, updated_at INTO current_tokens, previous_update
    FROM rich_private.rate_buckets
    WHERE scope = request_scope AND subject_hash = request_subject_hash FOR UPDATE;
    current_time_value := clock_timestamp();
    current_tokens := least(policy_capacity, current_tokens + greatest(0, extract(epoch FROM current_time_value - previous_update)) / policy_refill);
    allowed := current_tokens >= 1;
    IF allowed THEN current_tokens := current_tokens - 1; END IF;
    remaining := floor(current_tokens)::integer;
    retry_after := CASE WHEN allowed THEN 0 ELSE greatest(1, ceil(greatest(0, extract(epoch FROM previous_update - current_time_value)) + (1 - current_tokens) * policy_refill)::integer) END;
    UPDATE rich_private.rate_buckets SET tokens = current_tokens, updated_at = greatest(previous_update, current_time_value)
    WHERE scope = request_scope AND subject_hash = request_subject_hash;
    RETURN NEXT;
END $$;

REVOKE ALL ON FUNCTION rich_private.consume_rate_limit(TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION rich_private.consume_rate_limit(TEXT, TEXT) TO rich_ratelimit;

COMMIT;
