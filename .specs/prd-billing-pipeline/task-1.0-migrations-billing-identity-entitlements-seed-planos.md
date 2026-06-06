# Tarefa 1.0: Migrations 0004–0009 + seed de planos

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Subir o schema relacional que sustenta o ciclo de vida da assinatura: 5 tabelas em `billing` (`billing_plans`, `billing_subscriptions`, `billing_processed_events`, `billing_kiwify_events`, `billing_reconciliation_checkpoints`) e 2 tabelas em `identity` (`identity_entitlements`, `identity_entitlements_pending`). Inclui seed inicial dos 3 planos do MVP (`MONTHLY`, `QUARTERLY`, `ANNUAL`).

<requirements>
- 6 pares `migrations/000{4..9}_*.{up,down}.sql` versionados.
- DDL conforme techspec §6.7 (tipos, índices únicos parciais e CHECKs).
- Seed em `0004_create_billing_plans.up.sql` com `kiwify_product_id` placeholder que será preenchido por env/secret antes do deploy.
- Não introduzir colunas, índices ou constraints não previstos na techspec.
- `down.sql` deve reverter cleanly em ambiente local de teste.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/0004_create_billing_plans.{up,down}.sql` com tabela `billing_plans` (PK `kiwify_product_id`, UNIQUE `code` IN ('MONTHLY','QUARTERLY','ANNUAL'), `duration_days` ∈ {30, 90, 365}, `currency` default 'BRL') e seed dos 3 planos.
- [ ] 1.2 Criar `migrations/0005_create_billing_subscriptions.{up,down}.sql` com tabela `billing_subscriptions`, índice único parcial `billing_subscriptions_user_active_uniq_idx` (RF-17), índice único `kiwify_order_id` e índice por `funnel_token` (RF-16).
- [ ] 1.3 Criar `migrations/0006_create_billing_processed_events.{up,down}.sql` (PK `event_key`, CHECK `status IN ('applied','superseded')`).
- [ ] 1.4 Criar `migrations/0007_create_billing_kiwify_events.{up,down}.sql` (auditoria raw em JSONB, CHECK `signature_status IN ('valid','invalid','rotated')`, índices por `received_at` e `trigger`).
- [ ] 1.5 Criar `migrations/0008_create_billing_reconciliation_checkpoints.{up,down}.sql` (PK `name`, `watermark TIMESTAMPTZ`).
- [ ] 1.6 Criar `migrations/0009_create_identity_entitlements.{up,down}.sql` com `identity_entitlements` (FK `users(id)`) e `identity_entitlements_pending` (sem FK; usa apenas `subscription_id` e `funnel_token`).
- [ ] 1.7 Validar aplicação up/down em Postgres local (via tooling existente do projeto).

## Detalhes de Implementação

- DDL completa em techspec §6.7. Não duplicar SQL aqui.
- `billing_plans.kiwify_product_id` no seed inicial: usar placeholders `'<id-mensal>'`, `'<id-trimestral>'`, `'<id-anual>'` que serão substituídos por valores reais via UPDATE de migration posterior ou via runbook; não bloquear esta tarefa por ausência dos IDs (decisão consolidada na techspec §6.7).
- Índice único parcial `billing_subscriptions_user_active_uniq_idx` é a única barreira de RF-17 (1 sub ativa por user); confirmar `WHERE user_id IS NOT NULL AND status IN ('ACTIVE','PAST_DUE','CANCELED_PENDING')`.
- `billing_subscriptions.status` CHECK deve incluir `TRIALING` (reservado/inerte, RF-04).
- `identity_entitlements.user_id` é PK e FK para `users(id)`. `identity_entitlements_pending.subscription_id` é PK e fica sem user enquanto E3 não fechar o bind.

## Critérios de Sucesso

- 12 arquivos SQL (6 pares) presentes em `migrations/`, numerados 0004 a 0009.
- `up.sql` aplica sem erro em base limpa; `down.sql` reverte sem deixar artefatos.
- `SELECT code, duration_days FROM billing_plans` após seed retorna exatamente 3 linhas `(MONTHLY,30)`, `(QUARTERLY,90)`, `(ANNUAL,365)`.
- INSERTs concorrentes de 2ª `billing_subscriptions` ativa para o mesmo `user_id` falham com violação de índice único.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de migração: aplicar `up` e `down` numa base efêmera; verificar diff de schema = vazio após `down`.
- [ ] Teste de integração leve (em `internal/billing/infrastructure/repositories/postgres/` ou pasta de migrations) confirmando seed dos planos e rejeição de 2ª sub ativa para mesmo user.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `migrations/0004_create_billing_plans.{up,down}.sql`
- `migrations/0005_create_billing_subscriptions.{up,down}.sql`
- `migrations/0006_create_billing_processed_events.{up,down}.sql`
- `migrations/0007_create_billing_kiwify_events.{up,down}.sql`
- `migrations/0008_create_billing_reconciliation_checkpoints.{up,down}.sql`
- `migrations/0009_create_identity_entitlements.{up,down}.sql`
- Referência: `.specs/prd-billing-pipeline/techspec.md` §6.7 e §11.3.
