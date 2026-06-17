SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.cards_due_day_scan_idx;
DROP INDEX IF EXISTS mecontrola.card_invoice_alerts_sent_pending_notify_idx;
DROP INDEX IF EXISTS mecontrola.card_invoice_alerts_sent_user_due_idx;
DROP TABLE IF EXISTS mecontrola.card_invoice_alerts_sent;
