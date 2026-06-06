CREATE TABLE billing_reconciliation_checkpoints (
    name       TEXT PRIMARY KEY,
    watermark  TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
