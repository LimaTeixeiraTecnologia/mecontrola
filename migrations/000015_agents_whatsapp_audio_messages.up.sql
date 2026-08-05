SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- ============================================================
-- Agents: WhatsApp audio inbound audit trail
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.agents_whatsapp_audio_messages (
    wamid                 TEXT        NOT NULL,
    user_id               UUID        NOT NULL,
    peer                  TEXT        NOT NULL,
    media_id              TEXT        NOT NULL,
    media_sha256          TEXT        NOT NULL,
    mime_type             TEXT        NOT NULL,
    size_bytes            BIGINT      NOT NULL,
    duration_ms           INTEGER,
    stt_model             TEXT,
    outcome               TEXT        NOT NULL,
    reason                TEXT        NOT NULL,
    transcription         TEXT,
    transcription_sha256  TEXT,
    cost_microusd         BIGINT,
    error_code            TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at          TIMESTAMPTZ,

    CONSTRAINT agents_whatsapp_audio_messages_pkey PRIMARY KEY (wamid),
    CONSTRAINT agents_whatsapp_audio_messages_wamid_nonempty_chk
        CHECK (length(wamid) > 0),
    CONSTRAINT agents_whatsapp_audio_messages_peer_nonempty_chk
        CHECK (length(peer) > 0),
    CONSTRAINT agents_whatsapp_audio_messages_media_id_nonempty_chk
        CHECK (length(media_id) > 0),
    CONSTRAINT agents_whatsapp_audio_messages_media_sha256_nonempty_chk
        CHECK (length(media_sha256) > 0),
    CONSTRAINT agents_whatsapp_audio_messages_mime_type_nonempty_chk
        CHECK (length(mime_type) > 0),
    CONSTRAINT agents_whatsapp_audio_messages_size_bytes_chk
        CHECK (size_bytes >= 0),
    CONSTRAINT agents_whatsapp_audio_messages_duration_ms_chk
        CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CONSTRAINT agents_whatsapp_audio_messages_cost_microusd_chk
        CHECK (cost_microusd IS NULL OR cost_microusd >= 0),
    CONSTRAINT agents_whatsapp_audio_messages_outcome_chk
        CHECK (outcome IN (
            'approved',
            'rejected',
            'transcription_uncertain',
            'transcription_failed',
            'dispatched'
        )),
    CONSTRAINT agents_whatsapp_audio_messages_reason_chk
        CHECK (reason IN (
            'approved',
            'stt_error',
            'truncated',
            'empty_text',
            'incoherent',
            'language_unsupported',
            'low_confidence'
        ))
);

CREATE INDEX IF NOT EXISTS agents_whatsapp_audio_messages_created_at_idx
    ON mecontrola.agents_whatsapp_audio_messages (created_at);
