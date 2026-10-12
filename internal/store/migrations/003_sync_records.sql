-- Sincronização entre aparelhos (versão 1.1).
--
-- Categorias, lançamentos, dívidas e preferências viram documentos JSON numa
-- tabela única. Cada aparelho guarda a mesma estrutura localmente e a
-- sincronização troca só o que mudou:
--   - id: UUID criado por quem criou o registro (funciona offline);
--   - updated_at: hora da última alteração (ms); em conflito, vence a maior;
--   - deleted: apagar é marcar, para o apagamento chegar aos outros aparelhos;
--   - rev: contador global do servidor; o aparelho pede "o que mudou depois da rev N".

CREATE TABLE records (
    type       TEXT    NOT NULL CHECK (type IN ('category', 'transaction', 'debt', 'pref')),
    id         TEXT    NOT NULL,
    data       TEXT    NOT NULL CHECK (json_valid(data)),
    updated_at INTEGER NOT NULL,
    deleted    INTEGER NOT NULL DEFAULT 0,
    rev        INTEGER NOT NULL,
    PRIMARY KEY (type, id)
);
CREATE INDEX records_rev_idx ON records (rev);

CREATE TABLE sync_state (
    id  INTEGER PRIMARY KEY CHECK (id = 1),
    rev INTEGER NOT NULL
);
INSERT INTO sync_state (id, rev) VALUES (1, 1);

-- Converte os dados da versão 1.0. Os ids antigos (números) ganham um prefixo
-- e continuam únicos; registros novos usam UUID.
INSERT INTO records (type, id, data, updated_at, rev)
SELECT 'category', 'cat-' || id,
       json_object('id', 'cat-' || id, 'updatedAt', CAST(strftime('%s', 'now') AS INTEGER) * 1000,
                   'name', name, 'bucket', bucket),
       CAST(strftime('%s', 'now') AS INTEGER) * 1000, 1
FROM categories;

INSERT INTO records (type, id, data, updated_at, rev)
SELECT 'transaction', 'tx-' || id,
       json_object('id', 'tx-' || id, 'updatedAt', CAST(strftime('%s', 'now') AS INTEGER) * 1000,
                   'kind', kind, 'description', description, 'amountCents', amount_cents,
                   'categoryId', 'cat-' || category_id, 'date', occurred_on),
       CAST(strftime('%s', 'now') AS INTEGER) * 1000, 1
FROM transactions;

INSERT INTO records (type, id, data, updated_at, rev)
SELECT 'debt', 'debt-' || id,
       json_object('id', 'debt-' || id, 'updatedAt', CAST(strftime('%s', 'now') AS INTEGER) * 1000,
                   'name', name, 'creditor', creditor, 'balanceCents', balance_cents,
                   'monthlyRateBp', monthly_rate_bp, 'minPaymentCents', min_payment_cents,
                   'dueDay', COALESCE(due_day, 0)),
       CAST(strftime('%s', 'now') AS INTEGER) * 1000, 1
FROM debts;

INSERT INTO records (type, id, data, updated_at, rev)
SELECT 'pref', 'split',
       json_object('id', 'split', 'updatedAt', CAST(strftime('%s', 'now') AS INTEGER) * 1000,
                   'value', json_object(
                       'necessidades', CAST((SELECT value FROM settings WHERE key = 'pct_necessidades') AS INTEGER),
                       'desejos', CAST((SELECT value FROM settings WHERE key = 'pct_desejos') AS INTEGER),
                       'investimentos', CAST((SELECT value FROM settings WHERE key = 'pct_investimentos') AS INTEGER))),
       CAST(strftime('%s', 'now') AS INTEGER) * 1000, 1;

DELETE FROM settings WHERE key LIKE 'pct_%';
DROP TABLE transactions;
DROP TABLE debts;
DROP TABLE categories;
