CREATE TABLE billing_plans (
    kiwify_product_id TEXT PRIMARY KEY,
    code              TEXT NOT NULL UNIQUE CHECK (code IN ('MONTHLY', 'QUARTERLY', 'ANNUAL')),
    duration_days     INTEGER NOT NULL CHECK (duration_days > 0),
    currency          TEXT NOT NULL DEFAULT 'BRL'
);

INSERT INTO billing_plans (kiwify_product_id, code, duration_days) VALUES
    ('<id-mensal>', 'MONTHLY', 30),
    ('<id-trimestral>', 'QUARTERLY', 90),
    ('<id-anual>', 'ANNUAL', 365);
