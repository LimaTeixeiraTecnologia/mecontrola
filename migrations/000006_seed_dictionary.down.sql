SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

SELECT 1 WHERE false;
