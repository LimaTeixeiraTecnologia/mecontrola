SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.whatsapp_message_status_status_idx;
DROP INDEX IF EXISTS mecontrola.whatsapp_message_status_message_id_idx;

ALTER TABLE mecontrola.whatsapp_message_status
    RENAME CONSTRAINT whatsapp_message_status_pkey TO whatsapp_message_status_archived_20260622130000_pkey;

ALTER TABLE mecontrola.whatsapp_message_status
    RENAME CONSTRAINT whatsapp_message_status_message_id_len_chk TO whatsapp_message_status_archived_20260622130000_message_id_len_chk;

ALTER TABLE mecontrola.whatsapp_message_status
    RENAME CONSTRAINT whatsapp_message_status_status_chk TO whatsapp_message_status_archived_20260622130000_status_chk;

ALTER TABLE mecontrola.whatsapp_message_status
    RENAME CONSTRAINT whatsapp_message_status_message_status_uniq TO whatsapp_message_status_archived_20260622130000_message_status_uniq;

ALTER TABLE mecontrola.whatsapp_message_status RENAME TO whatsapp_message_status_archived_20260622130000;
