SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.banks (
    code            TEXT     NOT NULL,
    name            TEXT     NOT NULL,
    days_before_due SMALLINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT banks_pkey         PRIMARY KEY (code),
    CONSTRAINT banks_code_len_chk CHECK (char_length(code) BETWEEN 1 AND 64),
    CONSTRAINT banks_name_len_chk CHECK (char_length(name) BETWEEN 1 AND 128),
    CONSTRAINT banks_days_chk     CHECK (days_before_due BETWEEN 1 AND 28)
);

INSERT INTO mecontrola.banks (code, name, days_before_due) VALUES
    ('nubank',          'Nubank',          7),
    ('itau',            'Itaú',            8),
    ('santander',       'Santander',       8),
    ('bradesco',        'Bradesco',        7),
    ('banco-do-brasil', 'Banco do Brasil', 7),
    ('caixa',           'Caixa',           7),
    ('inter',           'Inter',           7),
    ('c6-bank',         'C6 Bank',         7)
ON CONFLICT (code) DO NOTHING;

ALTER TABLE mecontrola.cards DROP CONSTRAINT cards_limit_cents_chk;
ALTER TABLE mecontrola.cards DROP COLUMN limit_cents;
ALTER TABLE mecontrola.cards DROP CONSTRAINT cards_name_len_chk;
ALTER TABLE mecontrola.cards DROP COLUMN name;
ALTER TABLE mecontrola.cards ADD COLUMN bank TEXT NOT NULL DEFAULT '';
ALTER TABLE mecontrola.cards ADD CONSTRAINT cards_bank_len_chk CHECK (char_length(bank) BETWEEN 1 AND 64);
ALTER TABLE mecontrola.cards ALTER COLUMN bank DROP DEFAULT;
