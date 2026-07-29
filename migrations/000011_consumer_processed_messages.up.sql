SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- ============================================================
-- Platform: Consumer deduplication
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.consumer_processed_messages (
    consumer      TEXT        NOT NULL,
    message_id    TEXT        NOT NULL,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT consumer_processed_messages_pkey PRIMARY KEY (consumer, message_id),
    CONSTRAINT consumer_processed_messages_consumer_nonempty_check
        CHECK (length(consumer) > 0),
    CONSTRAINT consumer_processed_messages_message_id_nonempty_check
        CHECK (length(message_id) > 0)
);

CREATE INDEX IF NOT EXISTS consumer_processed_messages_processed_at_idx
    ON mecontrola.consumer_processed_messages (processed_at);
