SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE mecontrola.agents_whatsapp_audio_messages
    DROP CONSTRAINT IF EXISTS agents_whatsapp_audio_messages_reason_chk;

ALTER TABLE mecontrola.agents_whatsapp_audio_messages
    ADD CONSTRAINT agents_whatsapp_audio_messages_reason_chk
        CHECK (reason IN (
            'approved',
            'stt_error',
            'truncated',
            'empty_text',
            'incoherent',
            'language_unsupported',
            'low_confidence'
        ));
