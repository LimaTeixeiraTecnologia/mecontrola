<!-- spec-hash-prd: aeebc1a1f0702c58ddd0002ba503af5b4fc7a0702686a948bb7152477fed9830 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Superfície de Tools do MeControla Agent

> PRD consumido: `.specs/prd-mecontrola-agent-tools/prd.md` (spec-version 3).
> Skills obrigatórias na implementação: `go-implementation` (Etapas 1–5 + checklist R0–R7) e `mastra`.
> Regras hard aplicáveis: R-ADAPTER-001, R-AGENT-WF-001, R-WF-KERNEL-001, R-DTO-VALIDATE-001,
> R-TESTING-001, R-TXN-004.
>
> **Alinhamento à spec-version 3 (2026-07-02).** Esta techspec absorve a correção da premissa falsa
> "bucket 1 funcional": a evidência de produção (`Evidência de Produção`, EP-01..EP-05 do PRD) provou
> que o substrato de escrita/leitura está **quebrado** (sucesso alucinado com 0 linhas em
> `transactions`/`agents_write_ledger`; `query_plan` não exercida; nenhuma `role=tool` persistida).
> Consequentemente, o desenho passa a incluir, como pré-requisito **P0 bloqueante** anterior a
> qualquer tool nova: (a) injeção server-side de identidade/idempotência no ponto de invocação da
> tool (RF-37); (b) guard bloqueante de anti-simulação de escrita (RF-38); (c) Run auditável com
> evidência de escrita real — persistência de `memory.RoleTool` e `resource_id` (RF-39); a premissa
> corrigida (RF-40). Adiciona também a clarificação de registro reutilizando `ConfirmState` com um
> `OperationKind` não-destrutivo `OpConfirmRegister` (RF-41..RF-43) e a tool `list_categories`
> (RF-18e). O conjunto-alvo passa de **15 para 16 tools novas** (9 atuais → **25 no total**).

## Resumo Executivo

O `mecontrola-agent` hoje expõe 9 tools (`internal/agents/module.go:254-262`). O PRD prescreve um
conjunto-alvo de **16 tools novas** (RF-09..RF-18e), cada uma mapeada a um use case real já existente
em `internal/{budgets,card,categories,transactions}`. Nenhuma capacidade de domínio nova é criada na
camada consumidora; a evolução é **majoritariamente na camada consumidora** `internal/agents` — port
do padrão Mastra — reutilizando o substrato `internal/platform/{agent,tool,workflow,scorer}`.

**Exceção de plataforma (spec-version 3, P0 bloqueante).** Diferente da spec-version 2, esta iteração
**toca o substrato de agent** `internal/platform/agent` (não o kernel `internal/platform/workflow`)
para corrigir o defeito comprovado em produção: identidade/idempotência (`userId`/`wamid`/`itemSeq`)
não injetadas server-side, sucesso de escrita reportado sem retorno real do use case, e ausência de
persistência de mensagens de tool. Estas correções (RF-37..RF-40) são pré-requisito bloqueante:
nenhuma tool nova é considerada coberta enquanto o substrato não for corrigido e verificado por
escrita real no banco. O kernel `internal/platform/workflow` permanece intocado (R-WF-KERNEL-001).

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

Componentes **novos ou modificados** (em `internal/agents`, exceto o substrato P0 em
`internal/platform/agent`):

- **Substrato de agent (modificado, P0 — RF-37..RF-39)** `internal/platform/agent/`:
  `agent.go` (`invokeToolCall`) injeta identidade/idempotência server-side e propaga registros de
  tool-call; `ports.go` (`Result`) ganha os registros de tool-call; `runtime.go` (`Execute`,
  `buildMessages`) grava `memory.RoleTool` + `resource_id` e aplica o guard anti-simulação antes de
  marcar `RunStatusSucceeded`. Kernel `internal/platform/workflow` intocado.
