\set ON_ERROR_STOP on
\ir ../migrations/002_rate_limits.sql

BEGIN;
SET LOCAL ROLE rich_ratelimit;
DO $$
DECLARE result RECORD; attempt INTEGER;
BEGIN
    FOR attempt IN 1..5 LOOP
        SELECT * INTO result FROM rich_private.consume_rate_limit('auth-login-account', repeat('a', 64));
        IF NOT result.allowed OR result.remaining <> 5 - attempt OR result.retry_after <> 0 THEN
            RAISE EXCEPTION 'rajada não respeitou a capacidade';
        END IF;
    END LOOP;
    SELECT * INTO result FROM rich_private.consume_rate_limit('auth-login-account', repeat('a', 64));
    IF result.allowed OR result.remaining <> 0 OR result.retry_after NOT BETWEEN 1 AND 12 THEN
        RAISE EXCEPTION 'balde vazio não recusou com prazo válido';
    END IF;
    BEGIN
        PERFORM * FROM rich_private.rate_buckets;
        RAISE EXCEPTION 'papel do limitador leu sujeitos';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        PERFORM * FROM rich_private.sessions;
        RAISE EXCEPTION 'papel do limitador leu sessões';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        PERFORM * FROM rich_private.consume_rate_limit('politica-inexistente', repeat('b', 64));
        RAISE EXCEPTION 'política inexistente foi aceita';
    EXCEPTION WHEN invalid_parameter_value THEN NULL;
    END;
    BEGIN
        PERFORM * FROM rich_private.consume_rate_limit('auth-login-account', 'email-ou-ip-em-texto');
        RAISE EXCEPTION 'sujeito sem hash foi aceito';
    EXCEPTION WHEN invalid_parameter_value THEN NULL;
    END;
END $$;
COMMIT;

UPDATE rich_private.rate_buckets SET tokens = 0, updated_at = clock_timestamp() - interval '6 seconds';
DO $$ DECLARE result RECORD; BEGIN
    SELECT * INTO result FROM rich_private.consume_rate_limit('auth-login-account', repeat('a', 64));
    IF result.allowed OR result.retry_after NOT BETWEEN 1 AND 6 THEN
        RAISE EXCEPTION 'recarga fracionada falhou';
    END IF;
END $$;
UPDATE rich_private.rate_buckets SET tokens = 0, updated_at = clock_timestamp() - interval '12 seconds';
DO $$ DECLARE result RECORD; BEGIN
    SELECT * INTO result FROM rich_private.consume_rate_limit('auth-login-account', repeat('a', 64));
    IF NOT result.allowed OR result.remaining <> 0 THEN RAISE EXCEPTION 'recarga de um token falhou'; END IF;
END $$;
UPDATE rich_private.rate_buckets SET tokens = 0, updated_at = clock_timestamp() - interval '1 day';
DO $$ DECLARE result RECORD; BEGIN
    SELECT * INTO result FROM rich_private.consume_rate_limit('auth-login-account', repeat('a', 64));
    IF NOT result.allowed OR result.remaining <> 4 THEN RAISE EXCEPTION 'recarga ultrapassou a capacidade'; END IF;
END $$;
UPDATE rich_private.rate_buckets SET tokens = 0, updated_at = clock_timestamp() + interval '1 hour';
DO $$ DECLARE result RECORD; BEGIN
    SELECT * INTO result FROM rich_private.consume_rate_limit('auth-login-account', repeat('a', 64));
    IF result.allowed OR result.retry_after NOT BETWEEN 3600 AND 3612 OR EXISTS (SELECT FROM rich_private.rate_buckets WHERE updated_at < clock_timestamp()) THEN
        RAISE EXCEPTION 'recuo do relógio recarregou ou recuou o balde';
    END IF;
END $$;

BEGIN;
SET LOCAL ROLE rich_test_untrusted;
DO $$ BEGIN
    BEGIN
        PERFORM * FROM rich_private.consume_rate_limit('auth-login-account', repeat('a', 64));
        RAISE EXCEPTION 'papel sem permissão consumiu limite';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
END $$;
COMMIT;

INSERT INTO rich_private.rate_policies VALUES ('test-concurrency', 5, 3600);
SELECT 'Contrato de token bucket PostgreSQL: passou.' AS resultado;
