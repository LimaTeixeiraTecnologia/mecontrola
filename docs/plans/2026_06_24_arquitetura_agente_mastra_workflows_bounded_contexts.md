# Plano Arquitetural — Arquitetura de Agentes/Workflows para o MeControla

## Contexto

- O documento oficial (`docs/oficial/2026_06_24_mecontrola_oficial.md`) define uma experiência conversacional única no WhatsApp: onboarding, operação diária, regras de negócio, tom de voz, guardrails e fluxos de confirmação.
- Os bounded contexts existentes são `internal/categories`, `internal/transactions`, `internal/card` e `internal/budgets`, cada um com use cases, repositórios e eventos próprios.
- O módulo `internal/agent` já implementa o padrão Mastra mapeado para Go: `AgentRuntime`, `DailyLedgerAgent`, `WorkflowRegistry`, `Tool`, `Thread`, `Run`, `WorkingMemory`, `PendingStep` e o kernel genérico `internal/platform/workflow`.
- O PRD/techspec `.specs/prd-agent-platform-evolution/` detalha a evolução incremental: HITL, plano multi-tool e recuperação contextual.

## Recomendação Principal — Opção A (Recomendada)

**Manter e consolidar a arquitetura Mastra-style single-agent com workflows/tools especializadas**, consumindo os bounded contexts via **binding adapters** sobre seus use cases. O agente único (`DailyLedgerAgent`) é o ponto de entrada conversacional; a especialização ocorre em `Workflow`/`Tool`, não em agentes separados.

### Componentes da arquitetura

1. **ParseInbound + JSON Schema** — único call-site de LLM. O schema (`prompting.ParseIntentJSONSchema`) define os `kind`s, campos, enums e constraints. A resposta do LLM é decodificada e passada por *smart constructors* em `domain/intent`, que rejeitam valores inválidos.
2. **AgentRuntime** — resolve `Thread` via `ThreadGateway.GetOrCreate(user_id, channel)`, abre/fecha `Run` auditável e emite métricas com cardinalidade controlada.
3. **IntentRouter / DailyLedgerAgent** — orquestrador fino que mantém o `WorkflowRegistry`, aplica formatação compartilhada e delega a resolução de `kind` ao registry.
4. **WorkflowRegistry** — 4 workflows: `transactions`, `budget`, `cards`, `conversational`. Cada workflow agrupa tools de um domínio e aplica o `WriteGuard` quando `kind.IsWrite()`.
5. **Tools** (`internal/agent/application/tools`) — adapters finos de responsabilidade única: recebem `ToolInput`, mapeiam `intent.Intent` para DTO/command do use case, invocam o binding, mapeiam o resultado para `ToolResult`. **Sem regra de negócio, SQL direto ou branching de domínio** (R-AGENT-WF-001.2).
6. **Bindings** (`internal/agent/infrastructure/binding`) — implementam as interfaces do agente (`tools/contracts.go`) chamando use cases dos bounded contexts. Os módulos de negócio continuam expondo use cases; o agente adapta para suas próprias interfaces.
7. **WriteGuard** — authz + replay idempotente + policy de confiança + decision audit, aplicado de forma compartilhada a todas as writes (R-AGENT-WF-001.2).
8. **Kernel genérico** (`internal/platform/workflow`) — mecanismo de orquestração durável (`Step`, `Engine`, `Store`, suspend/resume, merge-patch). Usado pelo agente para writes duráveis e HITL, mas **sem semântica de domínio** (R-WF-KERNEL-001).
9. **PendingStep / HITL** — estados tipados (`pendingexpense.Draft`, `confirmation.ConfirmState`) salvos no snapshot do kernel; retomada ocorre **antes** do `ParseInbound`.
10. **Observability** — métricas, traces e logs estruturados; labels de métrica são enums fechados, nunca `user_id`/`category_id`/`correlation_key`.

### Por que atende os objetivos

| Objetivo | Como a arquitetura entrega |
|----------|---------------------------|
| **Robustez** | Smart constructors de intent, JSON Schema, WriteGuard, HITL durável, idempotência, kernel com retry e persistência. |
| **Eficiência/economia** | LLM apenas no parse; execução determinística; reuse direto dos use cases dos bounded contexts; sem infra nova (RAG, vetor, fila dedicada). |
| **0 gap / 0 falso positivo / 0 lacuna** | Schema captura formato; smart constructors capturam invariantes; policy de confiança captura baixa certeza; pending step captura categorias ambíguas; HITL captura operações destrutivas; fallback conversacional captura o restante. Cada falha possível tem um handler tipado. |
| **Escalabilidade sem suposições** | Bounded contexts independentes, kernel durável, workers/consumers para eventos, outbox para side-effects, métricas com cardinalidade controlada. |

## Decisões Específicas

### 1. JSON Schema — sim, como contrato primário do parse

O `ParseIntentJSONSchema` já é a fonte de verdade para o formato de saída do LLM. Recomenda-se:
- Mantê-lo como **único schema de parse** e garantir que todo novo `intent.Kind` seja refletido no schema, nos smart constructors e nos testes de regressão.
- Adicionar testes que valam a ida e volta: schema → exemplo JSON → `intent.Intent` → `String()`/`Kind()`.
- No futuro, estender o schema com um campo opcional `plan` (lista ordenada de intents) para habilitar o plano multi-tool determinístico sem quebrar fluxos single-intent.

### 2. Tools para comunicação com bounded contexts — sim, via bindings

Cada bounded context já possui use cases bem definidos. O agente não deve acessar repositórios ou handlers HTTP de outros módulos diretamente. O padrão correto é:

```text
Tool (agent) → binding adapter (agent/infrastructure/binding) → use case (bounded context) → domain → repo
```

Isso já está implementado para transações, cartões, orçamentos e categorias. Deve ser mantido e documentado como contrato.

### 3. Especializar agentes? Não no MVP

A experiência do MeControla é de **um único parceiro financeiro**. Especializar em múltiplos agentes (`TransactionsAgent`, `BudgetsAgent`, `CardsAgent`) fragmentaria a thread, a memória e a orquestração, além de violar `R-AGENT-WF-001.6` (Thread/Run/WorkingMemory exclusivos do `internal/agent`).

A especialização correta é no nível de **workflow + tool**: cada domínio tem seu workflow, suas tools e seus bindings, mas todos rodam sob o mesmo `AgentRuntime` e `DailyLedgerAgent`.

### 4. Kernel genérico — usar para writes duráveis e HITL

