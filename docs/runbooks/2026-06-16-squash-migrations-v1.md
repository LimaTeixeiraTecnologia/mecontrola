# Squash Migrations v1 — Runbook

**Data**: 2026-06-16
**Escopo**: Consolidação de 000001–000009 em único `000001_initial_baseline.{up,down}.sql`

## O que mudou

9 pares de migration (18 arquivos) foram squashados em 1 (1950 linhas). O schema final é
idêntico ao estado anterior mais as correções de integridade abaixo.

### PostgreSQL Best Practices aplicadas

| Problema | Correção |
|---|---|
| `CHAR(7)` em 9 colunas ref_month/competence | `TEXT CONSTRAINT <table>_chk CHECK (col ~ '^\d{4}-(0[1-9]\|1[0-2])$')` — nomeado e valida mês |
| `CREATE TABLE/INDEX` sem `IF NOT EXISTS` | Adicionado em todos os 37 objetos |
| `plan_id UUID` em onboarding_tokens | `plan_id TEXT` desde a criação |
| Colunas adicionadas em incrementais | Incluídas na criação das tabelas |
| Tabelas adicionadas em incrementais (budget_alerts_sent, onboarding_sessions) | Inline no baseline |
| Índices com prefixo `idx_*` | Renomeados para sufixo `_idx` |
| CHECKs inline anônimos em transactions + budgets | Todos nomeados (`<table>_<col>_chk`) |
| **FK faltante: `transactions_card_purchases.card_id`** | `FOREIGN KEY (card_id) REFERENCES mecontrola.cards(id) ON DELETE RESTRICT` |
| **FK faltante: `transactions_recurring_templates.card_id`** | idem |
| **FK faltante: `billing_subscriptions.user_id`** | `FOREIGN KEY (user_id) REFERENCES mecontrola.users(id) ON DELETE RESTRICT` |
| **CHECK faltante: `budgets.state`** | `CHECK (state IN (1, 2))` — Draft/Active |
| **CHECK faltante: `budgets_alerts.state`** | `CHECK (state BETWEEN 1 AND 5)` — 5 estados do domínio |
| **CHECK faltante: `budgets_expense_events_pending.state`** | `CHECK (state BETWEEN 1 AND 4)` — Pending/Applied/Failed/Expired |
| **CHECK faltante: `budgets_expense_events_pending.mutation_kind`** | `CHECK (mutation_kind BETWEEN 1 AND 3)` — Create/Update/Delete |

## Rollout — Ambientes Existentes

Ambientes com 000001–000009 já aplicadas possuem o schema correto. Apenas o registro
de controle do golang-migrate precisa ser atualizado:

```bash
# Opção A — golang-migrate CLI (recomendado)
migrate -path migrations -database "$DATABASE_URL" force 1

# Opção B — psql direto
psql "$DATABASE_URL" <<'SQL'
DELETE FROM schema_migrations;
INSERT INTO schema_migrations (version, dirty) VALUES (1, false);
SQL
```

## Rollout — Ambientes Fresh (CI, staging, novos deploys)

Nenhuma ação especial. `task migrate:up` aplica o único migration normalmente.

## Verificação pós-rollout

```bash
# 1. Nenhum CHAR(7) residual
psql "$DATABASE_URL" -c "
  SELECT table_name, column_name
  FROM information_schema.columns
  WHERE table_schema = 'mecontrola'
    AND data_type = 'character'
    AND character_maximum_length = 7;
"

# 2. Tabelas críticas presentes
psql "$DATABASE_URL" -c "
  SELECT table_name
  FROM information_schema.tables
  WHERE table_schema = 'mecontrola'
  ORDER BY table_name;
"

# 3. Testes de integração
go test -v -tags=integration -timeout=120s ./migrations/...
```

## Reversão

```bash
# Reverte o schema completo (destrói todos os dados)
task migrate:down
```
