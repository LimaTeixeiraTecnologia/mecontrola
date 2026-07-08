<!-- spec-hash-prd: c1abc3c8be24b9e5faaaf7b0f8db62062550ea9b22f65c11132551cf68fe2b0d -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica

## Resumo Executivo

Esta especificação propõe endurecer a malha de testes de `internal/agents` por contratos mínimos determinísticos no baseline padrão, preservando suites `integration` e `realllm` como evidência complementar. A abordagem evita dois falsos positivos atuais: cobertura declarada por contagem estática de tools e validação agentiva dependente apenas de provider real.

O desenho técnico é dividido em quatro frentes alinhadas ao PRD: jobs e `write_ledger_repository`, `transactions_ledger_adapter`, inventário/harness de tools e invariantes agentivos offline. A implementação deve reaproveitar seams reais do sistema, especialmente `buildFinancialTools`, `BuildGoalStep`, `BuildMeControlaAgent` e o runtime `internal/platform/agent`, em vez de duplicar lógica em fakes permissivos.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes modificados:

- `internal/agents/infrastructure/jobs/handlers/confirm_reaper_job_test.go`
- `internal/agents/infrastructure/jobs/handlers/ledger_retention_job_test.go`
- `internal/agents/infrastructure/persistence/write_ledger_repository_test.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter_test.go`
- `internal/agents/module_test.go`
- `internal/agents/application/scorers/mecontrola_tools_realllm_test.go`
- `internal/agents/application/scorers/mecontrola_scorers.go`
- `internal/agents/application/agents/*_offline*_test.go` ou arquivo equivalente dedicado à camada agentiva offline
- `internal/agents/application/workflows/*_test.go` apenas quando necessário para complementar `BuildGoalStep`

Responsabilidades:

- Jobs continuam finos; a mudança é apenas adicionar prova offline de defaults e propagação.
- `write_ledger_repository` ganha suíte determinística do contrato do adapter sem substituir a suíte com Postgres real.
- `transactions_ledger_adapter_test.go` deixa de provar só resumo/listagem e passa a cobrir as 9 operações públicas da interface `TransactionsLedger`.
- O inventário de tools passa a ser derivado da composição real em `buildFinancialTools`, não de listas duplicadas em scorer/harness.
- A camada agentiva offline passa a testar o runtime real `agent.Agent` com provider roteirizado e tools reais/fakes, preservando `realllm` apenas como smoke complementar.

Fluxo de dados alvo:

1. O módulo monta tools reais em `buildFinancialTools`.
2. O harness de cobertura deriva `tool.ID()` dos handles reais e valida paridade com a matriz de cenários obrigatórios.
3. O runtime `agent.Agent` recebe provider roteirizado nos testes offline e produz `ToolCalls`, `ToolOutcome`, `Content` e `RawJSON`.
4. Suites `integration` e `realllm` continuam exercitando banco real e provider externo, mas fora do gate mínimo do baseline.

## Design de Implementação

### Interfaces Chave

Interfaces contratuais relevantes já existentes:

```go
type WriteLedgerRepository interface {
    FindByKey(ctx context.Context, wamid string, itemSeq int, operation string) (WriteLedgerEntry, error)
    Insert(ctx context.Context, entry WriteLedgerEntry) error
    DeleteBefore(ctx context.Context, before time.Time, batchSize int) (int64, error)
}
```

```go
type TransactionsLedger interface {
    CreateTransaction(ctx context.Context, in RawTransaction) (EntryRef, error)
    UpdateTransaction(ctx context.Context, in RawUpdateTransaction) (EntryRef, error)
    CreateRecurringTemplate(ctx context.Context, in RawRecurringTemplate) (EntryRef, error)
    DeleteTransaction(ctx context.Context, ref EntryRef, version int64) error
    ListMonthlyEntries(ctx context.Context, userID uuid.UUID, refMonth string, cursor string, limit int) ([]MonthlyEntry, error)
    GetMonthlySummary(ctx context.Context, userID uuid.UUID, refMonth string) (MonthlySummary, error)
    GetCardInvoice(ctx context.Context, cardID uuid.UUID, refMonth string) (CardInvoice, error)
    SearchTransactions(ctx context.Context, userID uuid.UUID, query, refMonth string, limit int) ([]Entry, error)
    GetTransaction(ctx context.Context, txID string) (Entry, error)
}
```

