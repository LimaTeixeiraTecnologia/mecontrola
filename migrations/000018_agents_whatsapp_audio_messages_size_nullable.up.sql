SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- ============================================================
-- Agents: size_bytes passa a ser NULL quando o tamanho e desconhecido.
-- Antes, rejeicoes anteriores ao download (media_resolve_failed,
-- max_bytes_not_configured, falha de download) gravavam 0, tornando
-- "desconhecido" indistinguivel de "audio de zero bytes" na auditoria.
-- ============================================================

ALTER TABLE mecontrola.agents_whatsapp_audio_messages
    ALTER COLUMN size_bytes DROP NOT NULL;

UPDATE mecontrola.agents_whatsapp_audio_messages
   SET size_bytes = NULL
 WHERE size_bytes = 0;
