-- migration: 0002_outbox.up.sql
-- Cria o substrato Transactional Outbox: tabela imutavel de eventos +
-- tabela de deliveries (uma por subscription) com indices para claim, housekeeping e reaper.
-- Schema: public (D-07). Sem extensoes externas — apenas pgx/v5 nativo.

CREATE TABLE IF NOT EXISTS outbox_events (
    id              TEXT PRIMARY KEY,              -- ULID (events.EventID)
    event_type      TEXT NOT NULL,                 -- events.EventName (<modulo>.<acao>)
    event_version   SMALLINT NOT NULL DEFAULT 1,
    aggregate_type  TEXT NOT NULL,
    aggregate_id    TEXT NOT NULL,
    partition_key   TEXT NULL,                     -- D-10: reservada para V2
    payload         JSONB NOT NULL,
    headers         JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_outbox_events_type_aggregate
    ON outbox_events (event_type, aggregate_id);

CREATE TABLE IF NOT EXISTS outbox_deliveries (
    id                BIGSERIAL PRIMARY KEY,       -- ORDER BY id no claim
    event_id          TEXT NOT NULL REFERENCES outbox_events(id) ON DELETE CASCADE,
    subscription_name TEXT NOT NULL,
    status            TEXT NOT NULL CHECK (status IN ('pending','claimed','processed','dead_letter')),
    attempts          SMALLINT NOT NULL DEFAULT 0,
    next_retry_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error        TEXT NULL,
    processed_at      TIMESTAMPTZ NULL,
    dead_letter_at    TIMESTAMPTZ NULL,
    claimed_at        TIMESTAMPTZ NULL,
    claimed_by        TEXT NULL,                   -- D-11: hostname-pid
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_outbox_deliveries_event_subscription UNIQUE (event_id, subscription_name)
);

-- Index para o claim do Dispatcher (RF-10).
CREATE INDEX IF NOT EXISTS ix_outbox_deliveries_claim_ready
    ON outbox_deliveries (status, next_retry_at, id)
    WHERE status = 'pending';

-- Index para queries operacionais por subscription.
CREATE INDEX IF NOT EXISTS ix_outbox_deliveries_subscription_status
    ON outbox_deliveries (subscription_name, status);

-- Index parcial para housekeeping diario (RF-18).
CREATE INDEX IF NOT EXISTS ix_outbox_deliveries_housekeeping
    ON outbox_deliveries (COALESCE(processed_at, dead_letter_at))
    WHERE status IN ('processed','dead_letter');

-- Index parcial para reaper (RF-19).
CREATE INDEX IF NOT EXISTS ix_outbox_deliveries_claimed_stuck
    ON outbox_deliveries (claimed_at)
    WHERE status = 'claimed';