Novos helpers de teste recomendados:

```go
type toolCoverageScenario struct {
    ToolID string
    Input  string
    Assert func(t *testing.T, result agent.Result, captures Captures)
}
```

```go
type scriptedProviderStep struct {
    Match  func(req llm.Request) bool
    Reply  llm.Response
    Err    error
}
```

Diretrizes:

- Helpers de teste ficam nos próprios arquivos `_test.go` ou em `test_helpers.go` dentro do mesmo package, sem vazar abstrações para produção.
- O provider roteirizado deve operar no boundary `llm.Provider`, nunca reimplementar parsing semântico do agente.
- O harness de inventário deve consumir `buildFinancialTools(...)` diretamente com doubles mínimos das dependências.

### Modelos de Dados

Modelos relevantes à estratégia:

- `usecases.WriteLedgerEntry` é a fixture canônica para o repositório; usar UUIDs e `CreatedAt` fixos.
- `agentsifaces.Entry`, `CardInvoice`, `MonthlyEntry`, `MonthlySummary` e `EntryRef` são os oráculos de mapeamento do adapter.
- `agent.Result` é o oráculo da camada offline agentiva:
  - `ToolCalls` valida sequência e IDs.
  - `ToolOutcome` valida falha de tool sem falso sucesso.
  - `RawJSON` valida extrações estruturadas de onboarding quando o teste for em `BuildGoalStep`.
  - `Content` valida guardrails textuais determinísticos.

Matriz de cobertura por componente:

- Jobs:
  - `ConfirmReaperJob`: `Name`, `Schedule`, `Timeout`, override, `Run(ctx)` propaga erro.
  - `LedgerRetentionJob`: `Name`, `Schedule`, `Timeout`, override, `Run(ctx)` propaga erro.
- `write_ledger_repository`:
  - `FindByKey` sucesso.
  - `FindByKey` `sql.ErrNoRows -> ErrLedgerEntryNotFound`.
  - `FindByKey` erro genérico envelopado.
  - `FindByKey` usa tx do contexto quando presente.
  - `Insert` SQL contém `ON CONFLICT (wamid, item_seq, operation) DO NOTHING`.
  - `Insert` `UniqueViolation -> nil`.
  - `Insert` erro genérico envelopado.
  - `DeleteBefore` sucesso retorna `RowsAffected`.
  - `DeleteBefore` erro no `ExecContext`.
  - `DeleteBefore` erro em `RowsAffected`.
- `transactions_ledger_adapter`:
  - Todas as 9 operações com `success`, `downstream error`, `principal ausente/inválido`.
  - Casos específicos de `SubcategoryID=nil`, `kind inválido`, item inesperado, forwarding de `Version`, `CardID`, `Frequency`, `Reconciled`.
- Harness de tools:
  - igualdade exata entre `tool.ID()` reais e chaves da matriz obrigatória.
  - separação entre `coverageByTool` e `routingScenarios`.
- Agentivo offline:
  - onboarding objetivo+valor: `suspend`, `GoalValueAsked`, `complete`, `GoalValueCents`.
  - honestidade em falha de tool: `ToolOutcomeUsecaseError`, ausência de mensagem de sucesso.
  - roteamento mínimo: C1, C4, C5 e um cenário de escrita financeira principal.

### Endpoints de API

Não se aplica. O escopo é interno de engenharia, sem novos endpoints.

## Pontos de Integração

Integrações relevantes:

- Postgres real permanece na suíte `write_ledger_repository_integration_test.go`.
- OpenRouter/`llm.Provider` real permanece nas suites `realllm`.
- `internal/platform/agent` e `internal/platform/workflow` são consumidos como substrato real nos testes offline; não devem ser substituídos por harness paralelo incompatível.

Tratamento de erros:

- Erros de adapter e persistência devem continuar wrapped com prefixos estáveis já existentes.
- Nos testes, usar `errors.Is`/`errors.As` e asserts de prefixo para garantir semântica, não comparar mensagem completa quando houver wrapping adicional.

