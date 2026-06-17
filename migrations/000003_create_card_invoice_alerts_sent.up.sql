SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.card_invoice_alerts_sent (
    user_id        UUID        NOT NULL,
    card_id        UUID        NOT NULL,
    ref_due_date   DATE        NOT NULL,
    sent_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    notified_at    TIMESTAMPTZ NULL,
    notify_channel TEXT        NULL,

    CONSTRAINT card_invoice_alerts_sent_pkey PRIMARY KEY (user_id, card_id, ref_due_date),
    CONSTRAINT card_invoice_alerts_sent_card_fk FOREIGN KEY (card_id)
        REFERENCES mecontrola.cards(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS card_invoice_alerts_sent_user_due_idx
    ON mecontrola.card_invoice_alerts_sent (user_id, ref_due_date DESC);

CREATE INDEX IF NOT EXISTS card_invoice_alerts_sent_pending_notify_idx
    ON mecontrola.card_invoice_alerts_sent (sent_at)
    WHERE notified_at IS NULL;

CREATE INDEX IF NOT EXISTS cards_due_day_scan_idx
    ON mecontrola.cards (due_day)
    WHERE deleted_at IS NULL;
