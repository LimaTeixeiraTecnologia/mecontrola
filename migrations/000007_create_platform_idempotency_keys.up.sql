SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.idempotency_keys (
    scope            TEXT        NOT NULL,
    key              TEXT        NOT NULL,
    user_id          UUID        NOT NULL,
    request_hash     TEXT        NOT NULL,
    response_status  INT         NOT NULL,
    response_body    BYTEA       NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT idempotency_keys_pkey                  PRIMARY KEY (scope, key, user_id),
    CONSTRAINT idempotency_keys_key_len_chk           CHECK (char_length(key) BETWEEN 1 AND 128),
    CONSTRAINT idempotency_keys_request_hash_len_chk  CHECK (char_length(request_hash) = 64),
    CONSTRAINT idempotency_keys_status_chk            CHECK (response_status BETWEEN 200 AND 599),
    CONSTRAINT idempotency_keys_body_size_chk         CHECK (octet_length(response_body) <= 65536)
);

CREATE INDEX IF NOT EXISTS idempotency_keys_expires_idx
    ON mecontrola.idempotency_keys (expires_at);
