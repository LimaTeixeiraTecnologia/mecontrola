-- migration: 0003_identity.up.sql
-- Cria o substrato do módulo identity: users (PK UUID v4, soft delete, índice único
-- parcial em whatsapp_number) e user_whatsapp_history (append-only com active/unlinked_at).
-- Schema: public. Sem extensões — UUID v4 é gerado pela aplicação (google/uuid).

CREATE TABLE IF NOT EXISTS users (
    id              UUID         NOT NULL,
    whatsapp_number TEXT         NOT NULL,
    display_name    TEXT,
    email           TEXT,
    is_admin        BOOLEAN      NOT NULL DEFAULT false,
    status          TEXT         NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ,
    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT ck_users_status CHECK (status IN ('ACTIVE','BLOCKED','DELETED'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_whatsapp_number
    ON users (whatsapp_number)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email
    ON users (lower(email))
    WHERE deleted_at IS NULL AND email IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_status
    ON users (status)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS user_whatsapp_history (
    id           UUID         NOT NULL,
    user_id      UUID         NOT NULL,
    number       TEXT         NOT NULL,
    active       BOOLEAN      NOT NULL,
    linked_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    unlinked_at  TIMESTAMPTZ,
    reason       TEXT,
    CONSTRAINT pk_user_whatsapp_history PRIMARY KEY (id),
    CONSTRAINT fk_user_whatsapp_history_user_id
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_whatsapp_history_user_id_active
    ON user_whatsapp_history (user_id, active);

CREATE INDEX IF NOT EXISTS idx_user_whatsapp_history_number
    ON user_whatsapp_history (number);
