SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- ============================================================
-- Agents: estado nao-terminal 'processing' para auditoria de audio (ADR-002)
-- A linha passa a ser inserida ANTES do download/STT, garantindo que todo
-- WAMID tenha rastro mesmo se o worker morrer no meio do processamento.
-- Reason 'interrupted' finaliza linhas orfas de execucoes interrompidas.
-- ============================================================

ALTER TABLE mecontrola.agents_whatsapp_audio_messages
    DROP CONSTRAINT IF EXISTS agents_whatsapp_audio_messages_outcome_chk;

ALTER TABLE mecontrola.agents_whatsapp_audio_messages
    ADD CONSTRAINT agents_whatsapp_audio_messages_outcome_chk
        CHECK (outcome IN (
            'processing',
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
            'processing',
            'interrupted',
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

CREATE INDEX IF NOT EXISTS agents_whatsapp_audio_messages_user_id_created_at_idx
    ON mecontrola.agents_whatsapp_audio_messages (user_id, created_at DESC);
