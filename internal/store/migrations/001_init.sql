-- Valores monetários ficam em centavos (INTEGER) para evitar arredondamento.
-- Datas ficam como texto ISO (AAAA-MM-DD), que ordena e compara corretamente.

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- bucket: necessidades | desejos | investimentos | renda
CREATE TABLE categories (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    bucket     TEXT NOT NULL CHECK (bucket IN ('necessidades', 'desejos', 'investimentos', 'renda')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- kind: entrada | saida
CREATE TABLE transactions (
    id           INTEGER PRIMARY KEY,
    kind         TEXT NOT NULL CHECK (kind IN ('entrada', 'saida')),
    description  TEXT NOT NULL,
    amount_cents INTEGER NOT NULL CHECK (amount_cents >= 0),
    category_id  INTEGER NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    occurred_on  TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX transactions_occurred_on_idx ON transactions (occurred_on);

CREATE TABLE debts (
    id                INTEGER PRIMARY KEY,
    name              TEXT NOT NULL,
    creditor          TEXT NOT NULL DEFAULT '',
    balance_cents     INTEGER NOT NULL CHECK (balance_cents >= 0),
    monthly_rate_bp   INTEGER NOT NULL DEFAULT 0, -- juros ao mês em pontos-base (1,99% = 199)
    min_payment_cents INTEGER NOT NULL DEFAULT 0,
    due_day           INTEGER CHECK (due_day BETWEEN 1 AND 31),
    created_at        TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE chat_messages (
    id         INTEGER PRIMARY KEY,
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content    TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO settings (key, value) VALUES
    ('pct_necessidades', '50'),
    ('pct_desejos', '30'),
    ('pct_investimentos', '20');

INSERT INTO categories (name, bucket) VALUES
    ('Salário', 'renda'),
    ('Renda extra', 'renda'),
    ('Moradia', 'necessidades'),
    ('Contas da casa (luz, água, internet)', 'necessidades'),
    ('Mercado', 'necessidades'),
    ('Transporte', 'necessidades'),
    ('Saúde', 'necessidades'),
    ('Educação', 'necessidades'),
    ('Pagamento de dívidas', 'necessidades'),
    ('Lazer', 'desejos'),
    ('Restaurantes e delivery', 'desejos'),
    ('Compras', 'desejos'),
    ('Assinaturas', 'desejos'),
    ('Reserva de emergência', 'investimentos'),
    ('Investimentos', 'investimentos');
