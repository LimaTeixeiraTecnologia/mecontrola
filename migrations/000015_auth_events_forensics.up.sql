ALTER TABLE mecontrola.auth_events
    ADD COLUMN request_id TEXT NULL,
    ADD COLUMN client_ip  INET NULL;

ALTER TABLE mecontrola.auth_events
    DROP CONSTRAINT auth_events_reason_check;

ALTER TABLE mecontrola.auth_events
    ADD CONSTRAINT auth_events_reason_check
        CHECK (
            (kind = 'failed' AND reason IN (
                'invalid_signature',
                'unknown_wa_id',
                'invalid_country',
                'invalid_payload',
                'rate_limited',
                'db_unavailable',
                'gateway_missing_header',
                'gateway_invalid_timestamp',
                'gateway_stale_timestamp',
                'gateway_invalid_signature'
            ))
            OR (kind <> 'failed' AND reason IS NULL)
        );

CREATE INDEX IF NOT EXISTS auth_events_request_id_idx
    ON mecontrola.auth_events (request_id)
    WHERE request_id IS NOT NULL;
