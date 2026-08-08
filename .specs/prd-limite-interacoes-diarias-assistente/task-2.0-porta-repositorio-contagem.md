# Tarefa 2.0: Porta de contagem e repositório Postgres com teste de integração

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a porta privada `dailyInteractionCounter` no pacote `usecases` e sua implementação Postgres em `internal/agents/infrastructure/repositories/postgres`, executando a query de contagem sobre `mecontrola.platform_runs` servida pelo índice existente, sem nenhuma migration ou mudança de schema.

<requirements>
- RF-02: contagem usando os Runs de `mecontrola.platform_runs` por `resource_id` e `started_at` dentro do dia corrente, sem criar tabela ou migration nova.
</requirements>

## Subtarefas

- [ ] 2.1 Declarar a interface privada `dailyInteractionCounter` com `CountStartedSince(ctx context.Context, resourceID string, since time.Time) (int, error)` em arquivo próprio no pacote `usecases`, padrão de `usecases/write_ledger.go:24-28`
- [ ] 2.2 Implementar `NewDailyInteractionCounter(db database.DBTX, o11y observability.Observability)` em `internal/agents/infrastructure/repositories/postgres/daily_interaction_counter.go`, padrão de `audit_reconciliation.go:19-21`
- [ ] 2.3 Query `SELECT COUNT(*) FROM mecontrola.platform_runs WHERE resource_id = $1 AND started_at >= $2` com `database/sql` puro, placeholders `$N` e erros com wrap `%w` e contexto estável em lowercase (R-ERR-001)
- [ ] 2.4 Criar `daily_interaction_counter_integration_test.go` com build tag `//go:build integration`, pacote externo, suite testify e banco via `testcontainer.Postgres` (`internal/platform/testcontainer/postgres.go:13-16`)

## Detalhes de Implementação

- Seções `Interfaces Chave` e `Modelos de Dados` da techspec.md definem assinatura da porta e a query exata.
- ADR-002 (`adr-002-contagem-via-platform-runs-com-porta-no-consumidor.md`) registra a decisão de porta no consumidor e as alternativas rejeitadas (tabela de quota, extensão de `agent.RunStore`).
- `postgresql-production-standards` não é acionada: não há migration, tabela, coluna, índice, constraint, role ou mudança de schema; a query é ad-hoc sobre tabela e índice existentes, conforme critério de gatilho do AGENTS.md.

## Critérios de Sucesso

- Query coberta pelo índice `platform_runs_resource_started_idx` (`migrations/000001_initial_schema.up.sql:2390-2391`), sem estrutura nova.
- Erros de banco propagados com wrap `%w`, sem sentinel novo desnecessário.
- `go build ./...`, `go vet ./...`, `go test -race -count=1 ./internal/agents/...` e `go test -tags integration ./internal/agents/infrastructure/repositories/postgres/...` verdes.
- `task ci:agent-boundary` sem violação (SQL somente dentro de repositório).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: não aplicável além da compilação, pois a lógica é a query; decisão fica na tarefa 3.0
- [ ] Testes de integração: conta apenas Runs do `resource_id` informado; conta apenas Runs com `started_at` dentro da janela; retorna zero sem erro quando não há Runs; fronteira exata do `since` é inclusiva

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/usecases/daily_interaction_counter.go` (porta nova)
- `internal/agents/infrastructure/repositories/postgres/daily_interaction_counter.go` (novo)
- `internal/agents/infrastructure/repositories/postgres/daily_interaction_counter_integration_test.go` (novo)
- `internal/agents/infrastructure/repositories/postgres/audit_reconciliation.go` (padrão de referência)
- `migrations/000001_initial_schema.up.sql` (somente leitura, evidência de tabela e índice)
