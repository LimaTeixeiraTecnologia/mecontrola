SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.whatsapp_message_status (
    id           UUID        NOT NULL,
    message_id   TEXT        NOT NULL,
    status       TEXT        NOT NULL,
    recipient_id TEXT        NOT NULL DEFAULT '',
    error_code   TEXT,
    error_title  TEXT,
    status_at    TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT whatsapp_message_status_pkey PRIMARY KEY (id),
    CONSTRAINT whatsapp_message_status_message_id_len_chk
        CHECK (char_length(message_id) BETWEEN 1 AND 256),
    CONSTRAINT whatsapp_message_status_status_chk
        CHECK (status IN ('sent', 'delivered', 'read', 'failed')),
    CONSTRAINT whatsapp_message_status_message_status_uniq
        UNIQUE (message_id, status)
);

CREATE INDEX IF NOT EXISTS whatsapp_message_status_message_id_idx
    ON mecontrola.whatsapp_message_status (message_id);

CREATE INDEX IF NOT EXISTS whatsapp_message_status_status_idx
    ON mecontrola.whatsapp_message_status (status);
