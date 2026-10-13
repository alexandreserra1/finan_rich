-- Metas e lançamentos recorrentes (versão 1.5).
--
-- Recria records sem a lista fixa de tipos no CHECK: quem valida o tipo é o
-- domain.DecodeRecord (no servidor e no aparelho). Assim, os próximos tipos
-- de registro não precisam recriar a tabela.
CREATE TABLE records_new (
    type       TEXT    NOT NULL CHECK (type <> ''),
    id         TEXT    NOT NULL,
    data       TEXT    NOT NULL CHECK (json_valid(data)),
    updated_at INTEGER NOT NULL,
    deleted    INTEGER NOT NULL DEFAULT 0,
    rev        INTEGER NOT NULL,
    PRIMARY KEY (type, id)
);
INSERT INTO records_new (type, id, data, updated_at, deleted, rev)
    SELECT type, id, data, updated_at, deleted, rev FROM records;
DROP TABLE records;
ALTER TABLE records_new RENAME TO records;
CREATE INDEX records_rev_idx ON records (rev);
