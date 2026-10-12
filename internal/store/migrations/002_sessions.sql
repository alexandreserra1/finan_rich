-- Sessões de login para quem acessa o app pela rede (celular, servidor).
-- Guardamos só o hash SHA-256 do token; o token em si fica apenas no cookie.
CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
