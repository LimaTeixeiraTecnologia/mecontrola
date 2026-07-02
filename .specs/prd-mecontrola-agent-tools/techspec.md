<!-- spec-hash-prd: 67cf1b9b69f5ca5b244b64ffaff8e62de3d91e22972f934e78bb3849711bba59 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Superfície de Tools do MeControla Agent

> PRD consumido: `.specs/prd-mecontrola-agent-tools/prd.md` (spec-version 2).
> Skills obrigatórias na implementação: `go-implementation` (Etapas 1–5 + checklist R0–R7) e `mastra`.
> Regras hard aplicáveis: R-ADAPTER-001, R-AGENT-WF-001, R-WF-KERNEL-001, R-DTO-VALIDATE-001,
> R-TESTING-001, R-TXN-004.

## Resumo Executivo

O `mecontrola-agent` hoje expõe 9 tools (`internal/agents/module.go:254-262`). O PRD prescreve um
conjunto-alvo de **15 tools novas** (RF-09..RF-18d), cada uma mapeada a um use case real já existente
em `internal/{budgets,card,categories,transactions}`. Nenhuma capacidade de domínio nova é criada;
a evolução é **exclusivamente na camada consumidora** `internal/agents` — port do padrão Mastra —
reutilizando o substrato `internal/platform/{agent,tool,workflow,scorer}` sem tocá-lo.

A estratégia é aditiva e segue os idiomas já estabelecidos no módulo, verificados no código:
(1) estender as 4 interfaces de consumidor com os métodos faltantes, mais uma nova interface coesa
`RecurrenceManager`; (2) estender os 4 binding adapters com os métodos correspondentes (adapter fino:
mapeia args → DTO do use case → chama `Execute` → mapeia retorno, com span e wrapping de erro);
(3) criar um arquivo de tool por capacidade seguindo `tool.NewTool[I,O]`; (4) para as 3 operações
destrutivas/sensíveis novas (`update_recurrence`, `delete_recurrence`, `update_card` com mudança de
dia de vencimento), reutilizar o workflow único `destructive-confirm` adicionando novos
`OperationKind` fechados e seus executores; (5) para a criação `create_recurrence`, reutilizar
`IdempotentWrite`; (6) reforçar a validação de seleção de tool com um scorer de tool esperada por
cenário, elevando a barra do scorer coarse atual para atender M-04 (≥ 0.90) e RF-29. Decisões
materiais estão registradas em ADR-001..ADR-004.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes **novos ou modificados** (todos em `internal/agents`, exceto onde indicado):

- **Interfaces de consumidor (modificadas)** `internal/agents/application/interfaces/`:
  `card_manager.go`, `transactions_ledger.go`, `budget_planner.go` ganham métodos novos; nova
  interface `recurrence_manager.go` (`RecurrenceManager`). `categories_reader.go` inalterada.
- **Binding adapters (modificados)** `internal/agents/infrastructure/binding/`:
  `card_manager_adapter.go`, `transactions_ledger_adapter.go`, `budget_planner_adapter.go` ganham
  campos/métodos; novo `recurrence_manager_adapter.go`.
- **Tools novas (15)** `internal/agents/application/tools/`: um arquivo por tool.
- **Estado de confirmação (modificado)** `internal/agents/application/workflows/confirm_state.go`:
  novos `OperationKind` (`OpUpdateRecurrence`, `OpDeleteRecurrence`, `OpUpdateCard`).
- **Workflow destrutivo (modificado)** `internal/agents/application/workflows/destructive_confirm_workflow.go`:
  dispatch por mapa `map[OperationKind]executeFn`, novos executores, mensagens e impact notes.
- **Scorers (modificados)** `internal/agents/application/scorers/mecontrola_scorers.go`: lista de
  tools atualizada + novo scorer de tool esperada por cenário; `internal/platform/scorer` ganha o
  campo `Args`/expected em `RunSample`/`ToolCallRecord` se necessário ao harness (ADR-002).
- **Wiring (modificado)** `internal/agents/module.go`: injeção dos novos use cases nos adapters,
  construção das tools em `buildFinancialTools`, e construção da nova `RecurrenceManager`.
