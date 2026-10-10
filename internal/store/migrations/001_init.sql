-- Valores monetários ficam em centavos (BIGINT) para evitar arredondamento.

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- bucket: necessidades | desejos | investimentos | renda
CREATE TABLE categories (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    bucket     TEXT NOT NULL CHECK (bucket IN ('necessidades', 'desejos', 'investimentos', 'renda')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- kind: entrada | saida
CREATE TABLE transactions (
    id           BIGSERIAL PRIMARY KEY,
    kind         TEXT NOT NULL CHECK (kind IN ('entrada', 'saida')),
    description  TEXT NOT NULL,
    amount_cents BIGINT NOT NULL CHECK (amount_cents >= 0),
    category_id  BIGINT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    occurred_on  DATE NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX transactions_occurred_on_idx ON transactions (occurred_on);

CREATE TABLE debts (
    id                  BIGSERIAL PRIMARY KEY,
    name                TEXT NOT NULL,
    creditor            TEXT NOT NULL DEFAULT '',
    balance_cents       BIGINT NOT NULL CHECK (balance_cents >= 0),
    monthly_rate_bp     INTEGER NOT NULL DEFAULT 0, -- juros ao mês em pontos-base (1,99% = 199)
    min_payment_cents   BIGINT NOT NULL DEFAULT 0,
    due_day             INTEGER CHECK (due_day BETWEEN 1 AND 31),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- content guarda o BetaMessageParam da Anthropic em JSON, exatamente como foi
-- enviado/recebido, para reenviar o histórico sem alterações.
CREATE TABLE chat_messages (
    id         BIGSERIAL PRIMARY KEY,
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    display    TEXT NOT NULL,
    content    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
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
