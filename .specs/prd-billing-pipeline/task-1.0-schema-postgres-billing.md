# Tarefa 1.0: Schema Postgres `0009_billing_schema` + seed `0010_billing_plans`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar as migrations `0009_billing_schema.{up,down}.sql` e `0010_billing_plans_seed.{up,down}.sql` em `migrations/` materializando o substrato físico de billing: event store imutável `webhook_events`, agregado `subscriptions`, idempotência `billing_event_applications` e seed de planos `billing_plans`.

<requirements>
- 4 tabelas com PK, FK, índices e constraints conforme techspec §Schema Postgres
- Convenções de nomenclatura `pk_/fk_/uq_/ck_/idx_` alinhadas com identity (migrations 0003/0004)
- Índice único parcial `WHERE deleted_at IS NULL` em `(provider, external_subscription_id)`
- Índice único parcial garantindo "1 user = 1 subscription ativa" (RF-13)
- `REVOKE ALL ON webhook_events FROM PUBLIC` aplicado na migration
- Seed dos 3 planos do MVP (D-03) idempotente via `ON CONFLICT DO NOTHING`
- Migrations down revertem na ordem inversa
- Validação via `RunMigrations` em testcontainer postgres:16-alpine
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/0009_billing_schema.up.sql` com `webhook_events` (id TEXT PK, provider, external_event_id, event_type, signature, headers JSONB, payload JSONB, received_at, processed_at, anonymized_at), constraint `UNIQUE (provider, external_event_id)`, índice `idx_webhook_events_received_at`, índice parcial `idx_webhook_events_pending_anonymization WHERE anonymized_at IS NULL`, `REVOKE ALL FROM PUBLIC`.
- [ ] 1.2 Adicionar `subscriptions` (id TEXT PK, user_id UUID FK ON DELETE RESTRICT, provider, external_subscription_id, plan_code, status, period_start, period_end, grace_period_end, refund_amount_cents BIGINT default 0, last_event_at, last_webhook_event_id FK, created_at, updated_at, deleted_at), constraints `ck_subscriptions_status`, `ck_subscriptions_period`, `ck_subscriptions_refund`, índice único parcial `uq_subscriptions_external_id WHERE deleted_at IS NULL`, índice único parcial `uq_subscriptions_one_active_per_user WHERE status IN ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING') AND deleted_at IS NULL`, índice `idx_subscriptions_status_active`.
- [ ] 1.3 Adicionar `billing_event_applications` (event_id TEXT PK, subscription_id TEXT FK ON DELETE CASCADE, applied_at), índice `idx_billing_event_applications_subscription_id`.
- [ ] 1.4 Adicionar `billing_plans` (plan_code TEXT PK, display_name, period_length_days INT > 0, price_brl_cents BIGINT > 0, kiwify_product_id TEXT NULL, active BOOLEAN default true, created_at, updated_at), constraints `ck_billing_plans_period` e `ck_billing_plans_price`.
- [ ] 1.5 Criar `migrations/0009_billing_schema.down.sql` revertendo ordem inversa (DROP TABLE billing_event_applications → subscriptions → webhook_events → billing_plans; DROP INDEXES).
- [ ] 1.6 Criar `migrations/0010_billing_plans_seed.up.sql` com `INSERT INTO billing_plans (plan_code, display_name, period_length_days, price_brl_cents) VALUES ('MONTHLY','Mensal',30,2990), ('QUARTERLY','Trimestral',90,8073), ('ANNUAL','Anual',365,29780) ON CONFLICT (plan_code) DO NOTHING`.
- [ ] 1.7 Criar `migrations/0010_billing_plans_seed.down.sql` com `DELETE FROM billing_plans WHERE plan_code IN ('MONTHLY','QUARTERLY','ANNUAL')`.
- [ ] 1.8 Verificar embed em `migrations/embed.go` (já usa `//go:embed *.sql`).

## Detalhes de Implementação

Ver techspec §Schema Postgres (linhas referentes a `webhook_events`, `subscriptions`, `billing_event_applications`, `billing_plans`) e ADR-007 (retenção two-tier). Convenção de nomenclatura herdada do schema identity (`prd-identity-foundation/techspec.md` §Schema Postgres).

## Critérios de Sucesso

- `golang-migrate` aplica `0009` e `0010` em ordem sem erro em Postgres 16.
- `psql -c "\d webhook_events"` mostra todas as colunas + constraints; `\d subscriptions` idem.
- `SELECT count(*) FROM billing_plans` retorna `3` após `0010`.
- Re-execução de `0010` é no-op (`ON CONFLICT DO NOTHING`).
- `0009 down` + `0009 up` é idempotente (DROP IF EXISTS + CREATE IF NOT EXISTS).
- Tentativa de `INSERT INTO subscriptions` com `status='INVALID'` falha por `ck_subscriptions_status`.
- Tentativa de criar 2 subscriptions com mesmo `user_id` e ambos `status='ACTIVE'` falha por `uq_subscriptions_one_active_per_user`.
- `INSERT INTO webhook_events` com mesmo `(provider, external_event_id)` 2x falha por `uq_webhook_events_external`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de integração `migrations_test.go` com testcontainers postgres:16-alpine aplicando 0001..0010 em ordem, validando estrutura via `\d` e count em `billing_plans`.
- [ ] Teste negativo: INSERT violando cada constraint declarada retorna o código `pgerrcode.CheckViolation` ou `UniqueViolation` esperado.
- [ ] Teste de idempotência: aplicar 0010 duas vezes mantém 3 linhas.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `migrations/0009_billing_schema.up.sql` (novo)
- `migrations/0009_billing_schema.down.sql` (novo)
- `migrations/0010_billing_plans_seed.up.sql` (novo)
- `migrations/0010_billing_plans_seed.down.sql` (novo)
- `migrations/embed.go` (verificação — sem alteração esperada)
- Referências cruzadas: `internal/platform/database/database_integration_test.go` (padrão de aplicação de migrations em testcontainer)