- **Interfaces de consumidor (modificadas)** `internal/agents/application/interfaces/`:
  `card_manager.go`, `transactions_ledger.go`, `budget_planner.go` ganham métodos novos; nova
  interface `recurrence_manager.go` (`RecurrenceManager`); `categories_reader.go` ganha método de
  listagem `ListCategories` (RF-18e).
- **Binding adapters (modificados)** `internal/agents/infrastructure/binding/`:
  `card_manager_adapter.go`, `transactions_ledger_adapter.go`, `budget_planner_adapter.go`,
  `categories_reader_adapter.go` ganham campos/métodos; novo `recurrence_manager_adapter.go`.
- **Tools novas (16)** `internal/agents/application/tools/`: um arquivo por tool (inclui
  `list_categories`). Tools de escrita/leitura por usuário têm `wamid`/`itemSeq`/`userId` **removidos
  do schema** exposto ao modelo (RF-37).
- **Estado de confirmação (modificado)** `internal/agents/application/workflows/confirm_state.go`:
  novos `OperationKind` destrutivos (`OpUpdateRecurrence`, `OpDeleteRecurrence`, `OpUpdateCard`) e o
  `OperationKind` **não-destrutivo** `OpConfirmRegister` (RF-43), todos como tipo fechado.
- **Workflow destrutivo (modificado)** `internal/agents/application/workflows/destructive_confirm_workflow.go`:
  dispatch por mapa `map[OperationKind]executeFn`, novos executores, mensagens e impact notes.
- **Scorers (modificados)** `internal/agents/application/scorers/mecontrola_scorers.go`: lista de
  tools atualizada + novo scorer de tool esperada por cenário; `internal/platform/scorer` ganha o
  campo `Args`/expected em `RunSample`/`ToolCallRecord` se necessário ao harness (ADR-002).
- **Wiring (modificado)** `internal/agents/module.go`: injeção dos novos use cases nos adapters,
  construção das tools em `buildFinancialTools`, e construção da nova `RecurrenceManager`.
- **Harness de validação (novo)** `internal/agents/.../*_realllm_test.go`: suíte de seleção de tool
  com LLM real (gated `RUN_REAL_LLM`) medindo M-04.

Fluxo de dados: `InboundRequest → AgentRuntime.Execute → Agent.Execute (loop tool-calling) →
invokeToolCall (injeta identidade server-side + coleta ToolCallRecord) → ToolHandle.Invoke → exec →
binding → usecase.Execute → repo`. O runtime agrega os `ToolCallRecord` no `Result`, persiste as
mensagens `memory.RoleTool` e aplica o guard anti-simulação antes de fechar o Run. Tools destrutivas e
a clarificação de registro suspendem via `Engine.Start(confirmDef)` e retomam por merge-patch antes de
qualquer parse.

### Substrato de escrita/leitura confiável (P0 — RF-37..RF-40)

Pré-requisito bloqueante. Corrige o defeito comprovado em produção (EP-01/EP-02/EP-05). As três
correções abaixo vivem em `internal/platform/agent` (substrato de agent, não o kernel).

#### RF-37 — Injeção server-side de identidade/idempotência

Hoje o LLM é obrigado a fornecer `wamid`/`itemSeq`/`userId` (`register_expense.go:52` marca-os
`required` com `Strict:true`), mas `buildMessages` (`runtime.go:173-193`) nunca coloca
`in.ResourceID`/`in.MessageID` no prompt — o modelo não tem como fornecê-los corretamente, então a
escrita nasce inválida ou é alucinada. Correção:

1. **Remover** `wamid`, `itemSeq` e `userId` do `Schema` de input e da lista `required` de toda tool
   de escrita/leitura por usuário (`register_expense`, `register_income`, `register_card_purchase`,
   `query_month`, `query_plan`, `create_recurrence` e novas que precisem de identidade). O modelo
   deixa de ver esses campos.
