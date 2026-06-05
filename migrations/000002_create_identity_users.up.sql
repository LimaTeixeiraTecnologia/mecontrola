CREATE TABLE users (
    id              UUID        NOT NULL,
    whatsapp_number TEXT        NOT NULL,
    email           TEXT        NULL,
    display_name    TEXT        NULL,
    status          TEXT        NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ NULL,

    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_status_check CHECK (status IN ('ACTIVE', 'DELETED')),
    CONSTRAINT users_status_deleted_at_check
        CHECK ((status = 'DELETED') = (deleted_at IS NOT NULL))
);

-- Soft-deleted users do not block reactivation or recreation flows.
CREATE UNIQUE INDEX users_whatsapp_number_active_uniq_idx
    ON users (whatsapp_number)
    WHERE deleted_at IS NULL;

CREATE INDEX users_whatsapp_number_deleted_idx
    ON users (whatsapp_number)
    WHERE deleted_at IS NOT NULL;

CREATE UNIQUE INDEX users_email_active_uniq_idx
    ON users (email)
    WHERE email IS NOT NULL AND deleted_at IS NULL;
