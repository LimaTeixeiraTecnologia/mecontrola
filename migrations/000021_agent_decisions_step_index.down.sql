DROP INDEX IF EXISTS mecontrola.agent_decisions_user_channel_message_step_idx;

CREATE UNIQUE INDEX agent_decisions_user_channel_message_uniq_idx
    ON mecontrola.agent_decisions (user_id, channel, message_id);

ALTER TABLE mecontrola.agent_decisions
    DROP COLUMN IF EXISTS step_index;