2. **Injetar server-side** no ponto de invocação (`agent.invokeToolCall`, `agent.go:198-219`) antes de
   `h.Invoke`. O runtime propaga a identidade opaca do `InboundRequest` (`ResourceID`=userId,
   `MessageID`=wamid) e um `itemSeq` monotônico por Run através de um **valor de contexto tipado**
   (carrier), lido em `invokeToolCall` e mesclado nos `argsBytes` como campos reservados antes do
   `Invoke`. A tool continua fina (R-AGENT-WF-001.2): não decide nada; apenas recebe o input já
   completo. O `itemSeq` é um contador incrementado a cada tool de escrita dentro do mesmo Run
   (determinístico, sem `time`).

```go
type identityKey struct{}

type toolIdentity struct {
	userID  string
	wamid   string
	itemSeq int
}
```

O runtime injeta `context.WithValue(ctx, identityKey{}, &toolIdentity{userID: in.ResourceID, wamid: in.MessageID})`
antes de `Agent.Execute`; `invokeToolCall` lê o carrier, incrementa `itemSeq`, e faz merge JSON dos
três campos reservados sobre `argsBytes`. Campos ausentes no schema exposto ao LLM, presentes no
input decodificado pela tool. Nenhuma regra de domínio no substrato — só transporte de identidade.

RTA-08 e M-07: **100%** das escritas passam a derivar identidade do `InboundRequest`/Run; valor de
identidade fornecido pelo LLM é ignorado por não existir mais no schema.

#### RF-38 — Guard bloqueante de anti-simulação

Hoje `runtime.go:155-162` marca `RunStatusSucceeded`/`ToolOutcomeRouted` sempre que
`result.Content != ""` — texto de sucesso do LLM basta, mesmo com zero escrita (EP-01/EP-05). O
`anyFinancialToolScorer` (`scorers/mecontrola_scorers.go` + `scoring_hooks.go` AfterExecute) roda
assíncrono e não bloqueia `sendReply` (`whatsapp_inbound_consumer.go:163`). Correção:

1. **Propagar evidência de tool-call para o runtime.** `invokeToolCall` já constrói a `llm.Message`
   com `Role: roleTool` e conteúdo do resultado; passa a também registrar um `ToolCallRecord`
   (tipo fechado) coletado por `completeWithTools` e exposto em `Result`:

```go
type ToolCallRecord struct {
	Tool       string
	Outcome    ToolCallOutcome
	ResourceID string
	Content    string
}
```

   `ToolCallOutcome` é tipo fechado (`ToolCallOutcomeSuccess`, `ToolCallOutcomeError`), derivado do
   `toolExecStatus` já existente e do `outcome` decodificado do output da tool
   (`routed`/`reconciled`/`replay` = sucesso).

2. **Guard no fechamento do Run.** Antes de `closeRun`, o runtime avalia: se a intenção do turno é
   uma **escrita** (o modelo chamou ao menos uma tool de escrita) e **nenhuma** tool de escrita
   retornou `ToolCallOutcomeSuccess`, então o Run é `RunStatusFailed`/`ToolOutcomeUsecaseError` — é
   **proibido** marcar `RunStatusSucceeded`/`ToolOutcomeRouted` apenas por `result.Content` não-vazio.
   A classificação "tool de escrita" é um conjunto fechado conhecido pelo consumidor (registrado no
   `WriteToolSet`, ver adiante), não branching de domínio dinâmico.

3. **Bloquear resposta de sucesso simulado ao usuário.** O consumidor
   (`whatsapp_inbound_consumer.go`) só envia texto de sucesso quando `Outcome.Status` é sucesso; caso
   o guard reprove, envia mensagem honesta de indisponibilidade (RF-25), nunca "registrado com
   sucesso".

