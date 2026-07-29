SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.consumer_processed_messages_processed_at_idx;

DROP TABLE IF EXISTS mecontrola.consumer_processed_messages;
