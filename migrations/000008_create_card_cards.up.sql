SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.cards (
    id          UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    name        TEXT        NOT NULL,
    nickname    TEXT        NOT NULL,
    closing_day SMALLINT    NOT NULL,
    due_day     SMALLINT    NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL,
    CONSTRAINT cards_pkey             PRIMARY KEY (id),
    CONSTRAINT cards_user_fk          FOREIGN KEY (user_id)
        REFERENCES mecontrola.users(id) ON DELETE RESTRICT,
    CONSTRAINT cards_closing_day_chk  CHECK (closing_day BETWEEN 1 AND 31),
    CONSTRAINT cards_due_day_chk      CHECK (due_day     BETWEEN 1 AND 31),
    CONSTRAINT cards_name_len_chk     CHECK (char_length(name)     BETWEEN 1 AND 64),
    CONSTRAINT cards_nickname_len_chk CHECK (char_length(nickname) BETWEEN 1 AND 32)
);

CREATE UNIQUE INDEX IF NOT EXISTS cards_user_nickname_active_uniq_idx
    ON mecontrola.cards (user_id, nickname)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS cards_user_pagination_idx
    ON mecontrola.cards (user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