Detecção de "houve tool de escrita bem-sucedida": derivada exclusivamente dos `ToolCallRecord` do
loop — `exists r ∈ Result.ToolCalls : r.Tool ∈ WriteToolSet ∧ r.Outcome == ToolCallOutcomeSuccess`.
Sem consultar banco no caminho de resposta; a evidência vem do retorno real do use case propagado
pela tool.

#### RF-39 — Run auditável com evidência de escrita real

Hoje `runtime.go:138-153` só grava `memory.RoleUser` e `memory.RoleAssistant`; `memory.RoleTool`
(`internal/platform/memory/types.go:15`) existe mas nunca é persistido — o Run não distingue escrita
real de texto (EP-05). Correção: para cada `ToolCallRecord` do `Result`, o runtime grava uma mensagem
`memory.RoleTool` (com `Content` do resultado da tool e, quando presente, o `resource_id` retornado
pela escrita) via `messages.Append`, antes de `closeRun`. O `resource_id` da escrita fica assim
auditável no Run, satisfazendo RF-27/RF-39 (escrita referencia o identificador do audit trail).

#### RF-40 — Premissa corrigida

Estas três correções são **gate P0** anterior à validação de qualquer tool nova: o harness real-LLM
(RF-29/RF-33) só é executável como prova de cobertura após o substrato passar por escrita real
verificada no banco. Substitui, no eixo de plataforma, a premissa da spec-version 2 de que o bucket 1
já funcionava.

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