O `internal/platform/workflow` deve permanecer genérico. O agente instancia `Engine[steps.ExpenseState]` para writes de transações e `Engine[confirmation.ConfirmState]` para HITL. A semântica (Thread, Run auditável, PendingStep, WorkingMemory) permanece no agente.

### 5. Plano multi-tool — preparar sem loop LLM

Conforme PRD, o `ParseInbound` pode evoluir para retornar um **plano ordenado de 1..N intents**. A execução é determinística, reutilizando workflows/tools existentes, com short-circuit em falha dura e agregação de respostas. Não há loop de raciocínio LLM.

## Opção B — Agentes Especializados por Bounded Context (Não Recomendada)

Criar agentes separados (`TransactionsAgent`, `BudgetsAgent`, `CardsAgent`, etc.) orquestrados por um meta-router.

- **Prós**: isolamento maior entre domínios; equipes podem evoluir agents separadamente em escala futura.
- **Contras**: quebra a experiência de um único parceiro; exige sincronização de Thread/Run/WorkingMemory (proibido fora de `internal/agent`); aumenta complexidade de contexto, memória e routing; duplica infraestrutura; não compensa para o MVP.

## Próximos Passos Sugeridos

1. Consolidar os gates de verificação `R-AGENT-WF-001` e `R-WF-KERNEL-001` no CI.
2. Completar o HITL para as 4 operações destrutivas/sensíveis conforme techspec (`delete_last_transaction`, `edit_last_transaction`, `delete_card`, `configure_budget` commit).
3. Adicionar testes de regressão de parse: schema ↔ JSON de exemplo ↔ `intent.Intent`.
4. Avaliar a adição do campo `plan` no JSON Schema para habilitar plano multi-tool no futuro.
5. Documentar o contrato de bindings entre `internal/agent` e os bounded contexts.
6. Implementar recuperação contextual (histórico do usuário + taxonomia de categorias) via queries estruturadas no Postgres, sem RAG vetorial.

## Exemplo Completo — Adicionar uma Nova Tool de Leitura (`query_income_summary`)

Este exemplo ilustra, passo a passo, como uma nova capacidade de **consulta** (read-only) entra no padrão Mastra aprovado (Opção A). **Trata-se de especificação no plano; não é código a ser compilado agora.**

Cenário: o usuário pergunta `"Quanto recebi esse mês?"`. O agente deve consultar as transações de entrada (`direction=income`) do mês atual e retornar um resumo.

### Passo 1 — Novo `intent.Kind` com smart constructor

Arquivo: `internal/agent/domain/intent/intent.go`

```go
const (
    // ... kinds existentes
    KindQueryIncomeSummary Kind = iota + 1 // ou próximo valor disponível
)

func (k Kind) String() string {
    switch k {
    // ... cases existentes
    case KindQueryIncomeSummary:
        return "query_income_summary"
    }
}

func ParseKind(raw string) (Kind, error) {
    switch strings.ToLower(strings.TrimSpace(raw)) {
    // ... cases existentes
    case "query_income_summary":
        return KindQueryIncomeSummary, nil
    }
}

func (k Kind) IsWrite() bool {
    switch k {
    case KindRecordExpense,
        KindRecordIncome,
        KindRecordCardPurchase,
        // ... outras writes
        :
        return true
    }
    return false
}

func NewQueryIncomeSummary() Intent {
    return Intent{kind: KindQueryIncomeSummary}
}
```

`KindQueryIncomeSummary` **não é write**, portanto não passa pela `WriteGuard`.

### Passo 2 — `ParseInbound` reconhece o novo kind

Arquivo: `internal/agent/application/usecases/parse_inbound.go`

```go
func build(kind intent.Kind, dto rawIntentDTO, fallbackText string) (intent.Intent, error) {
    switch kind {
    // ... cases existentes
    case intent.KindQueryIncomeSummary:
        return intent.NewQueryIncomeSummary(), nil
    }
}
```

### Passo 3 — JSON Schema atualizado

Arquivo: `internal/agent/application/prompting/prompts.go`

```go
func ParseIntentJSONSchema() map[string]any {
    return map[string]any{
        "type": "object",
        "properties": map[string]any{
            "kind": map[string]any{
                "type": "string",
                "enum": []string{
                    // ... kinds existentes
                    "query_income_summary",
                },
            },
            // ... campos existentes
        },
    }
}
```

### Passo 4 — Interface do agente para a capacidade

Arquivo: `internal/agent/application/tools/contracts.go`

```go
type IncomeSummaryReader interface {
    Execute(ctx context.Context, in IncomeSummaryInput) (IncomeSummaryResult, error)
}

type IncomeSummaryInput struct {
    UserID   string
    RefMonth string
}

type IncomeSummaryResult struct {
    RefMonth     string
    TotalCents   int64
    Count        int
    Sources      []IncomeSourceView
}

type IncomeSourceView struct {
    Description string
    AmountCents int64
}
```

### Passo 5 — Tool fina

Arquivo: `internal/agent/application/tools/income_tools.go`

```go
type QueryIncomeSummary struct {
    recorder *Recorder
    reader   IncomeSummaryReader
    loc      *time.Location
    o11y     observability.Observability
}

func NewQueryIncomeSummary(recorder *Recorder, reader IncomeSummaryReader, loc *time.Location, o11y observability.Observability) *QueryIncomeSummary {
    return &QueryIncomeSummary{recorder: recorder, reader: reader, loc: loc, o11y: o11y}
}

func (t *QueryIncomeSummary) Name() string { return "query_income_summary" }

func (t *QueryIncomeSummary) Descriptor() ToolSpec {
    return ToolSpec{
        Name:        "query_income_summary",
        IntentKind:  intent.KindQueryIncomeSummary,
        Description: "mostrar um resumo das entradas (receitas) do mês",
    }
}

func (t *QueryIncomeSummary) Execute(ctx context.Context, in ToolInput) (ToolResult, error) {
    kind := intent.KindQueryIncomeSummary
    if t.reader == nil {
        t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeMissingResolver)
        return ToolResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: kind}, nil
    }

    refMonth := currentCompetence(t.loc)
    summary, err := WithReadRetry(ctx, func(ctx context.Context) (IncomeSummaryResult, error) {
        return t.reader.Execute(ctx, IncomeSummaryInput{
            UserID:   in.UserID.String(),
            RefMonth: refMonth,
        })
    })
    if err != nil {
        t.o11y.Logger().Warn(ctx, "agent.intent_router.query_income_summary_failed",
            observability.String("ref_month", refMonth),
            observability.Error(err),
        )
        t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
        return ToolResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: kind}, nil
    }

    t.recorder.Record(ctx, kind.String(), in.Channel, OutcomeRouted)
    return ToolResult{
        Reply:   formatIncomeSummary(summary),
        Outcome: OutcomeRouted,
        Kind:    kind,
    }, nil
}
```

