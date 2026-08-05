SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

UPDATE mecontrola.agents_whatsapp_audio_messages
   SET size_bytes = 0
 WHERE size_bytes IS NULL;

ALTER TABLE mecontrola.agents_whatsapp_audio_messages
    ALTER COLUMN size_bytes SET NOT NULL;
