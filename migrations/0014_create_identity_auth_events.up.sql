CREATE TABLE auth_events (
    id          UUID        NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id     UUID        NULL,
    kind        TEXT        NOT NULL,
    source      TEXT        NOT NULL,
    reason      TEXT        NULL,

    CONSTRAINT auth_events_pkey PRIMARY KEY (id),
    CONSTRAINT auth_events_kind_check
        CHECK (kind IN ('principal_established','failed','unknown_user')),
    CONSTRAINT auth_events_source_check
        CHECK (source = 'whatsapp'),
    CONSTRAINT auth_events_reason_check
        CHECK (
            (kind = 'failed' AND reason IN ('invalid_signature','unknown_wa_id','invalid_country','invalid_payload','rate_limited','db_unavailable'))
            OR (kind <> 'failed' AND reason IS NULL)
        )
);

CREATE INDEX auth_events_user_id_occurred_at_idx
    ON auth_events (user_id, occurred_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX auth_events_failed_occurred_at_idx
    ON auth_events (occurred_at DESC, reason)
    WHERE kind = 'failed';
