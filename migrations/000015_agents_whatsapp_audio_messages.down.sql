SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.agents_whatsapp_audio_messages_created_at_idx;

DROP TABLE IF EXISTS mecontrola.agents_whatsapp_audio_messages;
