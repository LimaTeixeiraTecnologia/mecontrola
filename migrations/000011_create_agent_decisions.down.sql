SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.agent_decisions_status_idx;
DROP INDEX IF EXISTS mecontrola.agent_decisions_user_created_idx;
DROP INDEX IF EXISTS mecontrola.agent_decisions_user_channel_message_uniq_idx;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_pkey TO agent_decisions_archived_20260622120000_pkey;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_user_id_fkey TO agent_decisions_archived_20260622120000_user_id_fkey;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_channel_len_chk TO agent_decisions_archived_20260622120000_channel_len_chk;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_message_id_len_chk TO agent_decisions_archived_20260622120000_message_id_len_chk;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_intent_kind_len_chk TO agent_decisions_archived_20260622120000_intent_kind_len_chk;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_prompt_sha256_len_chk TO agent_decisions_archived_20260622120000_prompt_sha256_len_chk;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_llm_model_len_chk TO agent_decisions_archived_20260622120000_llm_model_len_chk;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_decided_action_len_chk TO agent_decisions_archived_20260622120000_decided_action_len_chk;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_redacted_response_size_chk TO agent_decisions_archived_20260622120000_redacted_response_size_chk;

ALTER TABLE mecontrola.agent_decisions
    RENAME CONSTRAINT agent_decisions_status_chk TO agent_decisions_archived_20260622120000_status_chk;

ALTER TABLE mecontrola.agent_decisions RENAME TO agent_decisions_archived_20260622120000;
