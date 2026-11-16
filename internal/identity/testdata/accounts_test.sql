\set ON_ERROR_STOP on
CREATE SCHEMA auth;
CREATE TABLE auth.users (id UUID PRIMARY KEY);
CREATE ROLE rich_test_untrusted NOLOGIN;
INSERT INTO auth.users VALUES
    ('11111111-1111-4111-8111-111111111111'),
    ('22222222-2222-4222-8222-222222222222');
\ir ../migrations/001_accounts.sql

BEGIN;
SET LOCAL ROLE rich_test_untrusted;
DO $$ BEGIN
    BEGIN
        PERFORM * FROM rich.profiles;
        RAISE EXCEPTION 'um papel sem permissão leu perfis';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        PERFORM * FROM rich_private.sessions;
        RAISE EXCEPTION 'um papel sem permissão leu sessões';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
END $$;
COMMIT;

BEGIN;
SET LOCAL ROLE rich_accounts;
SELECT set_config('rich.user_id', '11111111-1111-4111-8111-111111111111', true);
INSERT INTO rich.profiles (user_id, display_name) VALUES ('11111111-1111-4111-8111-111111111111', 'Pessoa A');
COMMIT;

BEGIN;
SET LOCAL ROLE rich_accounts;
SELECT set_config('rich.user_id', '22222222-2222-4222-8222-222222222222', true);
INSERT INTO rich.profiles (user_id, display_name) VALUES ('22222222-2222-4222-8222-222222222222', 'Pessoa B');
DO $$ BEGIN
    IF (SELECT count(*) FROM rich.profiles) <> 1 THEN
        RAISE EXCEPTION 'uma conta leu o perfil de outra';
    END IF;
    UPDATE rich.profiles SET display_name = 'Nome alterado' WHERE user_id = '11111111-1111-4111-8111-111111111111';
    IF FOUND THEN RAISE EXCEPTION 'uma conta alterou o perfil de outra'; END IF;
    BEGIN
        UPDATE rich.profiles SET user_id = '11111111-1111-4111-8111-111111111111';
        RAISE EXCEPTION 'foi possível mudar o dono';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        INSERT INTO rich.profiles (user_id, display_name) VALUES ('33333333-3333-4333-8333-333333333333', 'Perfil indevido');
        RAISE EXCEPTION 'foi possível criar perfil de outro dono';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        PERFORM * FROM rich_private.sessions;
        RAISE EXCEPTION 'o papel de perfis leu sessões';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
END $$;
COMMIT;

BEGIN;
SET LOCAL ROLE rich_accounts;
DO $$ BEGIN
    IF (SELECT count(*) FROM rich.profiles) <> 0 THEN
        RAISE EXCEPTION 'contexto de identidade vazou para outra transação';
    END IF;
END $$;
COMMIT;

BEGIN;
SET LOCAL ROLE rich_sessions;
INSERT INTO rich_private.account_security (user_id) VALUES ('11111111-1111-4111-8111-111111111111');
INSERT INTO rich_private.sessions (user_id, token_hash, provider_session_id, provider_tokens_ciphertext, encryption_key_id, auth_version, authenticated_at, last_verified_at, expires_at)
VALUES ('11111111-1111-4111-8111-111111111111', repeat('a', 64), '44444444-4444-4444-8444-444444444444', decode('abcd', 'hex'), 'chave-falsa-de-teste', 1, now(), now(), now() + interval '1 hour');
DO $$ BEGIN
    BEGIN
        PERFORM * FROM rich.profiles;
        RAISE EXCEPTION 'o papel de sessões leu perfis';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;
    BEGIN
        UPDATE rich_private.sessions SET expires_at = created_at;
        RAISE EXCEPTION 'prazo inválido foi aceito';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
END $$;
COMMIT;

DELETE FROM auth.users WHERE id = '11111111-1111-4111-8111-111111111111';
DO $$ BEGIN
    IF EXISTS (SELECT FROM rich_private.sessions) OR EXISTS (SELECT FROM rich_private.account_security) THEN
        RAISE EXCEPTION 'excluir identidade deixou sessões ativas';
    END IF;
END $$;
SELECT 'Contrato de perfis, papéis e sessões: passou.' AS resultado;