// categories_reader.go — adicionar (RF-18e); hoje só SearchDictionary/ResolveRootsBySlug
ListCategories(ctx context.Context, userID uuid.UUID) ([]Category, error)
```

`ListCategories` delega ao use case real `internal/categories/application/usecases/list_categories.go`
(`ListCategories`) via novo método em `categories_reader_adapter.go` (adapter fino: span
`agents.binding.categories.list`, mapeia retorno → `[]Category` agent-owned, wrapping de erro). O tipo
`Category` é struct plana espelhando o DTO do módulo, sem lógica.

Cada método do adapter segue o idioma verificado (`card_manager_adapter.go:42-70`): abrir span
`agents.binding.<x>.<op>`, mapear args → DTO de input do use case, `Execute`, mapear retorno → tipo
agent-owned, `fmt.Errorf("agents/binding/<x>: <ação>: %w", err)`. Zero regra de negócio no adapter.

### Tools novas (16) — mapeamento tool → capacidade real

> `UserId via` = origem da identidade. Onde consta **runtime ctx (RF-37)**, `userId`/`wamid`/`itemSeq`
> são injetados server-side pelo substrato (não expostos ao LLM); a coluna descreve a identidade, não
> um argumento do modelo.

| Tool (id LLM) | UserId via | Delega a | Idempotência/gate |
|---|---|---|---|
| `list_cards` | runtime ctx (RF-37) | `CardManager.ListCards` | leitura |
| `get_card` | runtime ctx (RF-37) | `CardManager.GetCard` | leitura |
| `count_cards` | runtime ctx (RF-37) | `CardManager.CountCards` | leitura |
| `best_purchase_day` | — (bank+dueDay) | `CardManager.BestPurchaseDay` | leitura/cálculo |
| `query_card_invoice` | runtime ctx (RF-37)+cardId | `TransactionsLedger.GetCardInvoice` | leitura |
| `search_transactions` | runtime ctx (RF-37) | `TransactionsLedger.SearchTransactions` | leitura |
| `get_transaction` | input `txId` | `TransactionsLedger.GetTransaction` | leitura |
| `get_card_purchase` | input `purchaseId` | `TransactionsLedger.GetCardPurchase` | leitura |
| `list_card_purchases` | runtime ctx (RF-37)+cardId | `TransactionsLedger.ListCardPurchases` | leitura |
| `list_recurrences` | runtime ctx (RF-37) | `RecurrenceManager.ListRecurrences` | leitura |
| `list_categories` | runtime ctx (RF-37) | `CategoriesReader.ListCategories` | leitura (RF-18e) |
| `create_recurrence` | runtime ctx (RF-37) | `RecurrenceManager.CreateRecurrence` via `IdempotentWrite` | idempotente (ADR-003) |
| `suggest_allocation` | — (totalCents+allocations) | `BudgetPlanner.SuggestAllocation` | leitura/cálculo |
| `update_recurrence` | runtime ctx | `destructive-confirm` → `OpUpdateRecurrence` | gate (ADR-001) |
| `delete_recurrence` | runtime ctx | `destructive-confirm` → `OpDeleteRecurrence` | gate (ADR-001) |
| `update_card` | runtime ctx | `destructive-confirm` → `OpUpdateCard` **se muda dia venc.**; senão direto | gate condicional (ADR-001) |

`list_categories` (RF-18e) atende pedidos como "quais categorias existem?" (EP-03) delegando ao use
case real `ListCategories`; sai do Fora de Escopo FE-08 (que permanece só para navegação de dicionário
`GetCategory`/`ListDictionary`).

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
`OpDeleteCard=3`) com os destrutivos `OpUpdateRecurrence`, `OpDeleteRecurrence`, `OpUpdateCard` e o
**não-destrutivo** `OpConfirmRegister` (RF-43), atualizando `String()`, `IsValid()` e
`ParseOperationKind()`. Todos permanecem tipo fechado — sem string solta (RTA-03; gate reemitido
abaixo). `ConfirmState` já carrega `UpdatePayload`, `TargetRef`, `TargetKind`, `Version`, `UserID` —
suficientes para as novas operações (a carga de update de recorrência/cartão e o rascunho do registro
pendente são serializados em `UpdatePayload`, como `edit_entry` já faz). Nenhum campo novo
obrigatório; `TargetKind` passa a aceitar `"recurring_template"`, `"card"` e `"register_draft"`.

O dispatch em `executeOperation` migra de `switch` para `map[OperationKind]func(ctx, ConfirmState,
deps) error` (ADR-001, aderente a R-AGENT-WF-001.1 — resolução por mapa, não switch de domínio).

### Clarificação de registro (RF-41/RF-42/RF-43)

Reintroduz o fluxo de clarificação de registro **reutilizando** o substrato existente `ConfirmState`
+ `destructive_confirm_workflow.go`, sem mecanismo HITL paralelo (RF-43). O `OperationKind`
`OpConfirmRegister` é **não-destrutivo**: o `AwaitingKind` continua `AwaitingConfirm`, mas o executor
mapeado no dispatch (`map[OperationKind]executeFn`) grava o lançamento pendente via a tool de escrita
(`register_expense`/`register_income`) em vez de deletar/editar.

- **Categoria (RF-41):** o agente só entra no gate quando a categoria está **ausente ou ambígua** —
  isto é, `classify_category` (`CategoriesReader.SearchDictionary`) não resolveu com confiança. Quando
  a categoria é resolvida com confiança, grava direto (RF-21 — pede apenas o dado faltante). A decisão
  "ambígua" é resultado do use case de classificação, não branching novo na tool.
- **Data (RF-42):** resolvida por **default determinístico** — data corrente em `America/Sao_Paulo`,
  inferindo "ontem"/data relativa/data explícita a partir do texto — **sem perguntar**. Confirmação de
  data só quando genuinamente ambígua. O cálculo usa `time.Now().UTC()` convertido para a timezone no
  ponto de uso (sem abstração de tempo), dentro do use case/binding, não na tool.
- **Contrato de pending step (RF-43, R-AGENT-WF-001.7):** o rascunho do registro é persistido no
  `Snapshot` do kernel via `Engine.Start(confirmDef)` **antes** de o agente perguntar a categoria; o
  resume aplica merge-patch (ex.: `{"ResumeText":"mercado"}`) sobre o `Snapshot.State` **antes** de
  qualquer parse; a conclusão é determinística (confirma→grava e conclui; ambíguo→reprompt único;
  negativa/TTL→cancela sem efeito e conclui), sem draft órfão. Fonte única de verdade: o snapshot do
  kernel; sem side-store de domínio.

#### Reemissão de gate — R-AGENT-WF-001.7-A (SUPERSEDED como caminho literal)

O addendum R-AGENT-WF-001.7-A está SUPERSEDED como caminho literal (citava
`daily_ledger_agent.go`/`continuePendingApproval` do `internal/agent` removido). Reemitido aqui para
os arquivos reais deste consumidor:

- Estado de espera como tipo fechado: `OperationKind` (incl. `OpConfirmRegister`) e `AwaitingKind` em
  `internal/agents/application/workflows/confirm_state.go`; nunca string solta.
- Persistência antes de perguntar e resume por merge-patch antes do parse:
  `internal/agents/application/workflows/destructive_confirm_workflow.go` (passo `confirm_gate` +
  bloco de resume), consumido no caminho de inbound do
  `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`.

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "OperationKind\s*=\s*\"[^\"]*\"\|AwaitingKind\s*=\s*\"[^\"]*\"" \
  internal/agents/application/workflows/ \
  && echo "FAIL: OperationKind/AwaitingKind como string solta" && exit 1 \
  || true
```

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

