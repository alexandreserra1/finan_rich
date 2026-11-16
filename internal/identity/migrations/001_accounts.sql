BEGIN;

CREATE ROLE rich_accounts NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
CREATE ROLE rich_sessions NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;

CREATE SCHEMA rich;
CREATE SCHEMA rich_private;
REVOKE ALL ON SCHEMA rich, rich_private FROM PUBLIC;
GRANT USAGE ON SCHEMA rich TO rich_accounts;
GRANT USAGE ON SCHEMA rich_private TO rich_sessions;

CREATE TABLE rich.profiles (
    user_id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 80 AND display_name = btrim(display_name)),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE rich.profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE rich.profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY profile_owner ON rich.profiles TO rich_accounts
    USING (user_id = nullif(current_setting('rich.user_id', true), '')::uuid)
    WITH CHECK (user_id = nullif(current_setting('rich.user_id', true), '')::uuid);
GRANT SELECT, INSERT ON rich.profiles TO rich_accounts;
GRANT UPDATE (display_name) ON rich.profiles TO rich_accounts;

CREATE TABLE rich_private.account_security (
    user_id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    auth_version BIGINT NOT NULL DEFAULT 1 CHECK (auth_version > 0),
    blocked_at TIMESTAMPTZ
);

CREATE TABLE rich_private.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES rich_private.account_security(user_id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    provider_session_id UUID NOT NULL,
    provider_tokens_ciphertext BYTEA NOT NULL CHECK (octet_length(provider_tokens_ciphertext) > 0),
    encryption_key_id TEXT NOT NULL CHECK (char_length(encryption_key_id) BETWEEN 1 AND 100),
    auth_version BIGINT NOT NULL CHECK (auth_version > 0),
    authenticated_at TIMESTAMPTZ NOT NULL,
    last_verified_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (expires_at > created_at),
    CHECK (last_seen_at >= created_at),
    CHECK (authenticated_at <= created_at),
    CHECK (last_verified_at >= authenticated_at)
);

CREATE INDEX sessions_user ON rich_private.sessions (user_id);
CREATE INDEX sessions_expiry ON rich_private.sessions (expires_at) WHERE revoked_at IS NULL;

CREATE TABLE rich_private.consents (
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    document TEXT NOT NULL CHECK (document IN ('terms', 'privacy', 'cloud-sync')),
    version TEXT NOT NULL CHECK (char_length(version) BETWEEN 1 AND 100),
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    withdrawn_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, document, version),
    CHECK (withdrawn_at IS NULL OR withdrawn_at >= accepted_at)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA rich_private TO rich_sessions;
REVOKE ALL ON ALL TABLES IN SCHEMA rich, rich_private FROM PUBLIC;

COMMIT;
