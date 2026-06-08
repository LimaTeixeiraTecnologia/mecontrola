CREATE TABLE onboarding.meta_processed_messages (
    wamid        TEXT         PRIMARY KEY,
    processed_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_meta_processed_messages_processed_at
    ON onboarding.meta_processed_messages (processed_at);
