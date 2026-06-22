CREATE TABLE mecontrola.agent_observations (
    id         UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL,
    channel    TEXT        NOT NULL,
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '90 days',
    CONSTRAINT agent_observations_pk PRIMARY KEY (id)
);

CREATE INDEX agent_observations_user_channel_idx ON mecontrola.agent_observations (user_id, channel, created_at DESC);
CREATE INDEX agent_observations_expires_at_idx ON mecontrola.agent_observations (expires_at);
