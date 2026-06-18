SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.agent_sessions_expires_idx;
DROP INDEX IF EXISTS mecontrola.agent_sessions_user_channel_uniq_idx;

ALTER TABLE mecontrola.agent_sessions
    RENAME CONSTRAINT agent_sessions_pkey TO agent_sessions_archived_20260618120000_pkey;

ALTER TABLE mecontrola.agent_sessions
    RENAME CONSTRAINT agent_sessions_user_id_fkey TO agent_sessions_archived_20260618120000_user_id_fkey;

ALTER TABLE mecontrola.agent_sessions
    RENAME CONSTRAINT agent_sessions_channel_len_chk TO agent_sessions_archived_20260618120000_channel_len_chk;

ALTER TABLE mecontrola.agent_sessions
    RENAME CONSTRAINT agent_sessions_pending_action_size_chk TO agent_sessions_archived_20260618120000_pending_action_size_chk;

ALTER TABLE mecontrola.agent_sessions
    RENAME CONSTRAINT agent_sessions_recent_turns_size_chk TO agent_sessions_archived_20260618120000_recent_turns_size_chk;

ALTER TABLE mecontrola.agent_sessions RENAME TO agent_sessions_archived_20260618120000;