**A tool não contém regra de negócio, SQL ou branching de domínio.** Ela apenas adapta `intent.Intent` para o input do use case e formata a resposta.

### Passo 6 — Binding adapter

Arquivo: `internal/agent/infrastructure/binding/income_summary.go`

```go
type incomeSummaryReaderAdapter struct {
    lister *transactionsusecases.ListTransactions
    o11y   observability.Observability
}

func NewIncomeSummaryReaderAdapter(
    lister *transactionsusecases.ListTransactions,
    o11y observability.Observability,
) tools.IncomeSummaryReader {
    return &incomeSummaryReaderAdapter{lister: lister, o11y: o11y}
}

func (a *incomeSummaryReaderAdapter) Execute(ctx context.Context, in tools.IncomeSummaryInput) (tools.IncomeSummaryResult, error) {
    list, err := a.lister.Execute(ctx, transactionsinput.ListTransactions{
        UserID:   in.UserID,
        RefMonth: in.RefMonth,
    })
    if err != nil {
        return tools.IncomeSummaryResult{}, err
    }

    var total int64
    var sources []tools.IncomeSourceView
    for _, tx := range list.Transactions {
        if tx.Direction != "income" {
            continue
        }
        total += tx.AmountCents
        sources = append(sources, tools.IncomeSourceView{
            Description: tx.Description,
            AmountCents: tx.AmountCents,
        })
    }

    return tools.IncomeSummaryResult{
        RefMonth:   in.RefMonth,
        TotalCents: total,
        Count:      len(sources),
        Sources:    sources,
    }, nil
}
```

O binding reusa o use case `ListTransactions` do bounded context `transactions` e aplica o filtro de `direction=income` no adapter. Se o domínio exigir agregação específica complexa, o use case novo vive em `internal/transactions`, não no agente.

### Passo 7 — Registrar a tool no workflow

Arquivo: `internal/agent/application/services/agent_workflows.go`

```go
func (a *DailyLedgerAgent) buildRegistry() (*agentwf.IntentRegistry, error) {
    transactionsWorkflow, err := agentwf.NewIntentWorkflow("transactions", guard,
        // ... tools existentes
        agentwf.KindTool{Kind: intent.KindQueryIncomeSummary, Tool: tools.NewQueryIncomeSummary(a.recorder, a.incomeSummaryReader, a.loc, a.o11y)},
    )
    // ...
}

func routableKinds() []intent.Kind {
    return []intent.Kind{
        // ... kinds existentes
        intent.KindQueryIncomeSummary,
    }
}
```

Como `KindQueryIncomeSummary.IsWrite() == false`, a `WriteGuard` **não** é aplicada; a tool executa diretamente.

### Passo 8 — Injetar dependência no módulo do agente

Arquivo: `internal/agent/module.go`

```go
type IntentRouterDeps struct {
    // ... deps existentes
    IncomeSummaryReader tools.IncomeSummaryReader
}

func (b *agentModuleBuilder) attachIncomeSummaryReader(deps *appservices.IntentRouterDeps) {
    if b.transactionsModule.ListTransactionsUC == nil {
        return
    }
    deps.IncomeSummaryReader = agentbinding.NewIncomeSummaryReaderAdapter(
        b.transactionsModule.ListTransactionsUC,
        b.o11y,
    )
}
```

E chamar `b.attachIncomeSummaryReader(&deps)` antes de `appservices.NewIntentRouter`.

### Passo 9 — Testes de regressão

- Teste unitário para `intent.NewQueryIncomeSummary` validando que o kind é o esperado.
- Teste de `ParseInbound` com JSON contendo `"kind": "query_income_summary"`.
- Teste da tool `QueryIncomeSummary` com mock de `IncomeSummaryReader` cobrindo sucesso, erro de use case e resolver ausente.
- Teste de integração que envia `"Quanto recebi esse mês?"` e verifica a resposta formatada.

### Diagrama do fluxo

```text
Usuário: "Quanto recebi esse mês?"
    │
    ▼
ParseInbound (LLM + JSON Schema) ──► intent.KindQueryIncomeSummary
    │
    ▼
AgentRuntime (Thread → Run)
    │
    ▼
WorkflowRegistry.Resolve(KindQueryIncomeSummary)
    │
    ▼
Workflow "transactions" (sem WriteGuard, pois é leitura)
    │
    ▼
Tool QueryIncomeSummary.Execute
    │
    ▼
Binding NewIncomeSummaryReaderAdapter
    │
    ▼
Use case ListTransactions (internal/transactions)
    │
    ▼
Domain → Repository → Postgres
    │
    ▼
Resposta formatada para o usuário
```

### Regras respeitadas no exemplo

- `R-AGENT-WF-001.1`: nenhum `case intent.Kind` novo em `daily_ledger_agent.go`; roteamento via registry.
- `R-AGENT-WF-001.2`: tool e binding são adapters finos; regra de negócio fica no use case do bounded context.
- `R-AGENT-WF-001.3`: `ToolOutcome`, `RunStatus` e `intent.Kind` continuam tipados.
- `R-AGENT-WF-001.4`: LLM apenas no parse.
- `R-AGENT-WF-001.5`: execução observável como Run com workflow/tool/outcome.
- `R-AGENT-WF-001.6`: Thread/Run resolvidos pelo `AgentRuntime`.
- `R-WF-KERNEL-001`: kernel permanece genérico; leituras não precisam de suspensão, mas se um dia precisarem, usam `Engine[IncomeSummaryState]` sem expor domínio no kernel.

## Refatorações e Remoções Necessárias no Código Atual

A arquitetura aprovada (Opção A) já é majoritariamente aderente. No entanto, para chegar em **0 gap / 0 lacuna** e evitar duplicidade, alguns pontos do código atual devem ser refatorados ou removidos.

### 1. `DefaultRegistry` e `RenderSystemPrompt` em `internal/agent/application/tools/registry.go`

**Problema:**
- `RenderSystemPrompt()` existe, mas **não é usado em produção** (apenas em testes). É código potencialmente morto.
- `DefaultRegistry()` lista apenas **9 tools**, enquanto `buildRegistry()` em `agent_workflows.go` registra **21 intents**. Como `warnMissingToolBindings` usa `DefaultRegistry()` para avisar sobre bindings ausentes, tools novas podem passar despercebidas.

**Recomendação:**
- Se `RenderSystemPrompt` não tiver uso planejado, **remover**.
- Atualizar `DefaultRegistry()` para refletir **todas** as tools realmente registradas no agente, ou substituir seu uso por introspecção da registry real.

