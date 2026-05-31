-- migration: 0001_init.up.sql
-- Creates the health_probe table used by Manager.HealthCheck (SELECT 1 FROM health_probe).
-- This is the foundation migration; domain tables will be added by subsequent PRDs.

CREATE TABLE IF NOT EXISTS health_probe (
    id   SERIAL PRIMARY KEY,
    note TEXT NOT NULL DEFAULT 'ok'
);

INSERT INTO health_probe (note) VALUES ('ok')
    ON CONFLICT DO NOTHING;
