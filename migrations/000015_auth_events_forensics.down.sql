DROP INDEX IF EXISTS mecontrola.auth_events_request_id_idx;

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
                'db_unavailable'
            ))
            OR (kind <> 'failed' AND reason IS NULL)
        );

ALTER TABLE mecontrola.auth_events
    DROP COLUMN client_ip;

ALTER TABLE mecontrola.auth_events
    DROP COLUMN request_id;