### 2. Caminho legacy de pending expense confirmation

**Problema:**
- `daily_ledger_agent.go` mantém `continuePendingExpenseConfirmationLegacy` que usa `PendingExpenseConfirmationGateway` (side-store em `sessionRepo`) como fallback.
- O PRD/techspec define que o **snapshot do kernel** deve ser a fonte única de verdade para estados suspensos (ADR-001).

**Recomendação:**
- Quando `WorkflowKernelConfig.TransactionsWriteEnabled=true`, o estado de clarificação de categoria deve ser salvo no snapshot do kernel e a retomada deve vir do kernel.
- O side-store legado pode ser **removido** para o caminho kernel, mantido apenas como fallback quando o kernel estiver desabilitado.

### 3. Duplicação da WriteGuard

**Problema:**
- A lógica de pré-escrita (authz/replay/policy/audit) existe em dois lugares:
  - `newWriteGuard()` aplicado pelo workflow legacy (`composite.go`).
  - `buildKernelDefinition()` com steps reutilizáveis (`steps/authorize.go`, `steps/replay.go`, etc.) para o kernel.

**Recomendação:**
- Quando o kernel for o caminho padrão, a guarda do workflow legacy deve ser **removida** ou reimplementada como wrapper que chama os mesmos steps do kernel.
- Manter uma única implementação da guarda reduz risco de divergência.

### 4. Fallback legacy do HITL destrutivo

**Problema:**
- `dispatchWriteDestructive` e `wireBudgetCommitGate` já usam o kernel para HITL, mas podem conter fallbacks legacy.

**Recomendação:**
- Validar que os 4 fluxos destrutivos (`delete_last_transaction`, `edit_last_transaction`, `delete_card`, `configure_budget` commit) funcionam exclusivamente sobre o kernel durável.
- Remover qualquer fallback em memória ou side-store legado após os testes de durabilidade passarem.

### 5. `AgentModuleOption` em `internal/agent/module.go`

**Problema:**
- `NewAgentModule` recebe `opts ...AgentModuleOption`. As regras de módulo `internal/identity` e `internal/billing` proíbem o padrão `NewModule(opts...)` em favor de construtores diretos.

**Recomendação:**
- Avaliar se as options (`WithSessionStore`, `WithOutboxPublisher`, `WithOnboardingLLM`) são realmente opciais no bootstrap. Se forem obrigatórias para o MVP, transformá-las em parâmetros explícitos do construtor e **remover `AgentModuleOption`**.
- Se forem realmente opcionais (ex.: agente sem session store roda em modo degradado), manter, mas documentar a decisão.

### 6. Testes de regressão do schema

**Problema:**
- Não há testes que garantam que todo `intent.Kind` possível esteja refletido no `ParseIntentJSONSchema` e nos smart constructors.

**Recomendação:**
- Adicionar teste mecânico que itere sobre todos os `intent.Kind` e valide:
  - Presença no schema.
  - Smart constructor correspondente.
  - `String()` ↔ `ParseKind()` idempotente.

### Resumo das ações

| Item | Ação | Prioridade |
|------|------|------------|
| `RenderSystemPrompt` não usado | Remover | Baixa |
| `DefaultRegistry` desatualizado | Atualizar ou substituir por registry real | Média |
| Pending step legacy | Consolidar no kernel; remover side-store | Alta |
| WriteGuard duplicada | Consolidar nos steps do kernel | Média |
| HITL legacy fallback | Remover após testes de durabilidade | Alta |
| `AgentModuleOption` | Avaliar e possivelmente remover | Baixa/Média |
| Testes de regressão do schema | Adicionar | Alta |

## Onboarding Completo — Fluxo, Use Cases e Tabelas Persistidas

O onboarding do MeControla é uma **jornada conversacional** (`docs/oficial/2026_06_24_mecontrola_oficial.md`, Capítulo 08) implementada pelo `OnboardingAgent` em `internal/agent`, que delega para `RunOnboardingTurn` e, por sua vez, para os use cases do bounded context `internal/onboarding`. A experiência conversacional é conduzida por LLM, com scripts de fallback. Sob a arquitetura aprovada (Opção A), o agente único (`DailyLedgerAgent`) é o ponto de entrada; o onboarding é uma especialização interna, não um agente separado.

### Visão geral do fluxo

```text
Usuario inicia contato (token/identidade)
        │
        ▼
┌─────────────────────────────────────────────┐
│ AgentRuntime abre Thread + Run              │
└─────────────────────────────────────────────┘
        │
        ▼
OnboardingAgent/RunOnboardingTurn
        │
        ├──► LLM interpreta a mensagem e pode chamar tools
        │
        ├──► Tools dispatcham para use cases de internal/onboarding
        │
        ├──► OnboardingSession persistida em mecontrola.onboarding_sessions
        │
        ├──► Draft v2 (sugestoes, passo) em mecontrola.agent_sessions.pending_action
        │
        └──► Eventos de dominio publicados via mecontrola.outbox_events
                │
                ▼
        Bounded contexts (budgets, card) consomem eventos e escrevem
        em suas proprias tabelas
```

**Regras de fronteira:**
- `internal/onboarding` escreve apenas em `mecontrola.onboarding_sessions` e publica eventos em `mecontrola.outbox_events`.
- `internal/agent` escreve em `mecontrola.agent_sessions` (`pending_action`, `recent_turns`), `mecontrola.agent_threads` e `mecontrola.agent_runs`.
- `mecontrola.cards`, `mecontrola.budgets`, `mecontrola.budgets_allocations` e `mecontrola.transactions` são de responsabilidade exclusiva dos respectivos bounded contexts.

### Fases do onboarding (código real)

As fases são strings definidas em `internal/agent/application/usecases/onboarding_scripts.go`:

| Fase | Valor | Significado |
|------|-------|-------------|
| Boas-vindas | `""` (vazia) | Emite mensagem de boas-vindas, marca `welcome_sent_at` e avança para `objective`. |
| Objetivo | `objective` | Captura o objetivo financeiro via `save_onboarding_objective`. |
| Orçamento | `budget` | Captura a renda/orçamento via `save_onboarding_income`; sugere splits automáticos. |
| Cartões | `cards` | Cadastra cartões via `save_onboarding_card`; aceita múltiplos cartões por mensagem. |
| Plano Financeiro | `financial_plan` | Apresenta resumo + distribuição; chama `save_onboarding_budget_splits` de uma vez com as 5 categorias. |
| Primeiro Lançamento | `first_tx` | Captura a primeira movimentação via `record_transaction`; marca e completa o onboarding. |

