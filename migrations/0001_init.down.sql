-- migration: 0001_init.down.sql
-- Reverts 0001_init.up.sql by dropping the health_probe table.

DROP TABLE IF EXISTS health_probe;
