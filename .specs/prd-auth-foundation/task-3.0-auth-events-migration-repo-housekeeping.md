# Tarefa 3.0: Migrations 0014/0015 + auth_events_repository (UUID v7) + housekeeping job

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Entrega o substrato persistente da fundação de auth: a tabela `auth_events` (PK UUID v7, schema mínimo com `reason`), o repositório Postgres com `Insert`/`AnonymizeByUserID`/`DeleteOlderThan`, o seed condicional de staging para smoke test, e o housekeeping job mensal que apaga linhas > 180 dias em lotes de 10k.

<requirements>
- RF-09: migration `0014` criando `auth_events` com CHECK constraints (`kind`, `source`, `reason` incluindo `invalid_payload`) e índices parciais.
- RF-22: housekeeping job mensal com batching, idempotência e métricas.
- RF-24: numeração alocada lendo `migrations/` no momento (confirmar `0014` ainda disponível).
- RF-33: `invalid_payload` no CHECK de `reason`.
- RF-36: migration `0015` de seed condicional para staging com user fixo + `STAGING_SMOKE_WA`.
</requirements>

## Subtarefas

- [ ] 3.1 Verificar `ls migrations/ | tail -3` para confirmar `0014` ainda disponível; se não, alocar próximo livre e atualizar techspec.
- [ ] 3.2 Criar `migrations/0014_create_identity_auth_events.up.sql` exatamente como na techspec (UUID + CHECKs + 2 índices parciais).
- [ ] 3.3 Criar `migrations/0014_create_identity_auth_events.down.sql` com `RENAME TO auth_events_archived_20260608`.
- [ ] 3.4 Criar `migrations/0015_seed_smoke_user_staging.up.sql` com bloco `DO $$` condicional a `current_database() ~ 'staging'` + idempotência por UUID fixo.
- [ ] 3.5 Criar `migrations/0015_seed_smoke_user_staging.down.sql` apagando apenas o UUID fixo.
- [ ] 3.6 Criar entidade `internal/identity/domain/entities/auth_event.go` (struct + factory que gera UUID via `uuid.NewV7()`).
- [ ] 3.7 Criar `internal/identity/application/interfaces/auth_events_repository.go` (porta).
- [ ] 3.8 Criar `internal/identity/infrastructure/repositories/postgres/auth_events_repository.go` com métodos `Insert`, `AnonymizeByUserID`, `DeleteOlderThan(ctx, cutoff, batchSize)`.
- [ ] 3.9 Criar `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_job.go` seguindo padrão de jobs em billing/onboarding; emite métricas `auth_events_housekeeping_deleted_total` (counter) e `auth_events_housekeeping_duration_seconds` (histograma); roda em lotes de 10k.
- [ ] 3.10 Integration tests com framework detectado em 1.0 (PRE-01): inserts, idempotência por PK, AnonymizeByUserID, DeleteOlderThan em batches, CHECK constraints disparam em `kind`/`source`/`reason` inválidos.

## Detalhes de Implementação

Ver techspec `## Design de Implementação > Modelos de Dados` para SQL exato. UUID v7 via `github.com/google/uuid v1.6.0` já em `go.mod` — usar `uuid.NewV7()`. Repositório segue padrão ADR-008 (`repository-factory-per-module`) e ADR-007 (índices parciais).

## Critérios de Sucesso

- `task migrate-up` aplica `0014` e `0015` sem erro em dev e staging.
- `task migrate-down` reverte preservando dados (rename, não drop).
- CHECK constraints disparam corretamente em testes de integração.
- Housekeeping job apaga 25k linhas test em 3 lotes verificáveis; segunda execução é no-op.
- `STAGING_SMOKE_WA` configurado via `ALTER DATABASE staging SET app.smoke_wa = '+5511…'` cria o user fixo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários do repositório com mock DBTX para SQL paths
- [ ] Testes de integração com PG real (framework de 1.0 PRE-01)
- [ ] Microbenchmark `BenchmarkInsertAuthEvent` (alvo < 1 ms incluindo round-trip)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/0014_create_identity_auth_events.up.sql` (criar)
- `migrations/0014_create_identity_auth_events.down.sql` (criar)
- `migrations/0015_seed_smoke_user_staging.up.sql` (criar)
- `migrations/0015_seed_smoke_user_staging.down.sql` (criar)
- `internal/identity/domain/entities/auth_event.go` (criar)
- `internal/identity/application/interfaces/auth_events_repository.go` (criar)
- `internal/identity/infrastructure/repositories/postgres/auth_events_repository.go` + `_test.go` + `_integration_test.go` (criar)
- `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_job.go` + `_test.go` + `_integration_test.go` (criar)
- `internal/identity/infrastructure/repositories/factory.go` (atualizar — adicionar `AuthEventsRepository(tx)`)
