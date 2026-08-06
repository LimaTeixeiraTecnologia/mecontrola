SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DELETE FROM mecontrola.budget_alerts_sent
 WHERE kind IN (
     'category_threshold_80',
     'category_threshold_100',
     'budget_missing_month_start',
     'budget_not_reviewed_day_3'
 );

ALTER TABLE mecontrola.budget_alerts_sent
    DROP CONSTRAINT IF EXISTS budget_alerts_sent_kind_chk;

ALTER TABLE mecontrola.budget_alerts_sent
    ADD CONSTRAINT budget_alerts_sent_kind_chk
        CHECK (kind IN (
            'category_threshold',
            'goal_achieved',
            'card_limit_near'
        ));