### Etapas da jornada

#### FASE 1 — Boas-vindas (`""`)

**Objetivo:** criar conexão, apresentar o produto, as 5 categorias e iniciar o compromisso.

**Ação do sistema:**
- Identifica ou cria o usuário (`internal/identity`).
- Cria a sessão de onboarding (`mecontrola.onboarding_sessions`) com `state='in_progress'`, `phase=''` e `welcome_sent_at`.
- Cria a `Thread` e o primeiro `Run`.
- Gera a mensagem de boas-vindas por LLM (com `scriptWelcome` como fallback); a mensagem já apresenta as 5 categorias.
- Seta `phase='objective'`.

**Use cases:**
- Identificação/criação do usuário (`internal/identity`).
- `ThreadGateway.GetOrCreate` (`internal/agent`).
- `MarkWelcomeSent` (`internal/onboarding`).
- `SetOnboardingPhase` (`internal/onboarding`).
- `AppendOnboardingTurn` (`internal/onboarding`) — histórico de turnos.

**Tabelas persistidas pelo onboarding:**
- `mecontrola.onboarding_tokens` — token de ativação consumido (se fluxo de venda).
- `mecontrola.onboarding_sessions` — `state='in_progress'`, `phase=''`, `welcome_sent_at`.

**Tabelas persistidas por outros bounded contexts:**
- `mecontrola.users` — criada/identificada por `internal/identity`.
- `mecontrola.identity_entitlements` — vinculada por `internal/identity`.
- `mecontrola.agent_threads` + `mecontrola.agent_runs` — criadas por `internal/agent`.

**Resposta (fallback real do código):**
```
👋 Oi! Eu sou o *MeControla*, seu parceiro financeiro.

📊 Aqui no MeControla todo dinheiro é organizado em apenas *5 categorias*:

💰 *Custo Fixo*
Contas que acontecem todos os meses.

🎓 *Conhecimento*
Cursos, livros e tudo que faz você evoluir.

🎉 *Prazeres*
Lazer, diversão e experiências.

🎯 *Metas*
Objetivos específicos que você deseja realizar.

🏦 *Liberdade Financeira*
Reserva financeira e investimentos.

Pronto 😊

Agora vamos montar seu plano.

🔵 *Etapa 1/4 — Objetivo*
Qual é o seu objetivo principal? (ex: quitar dívidas, fazer uma viagem, criar uma reserva)
```

#### FASE 2 — Objetivo (`objective`)

**Objetivo:** entender o motivo pelo qual o usuário deseja organizar as finanças.

**Exemplo:** `Quero quitar minhas dívidas.`

**Use cases:**
- `SaveOnboardingObjective`
- `SetOnboardingPhase(phase='budget')`
- `AppendOnboardingTurn`

**Tabelas persistidas pelo onboarding:**
- `mecontrola.onboarding_sessions` — `payload.objective`, `payload.objective_profile`, `phase='budget'`.
- `mecontrola.agent_sessions.pending_action` — draft v2 com `objective_profile` e step.

#### FASE 3 — Orçamento (`budget`)

**Objetivo:** capturar o valor disponível para planejamento mensal e sugerir uma distribuição inicial.

**Exemplo:** `4000`

**Ação do sistema:**
- Salva a renda.
- Publica `IncomeRegistered`.
- Chama `SuggestBudgetSplit` para montar splits automáticos baseados no objetivo/orçamento.
- Persiste o draft v2 com os splits sugeridos.
- Avança para `cards`.

**Use cases:**
- `SaveOnboardingIncome`
- `SuggestBudgetSplit`
- `SetOnboardingPhase(phase='cards')`
- `AppendOnboardingTurn`

**Tabelas persistidas pelo onboarding:**
- `mecontrola.onboarding_sessions` — `payload.income_cents`, `phase='cards'`.
- `mecontrola.outbox_events` — evento `IncomeRegistered`.
- `mecontrola.agent_sessions.pending_action` — draft v2 com `income`, `auto_splits`, `step=cards`.

**Tabelas atualizadas pelo bounded context `budgets`:**
- `mecontrola.budgets` / `mecontrola.budgets_allocations` — ao consumir `IncomeRegistered`.

**Resposta (fallback real):**
```
🔵 *Etapa 2/4 — Orçamento*

Pra montar seu plano, qual é o seu *orçamento mensal*? (quanto você tem por mês)
```

#### FASE 4 — Cartões (`cards`)

**Objetivo:** registrar os cartões de crédito usados pelo usuário.

**Regra oficial:** solicitar apenas **apelido** e **dia de fechamento**. Nunca solicitar limite, banco, bandeira ou dados sensíveis.

**Exemplo:** `Nubank 13`

**Ação do sistema:**
- Aceita um ou mais cartões por mensagem.
- Para cada cartão, chama `save_onboarding_card`.
- Publica `CardRegistered` por cartão.
- Se o usuário negar (`não`, `não uso`, etc.), avança direto para `financial_plan`.

**Use cases:**
- `SaveOnboardingCard`
- `SetOnboardingPhase(phase='financial_plan')`
- `AppendOnboardingTurn`

**Tabelas persistidas pelo onboarding:**
- `mecontrola.onboarding_sessions` — `payload.cards[]` (apelido, dia de fechamento).
- `mecontrola.outbox_events` — evento(s) `CardRegistered`.
- `mecontrola.agent_sessions.pending_action` — draft v2 atualizado.

**Tabelas atualizadas pelo bounded context `card`:**
- `mecontrola.cards` — ao consumir `CardRegistered`.

> **Nota de fronteira:** se hoje o use case `SaveOnboardingCard` recebe um `SynchronousCardCreator`, essa interface é declarada pelo `internal/onboarding` e implementada por um adapter fino no `internal/card`. O módulo onboarding **nunca** executa SQL ou regras de negócio do domínio `card`.

**Resposta (fallback real):**
```
🔵 *Etapa 3/4 — Cartões*

💳 Você usa *cartão de crédito*?

Se usar, me responda assim:

Nubank 13 / Inter 5 / Itaú 10

(apelido + dia de fechamento)

Se não usar, basta responder: *Não uso*
```

#### FASE 5 — Plano Financeiro (`financial_plan`)

**Objetivo:** apresentar o resumo consolidado (objetivo, orçamento, cartões, distribuição) e capturar a confirmação/ajuste dos splits.