- **Harness de validação (novo)** `internal/agents/.../*_realllm_test.go`: suíte de seleção de tool
  com LLM real (gated `RUN_REAL_LLM`) medindo M-04.

Fluxo de dados (inalterado no substrato): `InboundRequest → AgentRuntime.Execute → Agent.Execute
(loop tool-calling) → ToolHandle.Invoke → exec → binding → usecase.Execute → repo`. Tools destrutivas
suspendem via `Engine.Start(confirmDef)` e retomam por merge-patch antes de qualquer parse.

## Design de Implementação

### Interfaces Chave

Métodos novos nas interfaces de consumidor (assinaturas concretas; tipos de retorno agent-owned
mapeados dos DTOs dos módulos):

```go
// card_manager.go — adicionar
GetCard(ctx context.Context, cardID, userID uuid.UUID) (Card, error)
CountCards(ctx context.Context, userID uuid.UUID) (int64, error)
BestPurchaseDay(ctx context.Context, bank string, dueDay int) (BestPurchaseDay, error)
UpdateCard(ctx context.Context, in CardUpdate) (Card, error) // Nickname/Bank/DueDay opcionais (*)

// transactions_ledger.go — adicionar (leitura)
GetCardInvoice(ctx context.Context, cardID uuid.UUID, refMonth string) (CardInvoice, error)
SearchTransactions(ctx context.Context, userID uuid.UUID, query, refMonth string, limit int) ([]Entry, error)
GetTransaction(ctx context.Context, txID string) (Entry, error)
GetCardPurchase(ctx context.Context, purchaseID uuid.UUID) (Entry, error)
ListCardPurchases(ctx context.Context, cardID uuid.UUID, refMonth, cursor string, limit int) ([]Entry, error)
```

```go
// recurrence_manager.go — NOVA interface coesa (ADR-004)
type RecurrenceManager interface {
    CreateRecurrence(ctx context.Context, in RawRecurrence) (EntryRef, error)
    UpdateRecurrence(ctx context.Context, templateID string, in RawUpdateRecurrence) (EntryRef, error)
    DeleteRecurrence(ctx context.Context, templateID string, version int64) error
    ListRecurrences(ctx context.Context, activeOnly bool, cursor string, limit int) ([]Recurrence, error)
}

// budget_planner.go — adicionar
SuggestAllocation(ctx context.Context, totalCents int64, allocations []AllocationBP) ([]AllocationCents, error)
```

Cada método do adapter segue o idioma verificado (`card_manager_adapter.go:42-70`): abrir span
`agents.binding.<x>.<op>`, mapear args → DTO de input do use case, `Execute`, mapear retorno → tipo
agent-owned, `fmt.Errorf("agents/binding/<x>: <ação>: %w", err)`. Zero regra de negócio no adapter.

### Tools novas (15) — mapeamento tool → capacidade real

| Tool (id LLM) | UserId via | Delega a | Idempotência/gate |
|---|---|---|---|
| `list_cards` | input `userId` | `CardManager.ListCards` | leitura |
| `get_card` | input `userId` | `CardManager.GetCard` | leitura |
| `count_cards` | input `userId` | `CardManager.CountCards` | leitura |
| `best_purchase_day` | — (bank+dueDay) | `CardManager.BestPurchaseDay` | leitura/cálculo |
| `query_card_invoice` | input `userId`+cardId | `TransactionsLedger.GetCardInvoice` | leitura |
| `search_transactions` | input `userId` | `TransactionsLedger.SearchTransactions` | leitura |
| `get_transaction` | input `txId` | `TransactionsLedger.GetTransaction` | leitura |
| `get_card_purchase` | input `purchaseId` | `TransactionsLedger.GetCardPurchase` | leitura |
| `list_card_purchases` | input `userId`+cardId | `TransactionsLedger.ListCardPurchases` | leitura |
| `list_recurrences` | input | `RecurrenceManager.ListRecurrences` | leitura |
| `create_recurrence` | input `userId`+`wamid`+`itemSeq` | `RecurrenceManager.CreateRecurrence` via `IdempotentWrite` | idempotente (ADR-003) |
| `suggest_allocation` | — (totalCents+allocations) | `BudgetPlanner.SuggestAllocation` | leitura/cálculo |
| `update_recurrence` | runtime ctx | `destructive-confirm` → `OpUpdateRecurrence` | gate (ADR-001) |
| `delete_recurrence` | runtime ctx | `destructive-confirm` → `OpDeleteRecurrence` | gate (ADR-001) |
| `update_card` | runtime ctx | `destructive-confirm` → `OpUpdateCard` **se muda dia venc.**; senão direto | gate condicional (ADR-001) |