## Abordagem de Testes

### Testes Unitários

Estratégia:

- Table-driven tests para jobs, repositório e adapter.
- `sqlmock` para `FindByKey` e validação de SQL/argumentos do repositório.
- `database/mocks.MockDBTX` e `MockResult` para `conn(ctx)` e `RowsAffected()`.
- Provider roteirizado ou `llmmocks.Provider` para camada agentiva offline.
- Fakes mínimos por contrato, nunca por conveniência ampla.

Requisitos de mock/double:

- Jobs: spies simples de `Reap(ctx)` e `Execute(ctx)`.
- Ledger repository: `sqlmock`, `pgconn.PgError`, `MockResult`.
- Adapter: mocks de use cases reais do módulo `transactions`, com verificação de payload exato.
- Tools/harness: fakes determinísticos de `TransactionsLedger`, `CardManager`, `BudgetPlanner`, `CategoriesReader`, `RecurrenceManager`, `RegisterAttempt`.
- Agentivo offline: provider roteirizado por `Schema.Name`, conteúdo das mensagens e sequência esperada de tool calls.

Cenários críticos:

- RF-01 a RF-03: contratos mínimos no baseline padrão.
- RF-04 e RF-05: 9 métodos da interface com identidade obrigatória e edge cases de transformação.
- RF-06 a RF-08: falha automática por drift estrutural de inventário.
- RF-09 a RF-12: oráculos determinísticos no boundary do runtime agentivo.

### Testes de Integração

Sim, o projeto precisa de integration tests porque:

- há fronteiras de IO críticas de banco;
- já existe distinção real entre baseline e `integration`;
- há comportamento de unicidade/concorrência que mocks não capturam com fidelidade.

Escopo de integração mantido:

- `write_ledger_repository_integration_test.go` continua cobrindo roundtrip real, unicidade, concorrência e `DeleteBefore`.
- Suites `realllm` continuam cobrindo aderência externa do provider, especialmente onboarding, cadeia de tools e smoke de guardrails.

Regra:

- Nenhum caso hoje coberto por `integration` ou `realllm` deve ser removido por esta iniciativa.
- Casos movidos para baseline padrão não substituem a camada complementar; eles eliminam a dependência exclusiva.

### Testes E2E

Não se aplica. O runtime local do agente e os testes de integração/realllm já cobrem o nível necessário para esta iniciativa.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. Endurecer jobs e `write_ledger_repository`.
   Motivo: menor superfície, dependência direta de RF-01 a RF-03 e efeito imediato no baseline.
2. Expandir `transactions_ledger_adapter_test.go`.
   Motivo: fecha 7 métodos públicos sem prova padrão e reduz fragilidade de bindings.
3. Corrigir a fonte de verdade do inventário de tools.
   Motivo: evita continuar medindo cobertura com denominador errado.
4. Introduzir a camada agentiva offline com provider roteirizado.
   Motivo: é a parte com maior acoplamento conceitual e depende de entender bem o harness já ajustado.
5. Ajustar scorers/harnesses complementares e validar regressão ampla.

### Dependências Técnicas

- `ai-spec` para hash e drift de spec já está disponível.
- `sqlmock` já está no `go.mod` e deve ser reutilizado.
- Mocks existentes de `transactions`, `database`, `llm` e `outbox` devem ser priorizados sobre novas abstrações.
- Nenhuma nova dependência de produção é necessária.

## Monitoramento e Observabilidade

Sinais a capturar nos testes e na futura implementação:

- `agent.Result.ToolCalls` por cenário.
- `agent.Result.ToolOutcome` quando houver falha de tool.
- `workflow.StepStatus` e `workflow.SuspendAwaitingInput` no onboarding.
- `errors.Is` para `ErrLedgerEntryNotFound`.
- Prefixos estáveis de erro:
  - `agents.persistence.write_ledger.find_by_key:`
  - `agents.persistence.write_ledger.insert:`
  - `agents.persistence.write_ledger.delete_before:`
  - `agents.persistence.write_ledger.delete_before.rows_affected:`
  - `agents/binding/transactions_ledger:`

Observabilidade operacional:

