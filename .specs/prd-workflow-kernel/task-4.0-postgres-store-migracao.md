# Tarefa 4.0: Adapter Postgres + migração 000019 (CAS + índice parcial)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o adapter Postgres da porta `Store` sobre `database.DBTX`, a migração
`000019_create_workflow_runtime` (tabelas `workflow_runs` + `workflow_steps`), o lock otimista por
versão (CAS) e o índice parcial único de run ativo. Cobrir durabilidade e concorrência com integration
tests reais (testcontainers).

<requirements>
- RF-07: estado de Run/Step persistido em `workflow_runs`/`workflow_steps` via uow, migrations versionadas.
- RF-09: resume sobrevive a restart/crash (prova de durabilidade com banco real).
- RF-10: concorrência por lock otimista por versão (CAS) + índice parcial único de run ativo.
</requirements>

## Subtarefas

- [ ] 4.1 `migrations/000019_create_workflow_runtime.{up,down}.sql`: tabelas no schema `mecontrola`,
  cabeçalho `SET LOCAL lock_timeout/statement_timeout`, `fillfactor=70`, CHECK de status, índice parcial
  único `(workflow, correlation_key) WHERE status IN ('running','suspended')`, índice `(run_id, seq)`.
  `down` idempotente.
- [ ] 4.2 `internal/platform/workflow/infrastructure/postgres/store.go` + `nullable.go`: implementar
  `Store` (Insert/Load/Save com `version = version + 1 WHERE id=$ AND version=$expected` →
  `RowsAffected()==0 ⇒ ErrVersionConflict`/AppendStep/DeleteCompleted), resolvendo tx via
  `database.FromContext` no molde de `idempotency.postgresStorage.conn`.
- [ ] 4.3 `factory.go`: `NewStoreFactory(o11y)` no padrão de `RepositoryFactory` do agent.
- [ ] 4.4 Integration tests (`//go:build integration`, testcontainers): Start→Suspend→(reabrir
  Engine/Store)→Resume sem duplicar `persist`; dois `Resume` concorrentes ⇒ exatamente um vence (CAS);
  migração up/down idempotente.

## Detalhes de Implementação

Ver techspec.md → "Modelos de Dados" (DDL completo) e "Testes de Integração". Espelhar
`agent_run_repository.go` (INSERT/UPDATE + `RowsAffected`) e `card_repository.UpdateLimitByIDForUser`
(CAS por versão). `defer func(){ _ = rows.Close() }()` obrigatório.

## Critérios de Sucesso

- Migração aplica/reverte limpa; índice parcial único impede dois runs ativos por chave.
- `Save` rejeita escrita com versão desatualizada (`ErrVersionConflict`).
- Integration tests de durabilidade e concorrência verdes.
- Adapter sem regra de domínio (gate R-WF-KERNEL-001).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/000019_create_workflow_runtime.up.sql`, `.down.sql` (novos)
- `internal/platform/workflow/infrastructure/postgres/store.go`, `nullable.go` (novos)
- `internal/platform/workflow/factory.go` (novo)
- `internal/platform/workflow/infrastructure/postgres/store_integration_test.go` (novo)