**Ação do sistema:**
- Monta a mensagem de resumo com `buildFinancialPlanMessage(snapshot, splits)`.
- Se o usuário responder "sim" (sem dígitos), chama `save_onboarding_budget_splits` com os splits sugeridos automaticamente.
- Se o usuário informar novos valores, o LLM mapeia as 5 categorias para `root_slug` e chama `save_onboarding_budget_splits` com todas as alocações de uma vez.
- Publica `SplitsCalculated`.
- Limpa o draft v2 e avança para `first_tx`.

**Use cases:**
- `SaveOnboardingBudgetSplits`
- `SetOnboardingPhase(phase='first_tx')`
- `AppendOnboardingTurn`

**Tabelas persistidas pelo onboarding:**
- `mecontrola.onboarding_sessions` — `payload.custom_split[]`, `phase='first_tx'`.
- `mecontrola.outbox_events` — evento `SplitsCalculated`.
- `mecontrola.agent_sessions.pending_action` — limpo (`{}`).

**Tabelas atualizadas pelo bounded context `budgets`:**
- `mecontrola.budgets` / `mecontrola.budgets_allocations` — ao consumir `SplitsCalculated`.

**Resposta (template real):**
```
🔵 *Etapa 4/4 — Plano Financeiro*

🎯 *Objetivo*: Quitar dívidas
💰 *Orçamento*: R$ 4.000/mês
💳 *Cartões*: Nubank (fecha dia 13)

📋 *Distribuição Final*:

💰 *Custo Fixo*: R$ 2.000
🎓 *Conhecimento*: R$ 300
🎉 *Prazeres*: R$ 500
🎯 *Metas*: R$ 700
🏦 *Liberdade Financeira*: R$ 500

Responde *sim* pra confirmar, ou me diz como quer distribuir.
```

#### FASE 6 — Primeiro Lançamento (`first_tx`)

**Objetivo:** capturar a primeira movimentação e concluir o onboarding.

**Regra técnica:** `CompleteOnboardingSession` só completa a sessão se `FirstTxRecorded` estiver `true`.

**Ação do sistema:**
- O LLM extrai direção, valor e estabelecimento e chama `record_transaction`.
- A tool `record_transaction` é delegada ao `ExpenseRecorder` do bounded context `transactions`.
- Em seguida, chama `MarkFirstTransactionRecorded`.
- Por fim, chama `CompleteOnboardingSession`, que publica `OnboardingCompleted`.

**Use cases:**
- `RecordTransaction` / `RecordExpense` (`internal/transactions`, via `tools.ExpenseRecorder`).
- `MarkFirstTransactionRecorded` (`internal/onboarding`).
- `CompleteOnboardingSession` (`internal/onboarding`).

**Tabelas persistidas pelo onboarding:**
- `mecontrola.onboarding_sessions` — `payload.first_tx_recorded=true`, `payload.completed_at`, `state='active'`, `recent_turns` limpo.
- `mecontrola.outbox_events` — evento `OnboardingCompleted`.

**Tabelas atualizadas pelo bounded context `transactions`:**
- `mecontrola.transactions` — primeira movimentação registrada.

### Tabelas envolvidas no onboarding (resumo)

| Tabela | Responsável pela escrita | Quando é escrita |
|--------|--------------------------|------------------|
| `mecontrola.users` | `internal/identity` | Criação/identificação do usuário. |
| `mecontrola.identity_entitlements` | `internal/identity` | Vinculação do plano/assinatura. |
| `mecontrola.onboarding_tokens` | `internal/onboarding` | Consumo do token de ativação. |
| `mecontrola.onboarding_sessions` | `internal/onboarding` | Toda etapa da jornada. |
| `mecontrola.outbox_events` | `internal/platform/outbox` | Publicação dos eventos de domínio. |
| `mecontrola.agent_threads` | `internal/agent` | Criação da thread do usuário. |
| `mecontrola.agent_runs` | `internal/agent` | Cada execução de run. |
| `mecontrola.agent_sessions` | `internal/agent` | `pending_action` (draft v2 de splits/sugestões) e `recent_turns`. |
| `mecontrola.cards` | `internal/card` | Consumo do evento `CardRegistered`. |
| `mecontrola.budgets` | `internal/budgets` | Consumo dos eventos `IncomeRegistered` / `SplitsCalculated`. |
| `mecontrola.budgets_allocations` | `internal/budgets` | Consumo do evento `SplitsCalculated`. |
| `mecontrola.transactions` | `internal/transactions` | Registro da primeira movimentação pelo próprio domínio. |
| `mecontrola.categories` | `internal/categories` | Consultada, não alterada pelo onboarding. |
| `mecontrola.category_dictionary` | `internal/categories` | Consultada, não alterada pelo onboarding. |

> **Observação:** o módulo `internal/onboarding` publica eventos no `mecontrola.outbox_events`, mas **não** escreve diretamente em `mecontrola.cards`, `mecontrola.budgets`, `mecontrola.budgets_allocations` ou `mecontrola.transactions`. Cada bounded context consome seus eventos e aplica as regras/repositórios próprios.

### Payload de `mecontrola.onboarding_sessions`

O estado da jornada é armazenado em JSONB (`payload`) com a seguinte estrutura (definida em `internal/onboarding/domain/entities/onboarding_session.go`):

```go
type OnboardingSessionPayload struct {
    IncomeCents      int64
    Cards            []OnboardingCardDraft
    PendingCard      OnboardingCardDraft
    HasPending       bool
    Split            []OnboardingCardSplitEntry
    Objective        string
    CustomSplit      []OnboardingBudgetAllocationEntry
    FirstTxRecorded  bool
    Phase            string
    RecentTurns      []OnboardingTurn
    WelcomeSentAt    *time.Time
    CompletedAt      *time.Time
    ObjectiveProfile string
}
```

### Draft v2 em `mecontrola.agent_sessions.pending_action`

O `RunOnboardingTurn` mantém um draft temporário (`internal/agent/domain/onboardingv2draft.Draft`) para sugestões automáticas de splits e passo atual. Ele é serializado e salvo em `mecontrola.agent_sessions.pending_action` pelo `OnboardingSessionGateway` (`internal/agent/infrastructure/binding/onboarding_session_gateway.go`). Ao final da fase `financial_plan`, o draft é limpo (`{}`).

### Regras arquiteturais aplicadas ao onboarding

