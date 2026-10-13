-- Notas (versão 1.3): novo tipo de registro sincronizado.
--
-- O SQLite não altera um CHECK existente, então a tabela é recriada com a
-- lista nova de tipos e os dados são copiados (mesmos ids e revisões).
CREATE TABLE records_new (
    type       TEXT    NOT NULL CHECK (type IN ('category', 'transaction', 'debt', 'pref', 'note')),
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
