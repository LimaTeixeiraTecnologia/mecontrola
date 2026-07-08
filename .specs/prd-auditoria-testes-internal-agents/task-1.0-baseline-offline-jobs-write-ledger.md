# Tarefa 1.0: Endurecer baseline offline de jobs e `write_ledger_repository`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar a malha determinística mínima para `ConfirmReaperJob`, `LedgerRetentionJob` e `write_ledger_repository`, preservando as suites `integration` como evidência complementar. Quando houver mocks, o uso de `.mockery.yaml` do repositório é obrigatório, e os testes devem seguir o padrão `testify/suite` com cenários table-driven no estilo aprovado pelo usuário.

<requirements>
- Cobrir RF-01, RF-02, RF-03 e RF-12.
- Criar testes offline dedicados para os dois jobs e para `write_ledger_repository`.
- Validar `Name`, `Schedule`, `Timeout` e propagação literal de erro dos jobs.
- Validar `FindByKey`, `Insert` e `DeleteBefore` com asserts de contrato, incluindo `sql.ErrNoRows`, `UniqueViolation`, erro genérico, `RowsAffected` e uso de tx via contexto.
- Manter `write_ledger_repository_integration_test.go` intacto como camada complementar.
- Usar `.mockery.yaml` quando a tarefa depender de mocks do repositório ou de `database.DBTX`.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `confirm_reaper_job_test.go` e `ledger_retention_job_test.go` com `testify/suite` e cenários table-driven.
- [ ] 1.2 Criar `write_ledger_repository_test.go` com `sqlmock`, mocks gerados via `.mockery.yaml` quando aplicável e fixtures determinísticas.
- [ ] 1.3 Validar que a suíte padrão cobre os contratos mínimos sem exigir `integration`.

## Detalhes de Implementação

Consultar `techspec.md`, especialmente:
- `## Modelos de Dados` para a matriz de cobertura de jobs e repositório.
- `## Abordagem de Testes` para uso de `sqlmock`, `MockDBTX`, `MockResult`, `pgconn.PgError` e asserts de prefixo de erro.
- `ADR-001` para a estratégia offline-first por contrato.

## Critérios de Sucesso

- Existem testes offline separados para `ConfirmReaperJob` e `LedgerRetentionJob`.
- Existe suíte offline para `write_ledger_repository` cobrindo sucesso, erro tipado, erro genérico, `RowsAffected` e tx via contexto.
- A solução usa `.mockery.yaml` quando houver geração/uso de mocks do repositório.
- Os testes seguem `testify/suite` + cenários table-driven no formato aprovado.
- Nenhuma suite `integration` é removida ou enfraquecida.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/agents/infrastructure/jobs/handlers/...`
- [ ] `go test -race -count=1 ./internal/agents/infrastructure/persistence/...`
- [ ] `go test -tags=integration -count=1 ./internal/agents/infrastructure/persistence/...` quando o ambiente estiver disponível

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/infrastructure/jobs/handlers/confirm_reaper_job.go`
- `internal/agents/infrastructure/jobs/handlers/ledger_retention_job.go`
- `internal/agents/infrastructure/persistence/write_ledger_repository.go`
- `internal/agents/infrastructure/persistence/write_ledger_repository_integration_test.go`
- `internal/agents/application/usecases/write_ledger.go`
- `internal/agents/application/usecases/purge_ledger.go`