- **Cada bounded context é dono das suas tabelas:** `internal/onboarding` escreve apenas em `mecontrola.onboarding_sessions` e publica eventos via `mecontrola.outbox_events`. Tabelas de `card`, `budgets` e `transactions` são atualizadas exclusivamente pelos seus próprios bounded contexts, consumindo eventos de domínio.
- **Sem acesso cruzado a repositórios:** o agente e o onboarding não acessam repositórios, SQL ou regras de negócio de outros domínios. A integração ocorre via bindings sobre use cases ou via eventos de domínio no outbox.
- **Tool fina:** as tools de onboarding no `internal/agent` apenas adaptam a mensagem para o input do use case e formatam a resposta; toda regra de negócio vive em `internal/onboarding` (`R-AGENT-WF-001.2`).
- **LLM no onboarding:** a interpretação conversacional e a escolha de tools são feitas por LLM (`RunOnboardingTurn.interpreter`). Os scripts fixos são fallbacks, não o comportamento principal.
- **State-as-type:** `OnboardingChannel`, `FinancialObjective`, `MonthlyIncome` e `BudgetAllocation` usam smart constructors (`R5.8`, DMMF).
- **Eventos de domínio:** `IncomeRegistered`, `CardRegistered`, `SplitsCalculated`, `OnboardingCompleted` são publicados via outbox para os bounded contexts reagirem.
- **0 gap / 0 falso positivo:** cada fase tem uma tool/use case correspondente; ausência de informação é resolvida com perguntas sequenciais e reconfirmação no plano financeiro.

## Exemplo de Conversa Real — Onboarding Passo a Passo

Cenário: usuário `+55 11 91234-5678` inicia contato pelo WhatsApp após comprar um token de ativação. Abaixo, cada mensagem da conversa é mapeada para os use cases executados e as tabelas afetadas, de acordo com o código real.

> **Legenda:** `(O)` = escrita pelo `internal/onboarding`, `(I)` = escrita pelo `internal/identity`, `(A)` = escrita pelo `internal/agent`, `(P)` = escrita pelo `internal/platform/outbox`, `(C)` = escrita pelo `internal/card`, `(B)` = escrita pelo `internal/budgets`, `(T)` = escrita pelo `internal/transactions`.

---

### Mensagem 1 — Usuário inicia o contato

**Usuário:** `Oi`

**Ação do sistema:**
- `internal/identity` identifica ou cria o usuário a partir do número do WhatsApp.
- `internal/agent` cria a `Thread` e abre o primeiro `Run`.
- `internal/onboarding` cria a sessão de onboarding em `in_progress`.
- `RunOnboardingTurn` detecta `phase=''`, emite boas-vindas via LLM (fallback `scriptWelcome`) e seta `phase='objective'`.

**Use cases:**
- Identificação/criação do usuário (`internal/identity`).
- `ThreadGateway.GetOrCreate` (`internal/agent`).
- `MarkWelcomeSent` + `SetOnboardingPhase` (`internal/onboarding`).

**Tabelas:**
- `(I) mecontrola.users` — novo usuário criado.
- `(I) mecontrola.identity_entitlements` — plano vinculado.
- `(I) mecontrola.onboarding_tokens` — token marcado como `CONSUMED`.
- `(O) mecontrola.onboarding_sessions` — sessão criada com `state='in_progress'`, `phase=''`, `welcome_sent_at`.
- `(A) mecontrola.agent_threads` — thread `(user_id, WhatsApp)`.
- `(A) mecontrola.agent_runs` — run de abertura com status `succeeded`.

**Resposta do MeControla (fallback real do código):**
```
👋 Oi! Eu sou o *MeControla*, seu parceiro financeiro.

📊 Aqui no MeControla todo dinheiro é organizado em apenas *5 categorias*:

💰 *Custo Fixo*
Contas que acontecem todos os meses.

🎓 *Conhecimento*
Cursos, livros e tudo que faz você evoluir.

🎉 *Prazeres*
Lazer, diversão e experiências.

🎯 *Metas*
Objetivos específicos que você deseja realizar.

🏦 *Liberdade Financeira*
Reserva financeira e investimentos.

Pronto 😊

Agora vamos montar seu plano.

🔵 *Etapa 1/4 — Objetivo*
Qual é o seu objetivo principal? (ex: quitar dívidas, fazer uma viagem, criar uma reserva)
```

---

### Mensagem 2 — Define objetivo

**Usuário:** `Quero quitar minhas dívidas`

**Ação do sistema:**
- Fase `objective`. O LLM chama `save_onboarding_objective`.
- Salva o objetivo e o perfil.
- Avança para `budget`.

**Use cases:**
- `SaveOnboardingObjective`
- `SetOnboardingPhase(phase='budget')`
- `AppendOnboardingTurn`

**Tabelas:**
- `(O) mecontrola.onboarding_sessions` — `payload.objective='Quero quitar minhas dívidas'`, `payload.objective_profile`, `phase='budget'`.
- `(A) mecontrola.agent_sessions.pending_action` — draft v2 com `objective_profile` e `step=budget`.
- `(A) mecontrola.agent_runs`.

**Resposta do MeControla (fallback real):**
```
🎯 Anotado: seu foco é *Quero quitar minhas dívidas*. Vou usar isso pra te manter motivado!

🔵 *Etapa 2/4 — Orçamento*

Pra montar seu plano, qual é o seu *orçamento mensal*? (quanto você tem por mês)
```

---

### Mensagem 3 — Define orçamento

**Usuário:** `4000`

**Ação do sistema:**
- Fase `budget`. O LLM chama `save_onboarding_income` com `income_cents=400000`.
- Publica `IncomeRegistered`.
- Chama `SuggestBudgetSplit` para gerar splits automáticos.
- Persiste o draft v2 com `income=400000` e `auto_splits`.
- Avança para `cards`.

**Use cases:**
- `SaveOnboardingIncome`
- `SuggestBudgetSplit`
- `SetOnboardingPhase(phase='cards')`
- `AppendOnboardingTurn`

**Tabelas:**
- `(O) mecontrola.onboarding_sessions` — `payload.income_cents=400000`, `phase='cards'`.
- `(P) mecontrola.outbox_events` — evento `IncomeRegistered`.
- `(A) mecontrola.agent_sessions.pending_action` — draft v2 com `income`, `auto_splits`, `step=cards`.
- `(B) mecontrola.budgets` — criado/atualizado pelo consumer de `IncomeRegistered`.
- `(A) mecontrola.agent_runs`.

**Resposta do MeControla (possivel, com splits sugeridos):**
```
✅ Orçamento de *R$ 4.000* registrado!

📊 Aqui está uma sugestão de distribuição para o seu orçamento:

💰 *Custo Fixo*: R$ 2.000
🎓 *Conhecimento*: R$ 300
🎉 *Prazeres*: R$ 500
🎯 *Metas*: R$ 700
🏦 *Liberdade Financeira*: R$ 500

🔵 *Etapa 3/4 — Cartões*

💳 Você usa *cartão de crédito*?

Se usar, me responda assim:

Nubank 13 / Inter 5 / Itaú 10

(apelido + dia de fechamento)

Se não usar, basta responder: *Não uso*
```