`best_purchase_day` espelha o use case (`Execute(bank, dueDay)`); para cartão existente o valor já
vem em `output.Card.BestPurchaseDay` via `get_card`/`list_cards`. `get_transaction` e
`get_card_purchase` são tools distintas — cada uma delega a um único use case de leitura
(`GetTransaction` / `GetCardPurchase`), sem branching de domínio, honrando RF-19. `update_card` sem
alteração de dia de vencimento (só apelido/banco) executa direto via `CardManager.UpdateCard`
(RF-18/D-02).

### Modelos de Dados

Sem migração de schema nova neste escopo. Reuso de:
- `platform_workflow_*` (Snapshot/StepRecord) para o gate `destructive-confirm` (já existente).
- Ledger de idempotência do agente (`agents_write_*`) via `IdempotentWrite` para `create_recurrence`.
- Tabelas dos módulos permanecem owned pelos módulos; o agente só as toca via binding→usecase.

Novos tipos agent-owned em `interfaces/` (structs planas espelhando os DTOs dos módulos, sem lógica):
`Card`, `BestPurchaseDay`, `CardInvoice`, `CardUpdate`, `Recurrence`, `RawRecurrence`,
`RawUpdateRecurrence`, `AllocationBP`, `AllocationCents`. Campos derivados de
`capability-map`/DTOs verificados (ex.: `Card{ID,UserID,Nickname,Bank,ClosingDay,DueDay,
BestPurchaseDay,...}`).

### Estado de confirmação — novos OperationKind (fechados)

`confirm_state.go`: estender o enum `OperationKind` (hoje `OpDeleteEntry=1`, `OpEditEntry=2`,
`OpDeleteCard=3`) com `OpUpdateRecurrence`, `OpDeleteRecurrence`, `OpUpdateCard`, atualizando
`String()`, `IsValid()` e `ParseOperationKind()`. `ConfirmState` já carrega `UpdatePayload`,
`TargetRef`, `TargetKind`, `Version`, `UserID` — suficientes para as novas operações (a carga de
update de recorrência/cartão é serializada em `UpdatePayload`, como `edit_entry` já faz). Nenhum
campo novo obrigatório; `TargetKind` passa a aceitar `"recurring_template"` e `"card"`.

O dispatch em `executeOperation` migra de `switch` para `map[OperationKind]func(ctx, ConfirmState,
deps) error` (ADR-001, aderente a R-AGENT-WF-001.1 — resolução por mapa, não switch de domínio).

## Pontos de Integração

- **OpenRouter (LLM):** único provider (`llm.NewOpenRouterProvider`), já wired. Nenhuma nova
  call-site de LLM fora das sancionadas (loop do agent, scorer LLM-judged). Tools são determinísticas.
- **Módulos de negócio:** consumidos exclusivamente via binding→usecase; sem SQL direto, sem tx
  compartilhada entre módulos (memória: agente pode ter persistência própria mas nunca compartilha tx).

## Abordagem de Testes

### Testes Unitários

- **Por tool (novo `<tool>_test.go`)**: exec testado com binding mockado (mockery), cobrindo:
  sucesso (mapeamento correto args→DTO e retorno→output), erro do use case (wrapping propagado),
  parsing inválido de `userId`/UUID, defaults de campos opcionais (ex.: `refMonth` corrente,
  `limit`), e — nas destrutivas — que o exec chama `engine.Start` com o `OperationKind` e
  `TargetKind` corretos e retorna `needsConfirmation=true` **sem** efetivar.
