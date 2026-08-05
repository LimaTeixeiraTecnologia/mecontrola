SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.agents_whatsapp_audio_messages_user_id_created_at_idx;

DELETE FROM mecontrola.agents_whatsapp_audio_messages
 WHERE outcome = 'processing' OR reason IN ('processing', 'interrupted');

ALTER TABLE mecontrola.agents_whatsapp_audio_messages
    DROP CONSTRAINT IF EXISTS agents_whatsapp_audio_messages_outcome_chk;

ALTER TABLE mecontrola.agents_whatsapp_audio_messages
    ADD CONSTRAINT agents_whatsapp_audio_messages_outcome_chk
        CHECK (outcome IN (
            'approved',
            'rejected',
            'transcription_uncertain',
            'transcription_failed',
            'dispatched'
        ));

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
            'low_confidence',
            'invalid_payload',
            'media_unavailable',
            'size_exceeded',
            'duration_unavailable',
            'duration_exceeded',
            'cost_exceeded'
        ));
