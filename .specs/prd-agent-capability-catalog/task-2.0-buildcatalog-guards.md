# Tarefa 2.0: BuildCatalog (24 specs) + guards de cobertura e consistência

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Declarar `BuildCatalog(...)` em `internal/agent/application/capability/build.go` com as **24 capabilities** (19 kinds roteáveis de `routableKinds()` + `KindUnknown` + 5 kinds destrutivos de `intentToOperationKind`), atribuindo o `WorkflowID` real (owner do registry) — incluindo a **correção de drift** (D-02/ADR-002) para `KindQueryIncomeSummary`→`transactions`, `KindBudgetRecurrence`→`budget`, `KindDeleteTransactionByRef`/`KindEditTransactionByRef`→destrutivo. Adicionar o teste-guard de cobertura e o teste de consistência catálogo↔registry (D-01/R2).

<requirements>
- RF-03: `BuildCatalog` declara as specs a partir do mesmo conjunto de kinds do wiring (registro único alimentando classificação).
- RF-06: cada spec carrega classificação operacional mínima (`Mode`, `RequiresConfirmation`, `SupportsSuspend`, `SupportsResume`, `Channels=["whatsapp"]`, `MetricsKey` espelhando `ToolName` — D-04).
- RF-10: teste-guard garante que todo `kind ∈ routableKinds()` tem `CapabilitySpec` (falha o build na ausência).
- D-01: teste de consistência: para todo kind roteável, `catalog.Classify(kind).workflow == ID do workflow que IntentRegistry.Resolve(kind) possui`.
</requirements>

## Subtarefas

- [ ] 2.1 `build.go`: `BuildCatalog()` retornando `*Catalog` com as 24 `CapabilitySpec` (labels reais; `ToolName`/`MetricsKey` = `kind.String()` por kind roteável, `""` para `KindUnknown`).
- [ ] 2.2 Marcar `Mode: ModeWrite` + `RequiresConfirmation: true` para os 5 destrutivos e para as escritas; `ModeRead` para consultas; `SupportsSuspend`/`SupportsResume` conforme o kind.
- [ ] 2.3 Teste-guard de cobertura: itera `routableKinds()` e exige `Lookup(kind).ok == true`.
- [ ] 2.4 Teste de consistência registry↔catálogo: para cada kind roteável, comparar `WorkflowID` com o owner real de `IntentRegistry.Resolve`.

## Detalhes de Implementação

Ver `techspec.md` → "Modelos de Dados" (tabela das 24 capabilities + tabela de drift) e ADR-001/ADR-002. Os `WorkflowID` válidos: `transactions`, `budget`, `cards`, `conversational`. Fonte dos destrutivos: `intentToOperationKind` (`daily_ledger_agent.go:648-654`). `MetricsKey` espelha `ToolName` (D-04). O teste de consistência pode instanciar o registry via o wiring existente ou um helper de teste que reproduza `buildRegistry()`.

## Critérios de Sucesso

- 24 capabilities registradas sem erro de `NewCatalog`.
- Teste-guard falha se um kind roteável novo for adicionado sem spec.
- Teste de consistência verde: nenhum `WorkflowID` diverge do owner real do registry.
- Drift dos 4 kinds refletido nos `WorkflowID` corretos (não no label legado).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — declara o mapeamento kind→workflow→tool e classificação de capabilities do `internal/agent`; gatilho da skill acionado.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/capability/build.go` (novo)
- `internal/agent/application/capability/build_test.go`, `guard_test.go` (novos)
- Referência: `internal/agent/application/services/agent_workflows.go` (`buildRegistry`/`routableKinds`), `daily_ledger_agent.go` (`intentToOperationKind`)