- **Confirm workflow**: novos casos para `OpUpdateRecurrence`/`OpDeleteRecurrence`/`OpUpdateCard`
  cobrindo confirm→executa, cancel→no-op, ambíguo→reprompt único, TTL→expira; asserção de que o
  executor chama o binding correto e limpa o run (sem suspenso órfão).
- **Adapters de binding**: teste de mapeamento args↔DTO com use case mockado.
- Padrão: para use cases dos módulos, R-TESTING-001 (testify/suite whitebox, `fake.NewProvider()`).
  Tools/adapters seguem o padrão já usado no módulo (mock de binding via mockery).

### Testes de Integração

> Critérios: fronteira de IO crítica (Postgres do workflow/ledger) onde mock não garante correção →
> **sim**. Já há suíte de workflow durável no repositório.

- Gate `destructive-confirm` end-to-end para as 3 novas operações via `testcontainers-go`
  (`//go:build integration`): Start → suspend persistido → Resume por merge-patch → efetivação →
  run concluído. Idempotência de `create_recurrence` (replay do mesmo `wamid|itemSeq|operation`
  não duplica).

### Testes E2E

- **Harness de seleção de tool com LLM real (mandatório, memória `feedback_realllm_validation`)**:
  `*_realllm_test.go` gated por `RUN_REAL_LLM=1` + credenciais `OPENROUTER_*` do `.env`. Conjunto
  canônico determinístico de cenários (1 tool esperada por cenário) cobrindo **todas** as 24 tools
  (9 atuais + 15 novas). Mede M-04 (acerto de tool esperada ≥ 0.90) e RF-29 (toda tool exercida ao
  menos uma vez). Mocks não contam como evidência.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Tipos + interfaces agent-owned** (`interfaces/`): structs planas e assinaturas novas; base para
   tudo. Sem comportamento — compila isolado.
2. **Binding adapters + wiring dos use cases** (`binding/` + `module.go`): injetar os use cases
   existentes nos adapters; nova `RecurrenceManager`. Valida que os módulos expõem os construtores.
3. **Tools de leitura (11)**: `list_cards`, `get_card`, `count_cards`, `best_purchase_day`,
   `query_card_invoice`, `search_transactions`, `get_transaction`, `get_card_purchase`,
   `list_card_purchases`, `list_recurrences`, `suggest_allocation`. Baixo risco, sem gate.
4. **`create_recurrence`** com `IdempotentWrite` (ADR-003).
5. **OperationKinds novos + executores no confirm workflow** (ADR-001) e as 3 tools destrutivas
   (`update_recurrence`, `delete_recurrence`, `update_card`).
6. **Registro em `buildFinancialTools` + instruções do agente** (RF-20): declarar todas as tools e
   quando usá-las.
7. **Scorers + harness de validação** (ADR-002): scorer de tool esperada, atualização da lista de
   tools, suíte real-LLM. Fecha M-03/M-04/RF-29.
8. **Mapa capacidade→tool e relatório de gaps** versionados (RF-04..RF-08) como artefato de
   verificação executável contra `module.go`.

### Dependências Técnicas

- Nenhuma infra nova. Postgres do workflow/ledger e OpenRouter já provisionados.
- `mockery` (`.mockery.yml`) para os mocks das interfaces novas.

## Monitoramento e Observabilidade

- Cada tool é um `Run` auditável (RF-27) com span por binding e por tool; status como tipo fechado.
- Métrica `agents_write_total` (já existente) ganha novo `operation` para `create_recurrence`,
  `update_recurrence`, `delete_recurrence`, `update_card`. Labels restritos a enums fechados
  (`operation`, `outcome`, `tool`, `status`); **proibido** `user_id`/`category_id` (RF-28/R-TXN-004).
- Scorer `tool-call-accuracy` (expected-tool, ADR-002) emite score por run para dashboard de M-04.

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — Reutilizar o workflow único `destructive-confirm` para as novas operações via novos
  `OperationKind` fechados + dispatch por mapa (vs. criar workflows dedicados por operação).
  `.specs/prd-mecontrola-agent-tools/adr-001-reuse-destructive-confirm-new-operationkinds.md`.
- **ADR-002** — Validação de seleção de tool por scorer de **tool esperada por cenário** + harness
  real-LLM, elevando o scorer coarse atual (match de qualquer tool financeira) para atender M-04.
  `.specs/prd-mecontrola-agent-tools/adr-002-expected-tool-scorer-and-realllm-harness.md`.