- Gate `destructive-confirm` end-to-end para as 3 novas operações destrutivas e para
  `OpConfirmRegister` (clarificação de registro, RF-43) via `testcontainers-go`
  (`//go:build integration`): Start → suspend persistido → Resume por merge-patch → efetivação →
  run concluído; `OpConfirmRegister` grava o lançamento pendente apenas após confirmação/resolução.
  Idempotência de `create_recurrence` (replay do mesmo `wamid|itemSeq|operation` não duplica).
- **Substrato P0 (RF-37..RF-39):** injeção server-side verificada por assert de que a linha em
  `agents_write_ledger` usa o `wamid`/`itemSeq` derivados do `InboundRequest` (não valor do LLM);
  persistência de `memory.RoleTool` verificada em `platform_messages`; guard anti-simulação verificado
  reproduzindo EP-01 (texto de sucesso do LLM + 0 escrita ⇒ Run `failed`).

### Testes E2E

- **Harness de seleção de tool com LLM real (mandatório, memória `feedback_realllm_validation`)**:
  `*_realllm_test.go` gated por `RUN_REAL_LLM=1` + credenciais `OPENROUTER_*` do `.env`. Conjunto
  canônico determinístico de cenários (1 tool esperada por cenário) cobrindo **todas** as 25 tools
  (9 atuais + 16 novas). Mede M-04 (acerto de tool esperada ≥ 0.90) e RF-29 (toda tool exercida ao
  menos uma vez). Mocks não contam como evidência.

- **ASSERT de escrita real no banco (RF-29/RF-33/M-05, mandatório)**: para **todo** cenário de
  escrita (`register_expense`, `register_income`, `register_card_purchase`, `create_recurrence`), o
  harness real-LLM DEVE, após o turno, verificar a **existência das linhas correspondentes** nas
  tabelas de destino — `transactions`, `transactions_card_purchases`, `agents_write_ledger`,
  `transactions_recurring_templates` — por consulta direta ao banco (testcontainers/Postgres real).
  Texto de sucesso do agente **não conta** como evidência (M-05 = 0). Assert complementar de M-07:
  a linha em `agents_write_ledger` referencia o `wamid`/`itemSeq` derivados do `InboundRequest`,
  provando identidade injetada server-side (nunca valor do LLM).

