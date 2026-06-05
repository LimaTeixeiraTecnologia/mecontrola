CREATE TABLE user_whatsapp_history (
    id          UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    number      TEXT        NOT NULL,
    active      BOOLEAN     NOT NULL,
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    unlinked_at TIMESTAMPTZ NULL,
    reason      TEXT        NULL,

    CONSTRAINT user_whatsapp_history_pkey PRIMARY KEY (id),
    CONSTRAINT user_whatsapp_history_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT user_whatsapp_history_active_unlinked_at_check
        CHECK ((active = TRUE) = (unlinked_at IS NULL))
);

CREATE INDEX user_whatsapp_history_user_active_idx
    ON user_whatsapp_history (user_id, active);

CREATE INDEX user_whatsapp_history_number_idx
    ON user_whatsapp_history (number);