---

### Mensagem 4 — Cadastra cartão

**Usuário:** `Nubank 13`

**Ação do sistema:**
- Fase `cards`. O LLM chama `save_onboarding_card` com `nickname='Nubank'`, `closing_day=13`.
- Publica `CardRegistered`.
- Avança para `financial_plan`.

**Use cases:**
- `SaveOnboardingCard`
- `SetOnboardingPhase(phase='financial_plan')`
- `AppendOnboardingTurn`

**Tabelas:**
- `(O) mecontrola.onboarding_sessions` — `payload.cards=[{name:'Nubank', closing_day:13, ...}]`.
- `(P) mecontrola.outbox_events` — evento `CardRegistered`.
- `(C) mecontrola.cards` — cartão criado pelo consumer de `CardRegistered`.
- `(A) mecontrola.agent_runs`.

**Resposta do MeControla (template real `buildFinancialPlanMessage`):**
```
💳 Cartão *Nubank* salvo (fecha dia 13 📅). Quer adicionar outro? Se não usa cartão, é só dizer.

🔵 *Etapa 4/4 — Plano Financeiro*

🎯 *Objetivo*: Quero quitar minhas dívidas
💰 *Orçamento*: R$ 4.000/mês
💳 *Cartões*: Nubank (fecha dia 13)

📋 *Distribuição Final*:

💰 *Custo Fixo*: R$ 2.000
🎓 *Conhecimento*: R$ 300
🎉 *Prazeres*: R$ 500
🎯 *Metas*: R$ 700
🏦 *Liberdade Financeira*: R$ 500

Responde *sim* pra confirmar, ou me diz como quer distribuir.
```

---

### Mensagem 5 — Confirma plano financeiro

**Usuário:** `Sim`

**Ação do sistema:**
- Fase `financial_plan`. Como a mensagem não tem dígitos e é afirmativa, o sistema constrói automaticamente a tool call `save_onboarding_budget_splits` com os splits sugeridos.
- Salva os splits.
- Publica `SplitsCalculated`.
- Limpa o draft v2.
- Avança para `first_tx`.

**Use cases:**
- `SaveOnboardingBudgetSplits`
- `SetOnboardingPhase(phase='first_tx')`
- `AppendOnboardingTurn`

**Tabelas:**
- `(O) mecontrola.onboarding_sessions` — `payload.custom_split` com as 5 alocações, `phase='first_tx'`.
- `(P) mecontrola.outbox_events` — evento `SplitsCalculated`.
- `(A) mecontrola.agent_sessions.pending_action` — limpo (`{}`).
- `(B) mecontrola.budgets` / `mecontrola.budgets_allocations` — atualizados pelo consumer.
- `(A) mecontrola.agent_runs`.

**Resposta do MeControla (fallback real):**
```
✅ *Distribuição salva!*
💰 Custo Fixo: R$ 2.000 (50%)
🎓 Conhecimento: R$ 300 (7%)
🎉 Prazeres: R$ 500 (12%)
🎯 Metas: R$ 700 (17%)
🏦 Liberdade Financeira: R$ 500 (12%)

🚀 Agora é só começar! Me manda seu *primeiro lançamento* do jeito que você fala:

"gastei 35 no mercado" ou "recebi 2500 de salário"
```

---

### Mensagem 6 — Primeira movimentação

**Usuário:** `Mercado 120 pix`

**Ação do sistema:**
- Fase `first_tx`. O LLM chama `record_transaction`.
- A tool delega para `ExpenseRecorder` (`internal/transactions`), que registra a despesa.
- Chama `MarkFirstTransactionRecorded`.
- Chama `CompleteOnboardingSession`, que publica `OnboardingCompleted`.

**Use cases:**
- `RecordTransaction` / `RecordExpense` (`internal/transactions`).
- `MarkFirstTransactionRecorded` (`internal/onboarding`).
- `CompleteOnboardingSession` (`internal/onboarding`).

**Tabelas:**
- `(T) mecontrola.transactions` — despesa `Mercado 120 pix` registrada.
- `(O) mecontrola.onboarding_sessions` — `payload.first_tx_recorded=true`, `payload.completed_at`, `state='active'`, `recent_turns` limpo.
- `(P) mecontrola.outbox_events` — evento `OnboardingCompleted`.
- `(A) mecontrola.agent_runs` — runs da tool `record_transaction` e da conclusão.

**Resposta do MeControla:**
```
🏆 Boa! Registrei *R$ 120 em custo_fixo/supermercado*. Esse é o primeiro passo pro seu controle financeiro!

🎉 *Onboarding concluído!* Agora é só me chamar: registrar gastos, ver fatura do cartão, acompanhar metas e pedir o resumo do mês. Conta comigo! ✅
```

---

### Resumo das tabelas ao final do exemplo

| Tabela | Responsável | Estado ao final |
|--------|-------------|-----------------|
| `mecontrola.users` | `internal/identity` | Usuário criado e ativo. |
| `mecontrola.identity_entitlements` | `internal/identity` | Plano ativo vinculado. |
| `mecontrola.onboarding_tokens` | `internal/onboarding` | Token `CONSUMED`. |
| `mecontrola.onboarding_sessions` | `internal/onboarding` | `state='active'`, `payload.completed_at` preenchido, `first_tx_recorded=true`. |
| `mecontrola.outbox_events` | `internal/platform/outbox` | Eventos `IncomeRegistered`, `CardRegistered`, `SplitsCalculated`, `OnboardingCompleted` publicados. |
| `mecontrola.agent_threads` | `internal/agent` | Thread ativa para o usuário. |
| `mecontrola.agent_runs` | `internal/agent` | Múltiplos runs (um por interação), todos auditados. |
| `mecontrola.agent_sessions` | `internal/agent` | `pending_action` limpo (`{}`); `recent_turns` mantido durante o onboarding. |
| `mecontrola.cards` | `internal/card` | Cartão `Nubank` cadastrado. |
| `mecontrola.budgets` | `internal/budgets` | Orçamento de R$ 4.000 para a competência atual. |
| `mecontrola.budgets_allocations` | `internal/budgets` | 5 alocações conforme splits informados. |
| `mecontrola.transactions` | `internal/transactions` | Despesa `Mercado 120 pix`. |
| `mecontrola.categories` | `internal/categories` | Consultada, não alterada. |
| `mecontrola.category_dictionary` | `internal/categories` | Consultada, não alterada. |