- **Cenário canônico de regressão EP-01..EP-05 (P0)**: reproduz a conversa de produção que expôs o
  defeito e trava a não-regressão:
  - **EP-01** — "compra no mercado R$150" → assert de 1 linha em `transactions` **e** 1 em
    `agents_write_ledger`; guard RF-38 impede `RunStatusSucceeded` sem escrita.
  - **EP-02** — com budget `2026-07` ativo semeado, "meu plano de julho" → `query_plan` exercida e
    retorna o plano (não "não encontrei").
  - **EP-03** — "quais categorias disponíveis?" → `list_categories` selecionada (não `query_plan`).
  - **EP-04** — registro com categoria resolvida com confiança grava sem perguntar; registro com
    categoria ausente/ambígua entra no gate `OpConfirmRegister` (persistência antes de perguntar).
  - **EP-05** — após um turno de escrita, `platform_messages` contém ao menos uma `role=tool` e o Run
    carrega o `resource_id` da escrita (RF-39).

## Sequenciamento de Desenvolvimento

### Ordem de Build

0. **P0 — Substrato de escrita/leitura confiável** (`internal/platform/agent`, RF-37..RF-40),
   **bloqueante e anterior a qualquer tool nova**: (a) `Result` + `ToolCallRecord`/`ToolCallOutcome`
   fechados em `ports.go`; (b) injeção server-side de identidade via carrier de contexto em
   `agent.invokeToolCall` e remoção de `wamid`/`itemSeq`/`userId` dos schemas das tools de
   escrita/leitura; (c) guard anti-simulação + persistência de `memory.RoleTool`/`resource_id` em
   `runtime.go`. Validado por escrita real no banco (harness P0 EP-01..EP-05) antes de prosseguir.
1. **Tipos + interfaces agent-owned** (`interfaces/`): structs planas e assinaturas novas (incl.
   `Category` e `CategoriesReader.ListCategories`, RF-18e); base para tudo. Compila isolado.
2. **Binding adapters + wiring dos use cases** (`binding/` + `module.go`): injetar os use cases
   existentes nos adapters (incl. `categories_reader_adapter.go` → `ListCategories`); nova
   `RecurrenceManager`. Valida que os módulos expõem os construtores.
3. **Tools de leitura (12)**: `list_cards`, `get_card`, `count_cards`, `best_purchase_day`,
   `query_card_invoice`, `search_transactions`, `get_transaction`, `get_card_purchase`,
   `list_card_purchases`, `list_recurrences`, `list_categories`, `suggest_allocation`. Baixo risco,
   sem gate.
4. **`create_recurrence`** com `IdempotentWrite` (ADR-003).
5. **OperationKinds novos + executores no confirm workflow** (ADR-001): 3 destrutivos
   (`update_recurrence`, `delete_recurrence`, `update_card`) e o não-destrutivo `OpConfirmRegister`
   (RF-41/42/43 — clarificação de registro), com dispatch por mapa.
6. **Registro em `buildFinancialTools` + instruções do agente** (RF-20): declarar todas as tools e
   quando usá-las (incl. quando pedir categoria e como resolver data por default).
7. **Scorers + harness de validação** (ADR-002): scorer de tool esperada, atualização da lista de
   tools, suíte real-LLM com **assert de linhas no banco** para escritas e cenário EP-01..EP-05.
   Fecha M-03/M-04/M-05/M-07/RF-29.
8. **Mapa capacidade→tool e relatório de gaps** versionados (RF-04..RF-08) como artefato de
   verificação executável contra `module.go`.

### Dependências Técnicas

- Nenhuma infra nova. Postgres do workflow/ledger e OpenRouter já provisionados.
- `mockery` (`.mockery.yml`) para os mocks das interfaces novas.

## Monitoramento e Observabilidade

