-- Aparelhos pareados (app do celular). Cada um tem um token próprio, enviado
-- no cabeçalho Authorization; guardamos só o hash SHA-256 dele.
CREATE TABLE devices (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TEXT
);