- Não há exigência de novas métricas de produção.
- O uso de `fake.NewProvider()` nos testes deve preservar spans/logs para diagnóstico local sem introduzir side effects externos.

## Considerações Técnicas

### Decisões Chave

- ADR-001: baseline offline-first por contrato, mantendo `integration`/`realllm` como prova complementar.
- ADR-002: inventário de tools derivado de `buildFinancialTools` como fonte única de verdade.
- ADR-003: camada agentiva offline no boundary `llm.Provider`/`agent.Agent`, não em harness paralelo ou keyword scorer.

Mapeamento requisito -> decisão -> teste:

- RF-01, RF-02, RF-03:
  - decisão: contratos mínimos offline + integração complementar.
  - testes: jobs dedicados + `write_ledger_repository_test.go` + manutenção da suíte `integration`.
- RF-04, RF-05:
  - decisão: matriz completa das 9 operações da interface `TransactionsLedger`.
  - testes: table-driven por método e matriz transversal de `principalCtx`.
- RF-06, RF-07, RF-08:
  - decisão: inventário real derivado de `buildFinancialTools` com conjunto exato de IDs.
  - testes: paridade `actualIDs == keys(coverageByTool)` e `routingScenarios` fora do denominador.
- RF-09, RF-10, RF-11, RF-12:
  - decisão: provider roteirizado e runtime real para onboarding, honestidade e roteamento mínimo.
  - testes: `BuildGoalStep` com extração estruturada e `BuildMeControlaAgent` com sequência explícita de tool calls.

Alternativas rejeitadas:

- Duplicar inventário manual em scorer/harness.
- Validar apenas contagem total de tools.
- Usar keyword scorers como gate principal.
- Espelhar integralmente a suíte `integration` no baseline.
- Criar fake de workflow/agent que não usa o runtime real.

### Riscos Conhecidos

- Overfitting do provider roteirizado a transcripts exatos.
  - mitigação: validar estrutura semântica do request, sequência de tool calls e campos mínimos, não string completa.
- Drift futuro entre lista de tools e harness complementar `realllm`.
  - mitigação: derivar IDs dos handles reais e reaproveitar o mesmo helper em suites offline/complementares quando possível.
- Testes do adapter ficarem excessivamente acoplados à implementação de use cases.
  - mitigação: verificar payloads públicos e wrappers de erro, não detalhes internos do módulo `transactions`.
- SQL regex permissiva demais no repositório.
  - mitigação: asserts de fragmentos críticos (`WHERE wamid = $1`, `ON CONFLICT (wamid, item_seq, operation) DO NOTHING`).

### Conformidade com Padrões

- `AGENTS.md` do repositório como fonte canônica.
- `R-TEST-001`: determinismo, sem rede real no baseline, happy path + falha + edge case.
- `R-ERR-001`: wrapping com contexto e sem engolir erro.
- `R-SEC-001`: escrita auditável, sem segredos, sem dependência externa obrigatória no baseline.
- Regras do módulo:
  - adapters permanecem finos;
  - jobs permanecem porta de entrada fina;
  - baseline padrão é o gate mínimo.

### Arquivos Relevantes e Dependentes

- `.specs/prd-auditoria-testes-internal-agents/prd.md`
- `.specs/prd-auditoria-testes-internal-agents/adr-001-baseline-offline-first.md`
- `.specs/prd-auditoria-testes-internal-agents/adr-002-inventario-real-tools.md`
- `.specs/prd-auditoria-testes-internal-agents/adr-003-seam-agentivo-offline.md`
- `internal/agents/module.go`
- `internal/agents/module_test.go`
- `internal/agents/application/scorers/mecontrola_scorers.go`
- `internal/agents/application/scorers/mecontrola_tools_realllm_test.go`
- `internal/agents/application/agents/mecontrola_agent_test.go`
- `internal/agents/application/agents/onboarding_goal_value_realllm_test.go`
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `internal/agents/infrastructure/jobs/handlers/confirm_reaper_job.go`
- `internal/agents/infrastructure/jobs/handlers/ledger_retention_job.go`
- `internal/agents/infrastructure/persistence/write_ledger_repository.go`
- `internal/agents/infrastructure/persistence/write_ledger_repository_integration_test.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter_test.go`
