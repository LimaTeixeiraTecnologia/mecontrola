SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

-- Indices para foreign keys de category_id/subcategory_id em transactions e
-- transactions_recurring_templates. O PostgreSQL nao cria indices automaticamente
-- para FKs; sem eles, DELETE/UPDATE na tabela pai (categories) forca sequential
-- scan nas tabelas filhas, gerando lock pesado e degradacao operacional.
--
-- Ref: postgresql.org/docs/17/sql-createindex.html
-- Ref: postgresql.org/docs/17/ddl-constraints.html#DDL-CONSTRAINTS-FK

CREATE INDEX IF NOT EXISTS transactions_category_id_idx
    ON mecontrola.transactions (category_id);

CREATE INDEX IF NOT EXISTS transactions_subcategory_id_idx
    ON mecontrola.transactions (subcategory_id);

CREATE INDEX IF NOT EXISTS transactions_recurring_templates_category_id_idx
    ON mecontrola.transactions_recurring_templates (category_id);

CREATE INDEX IF NOT EXISTS transactions_recurring_templates_subcategory_id_idx
    ON mecontrola.transactions_recurring_templates (subcategory_id);