- Cada tool é um `Run` auditável (RF-27) com span por binding e por tool; status como tipo fechado.
  A partir da spec-version 3, o Run persiste mensagens `memory.RoleTool` e o `resource_id` da escrita
  (RF-39), distinguindo escrita real de texto de sucesso; o guard anti-simulação (RF-38) impede
  `RunStatusSucceeded` sem retorno real de use case.
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
- **R6 — Mudança no substrato de agent (P0) afeta todas as tools de escrita/leitura.** Alterar
  `invokeToolCall`/`Result`/`runtime.go` toca o caminho de execução comum. Mitigação: mudança aditiva
  e retrocompatível (campos reservados server-side + guard só reprova sucesso simulado); cenário de
  regressão EP-01..EP-05 com assert de banco trava não-regressão antes de expor qualquer tool nova
  (RF-40).

### Conformidade com Padrões

- **R-ADAPTER-001**: tools e adapters finos, zero comentários, sem SQL/branching de domínio.
- **R-AGENT-WF-001**: roteamento por registry/mapa (sem `switch case intent.Kind`), Tool fina,
  estados fechados (`OperationKind` incl. `OpConfirmRegister`, `ToolCallOutcome`, `ToolOutcome`,
  `RunStatus`, `AwaitingKind`), LLM só nas call-sites sancionadas, Run auditável com evidência de
  escrita (`memory.RoleTool` + `resource_id`, RF-39), Thread-first, estado de espera persistido antes
  da confirmação e resume por merge-patch antes do parse (RF-43). Gate R-AGENT-WF-001.7-A reemitido
  para os arquivos reais deste consumidor (ver seção de clarificação de registro). Identidade/
  idempotência injetadas server-side em `invokeToolCall`, nunca fornecidas pelo LLM (RF-37/RTA-08).
- **R-WF-KERNEL-001**: kernel `internal/platform/workflow` intocado (sem domínio/LLM/SQL fora do
  adapter postgres).
- **R-DTO-VALIDATE-001**: qualquer input DTO novo em `application/dtos/input/` tem `Validate()`.
- **R-TESTING-001**: testes de use case em testify/suite whitebox com `fake.NewProvider()`.
- **R-TXN-004 / RF-28**: cardinalidade de métricas controlada.
- **Memórias**: sem abstração de tempo (`time.Now().UTC()` inline), `defer func(){ _ = rows.Close() }()`,
  validação com LLM real obrigatória, subagentes para refactor amplo.

### Arquivos Relevantes e Dependentes

- **P0 substrato (RF-37..RF-39):** `internal/platform/agent/{agent.go,ports.go,runtime.go}`;
  `internal/platform/memory/types.go` (uso de `RoleTool`). Kernel `internal/platform/workflow`
  intocado.
- **Tools de escrita/leitura existentes (remoção de `wamid`/`itemSeq`/`userId` do schema — RF-37):**
  `internal/agents/application/tools/{register_expense,register_income,register_card_purchase,
  query_month,query_plan}.go`.
- Modificados: `internal/agents/application/interfaces/{card_manager,transactions_ledger,budget_planner,
  categories_reader}.go`; novo `.../interfaces/recurrence_manager.go`.
- `internal/agents/infrastructure/binding/{card_manager,transactions_ledger,budget_planner,
  categories_reader}_adapter.go`; novo `recurrence_manager_adapter.go`.
- `internal/agents/application/tools/*` (16 arquivos novos, incl. `list_categories.go`).
- `internal/agents/application/workflows/{confirm_state,destructive_confirm_workflow}.go` (novos
  `OperationKind` destrutivos + `OpConfirmRegister`).
- `internal/agents/application/scorers/mecontrola_scorers.go`, `scoring_hooks.go`; possivelmente
  `internal/platform/scorer/{scorer,types}.go` (campo expected/Args — ADR-002).
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
  (só envia sucesso quando `Outcome.Status` é sucesso — RF-38).
- `internal/agents/application/agents/mecontrola_agent.go` (instruções) e `internal/agents/module.go`
  (wiring `buildFinancialTools` + adapters).
- Use cases consumidos (inalterados): `internal/card/...`, `internal/transactions/...`,
  `internal/budgets/application/usecases/suggest_allocation.go`,
  `internal/categories/application/usecases/list_categories.go`.
