SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE mecontrola.agent_decisions
    ADD COLUMN IF NOT EXISTS step_index INT NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS mecontrola.agent_decisions_user_channel_message_uniq_idx;

CREATE UNIQUE INDEX agent_decisions_user_channel_message_step_idx
    ON mecontrola.agent_decisions (user_id, channel, message_id, step_index);