- **ADR-003** — Idempotência: `create_recurrence` via `IdempotentWrite` (chave
  `wamid|itemSeq|operation`); edições/exclusões por `version` + gate. `adr-003-idempotency-new-write-tools.md`.
- **ADR-004** — Segregação de interface: nova `RecurrenceManager` coesa em vez de inflar
  `TransactionsLedger` com os 5 métodos de recorrência. `adr-004-recurrence-manager-interface.md`.

### Riscos Conhecidos

- **R1 — Ambiguidade de seleção de tool** entre tools semelhantes (`get_entry` vs `search_transactions`,
  `list_cards` vs `count_cards`). Mitigação: descrições precisas + conjunto canônico + scorer
  expected-tool (ADR-002); barra M-04 ≥ 0.90 bloqueia aceite.
- **R2 — Superfície destrutiva ampliada** (3 novas). Mitigação: reuso do gate existente com
  semântica estrita/TTL/reprompt já testada; impact notes específicas por `TargetKind`.
- **R3 — `update_card` condicional** (gate só quando muda dia de vencimento) pode confundir o modelo.
  Mitigação: a decisão de gate é do exec (determinística, compara campo `dueDay` presente), não do
  LLM; instrução explícita.
- **R4 — Falso positivo de cobertura** se uma tool for registrada mas nunca exercida. Mitigação:
  RF-29/RF-30 + harness real-LLM cobrindo todas as tools; gate de aceite bloqueia com M-03 < 100%.
- **R5 — `RawCreateRecurringTemplate` não tem campos de origem (`wamid`)** como `RawCreateTransaction`.
  Mitigação: idempotência aplicada na camada do agente (`IdempotentWrite` + ledger próprio), não no
  DTO do módulo — o closure de escrita chama a binding sem depender de origin fields (ADR-003).

### Conformidade com Padrões

- **R-ADAPTER-001**: tools e adapters finos, zero comentários, sem SQL/branching de domínio.
- **R-AGENT-WF-001**: roteamento por registry/mapa (sem `switch case intent.Kind`), Tool fina,
  estados fechados (`OperationKind`/`ToolOutcome`/`RunStatus`), LLM só nas call-sites sancionadas,
  Run auditável, Thread-first, estado de espera persistido antes da confirmação e resume por
  merge-patch antes do parse.
- **R-WF-KERNEL-001**: kernel `internal/platform/workflow` intocado (sem domínio/LLM/SQL fora do
  adapter postgres).
- **R-DTO-VALIDATE-001**: qualquer input DTO novo em `application/dtos/input/` tem `Validate()`.
- **R-TESTING-001**: testes de use case em testify/suite whitebox com `fake.NewProvider()`.
- **R-TXN-004 / RF-28**: cardinalidade de métricas controlada.
- **Memórias**: sem abstração de tempo (`time.Now().UTC()` inline), `defer func(){ _ = rows.Close() }()`,
  validação com LLM real obrigatória, subagentes para refactor amplo.

### Arquivos Relevantes e Dependentes

- Modificados: `internal/agents/application/interfaces/{card_manager,transactions_ledger,budget_planner}.go`;
  novo `.../interfaces/recurrence_manager.go`.
- `internal/agents/infrastructure/binding/{card_manager,transactions_ledger,budget_planner}_adapter.go`;
  novo `recurrence_manager_adapter.go`.
- `internal/agents/application/tools/*` (15 arquivos novos).
- `internal/agents/application/workflows/{confirm_state,destructive_confirm_workflow}.go`.
- `internal/agents/application/scorers/mecontrola_scorers.go`; possivelmente
  `internal/platform/scorer/{scorer,types}.go` (campo expected/Args — ADR-002).
- `internal/agents/application/agents/mecontrola_agent.go` (instruções) e `internal/agents/module.go`
  (wiring `buildFinancialTools` + adapters).
- Use cases consumidos (inalterados): `internal/card/...`, `internal/transactions/...`,
  `internal/budgets/application/usecases/suggest_allocation.go`.
