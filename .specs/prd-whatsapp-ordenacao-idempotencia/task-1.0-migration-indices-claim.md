# Tarefa 1.0: Migration 000003 — índices parciais do claim particionado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a migration `000003` que adiciona os dois índices parciais que sustentam o claim particionado
por usuário no `outbox_events`, sem downtime e sem colunas novas (reaproveita `aggregate_user_id` e
`occurred_at`, que já existem). É a base de tudo (RF-01).

<requirements>
- Índice de suporte à varredura por usuário pendente, ordenado por chegada:
  `outbox_events_user_pending_occurred_idx (aggregate_user_id, occurred_at) WHERE status = 1 AND aggregate_user_id IS NOT NULL`.
- Backstop "1 em voo por usuário":
  `outbox_events_user_inflight_uidx (aggregate_user_id) WHERE status = 2 AND aggregate_user_id IS NOT NULL` (UNIQUE).
- Usar `CREATE INDEX IF NOT EXISTS` / `CREATE UNIQUE INDEX IF NOT EXISTS` (idempotente).
- Pré-condição verificada: a tabela nasce com 0 linhas `status = 2` em produção; o índice único parcial é seguro. Registrar a checagem `SELECT count(*) FROM mecontrola.outbox_events WHERE status = 2` (deve ser 0) antes de aplicar.
- `.down.sql` dropa exatamente os dois índices (`DROP INDEX IF EXISTS`), restaurando o comportamento antigo.
- Schema `mecontrola`; nomes de tabela/coluna conforme produção `mastra-20260629` (verificado).
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/000003_*.up.sql` com os dois `CREATE INDEX IF NOT EXISTS` (parciais).
- [ ] 1.2 Criar `migrations/000003_*.down.sql` com os dois `DROP INDEX IF EXISTS`.
- [ ] 1.3 Documentar a checagem de pré-condição (0 linhas `status=2`) no cabeçalho da execução/runbook.
- [ ] 1.4 Validar aplicação/rollback em Postgres local (migrate up/down idempotente).

## Detalhes de Implementação

Ver techspec.md §Modelos de Dados (bloco SQL `outbox_events_user_pending_occurred_idx` e
`outbox_events_user_inflight_uidx`) e ADR-001 §Plano de Implementação (item 1). O índice único é o
backstop se dois dispatchers colidirem; o índice de suporte reduz o custo do `NOT EXISTS` por usuário.

## Critérios de Sucesso

- `migrate up` cria os dois índices; reexecução é no-op (idempotente).
- `migrate down` remove os dois índices sem afetar outras estruturas.
- Nenhum lock longo/rewrite de tabela (índice parcial sobre tabela vazia/pequena).
- Sem colunas novas em `outbox_events`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Validação: aplicar `up`/`down` em Postgres local (ou testcontainer) e confirmar presença/ausência dos
índices via `pg_indexes`; confirmar idempotência reexecutando `up`.

## Rollback

`migrate down` (`.down.sql`) dropa os dois índices; o `ClaimBatch` antigo (`ORDER BY next_attempt_at`)
volta a funcionar sem dependência de índice — restaura o comportamento pré-tarefa.

## Done-when

- Índices presentes em `pg_indexes` após `up`, ausentes após `down`.
- Reexecução de `up` não falha (idempotente).
- Checagem de pré-condição registrada.

## Arquivos Relevantes
- `migrations/000003_*.up.sql` (novo)
- `migrations/000003_*.down.sql` (novo)
- `internal/platform/outbox/status.go` (referência do enum de status: 1=pending, 2=processing)
