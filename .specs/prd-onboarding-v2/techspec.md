<!-- spec-hash-prd: df2f65638247544f7c78344d28f2e5d2c949e2dad7c107ad7b2f25a8ccfe5384 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — MeControla Onboarding V2

## Resumo Executivo

O auto-start do onboarding, o LLM mandatório e a homologação de modelos via OpenRouter (Gemini 2.5
Flash Lite, Mistral Small 3.2 24B, Claude Haiku 4.5, GPT-5 Nano) **já estão implementados** no
working tree (Parte 1). Esta especificação cobre o trabalho **restante e novo** para tornar o V2
robusto e production-ready, sem falso positivo:

1. **Persistência isolada** (Parte 2): mover histórico (`recent_turns`) e marcos (`welcome_sent_at`,
   `completed_at`) do onboarding de `mecontrola.agent_sessions` para `onboarding_sessions.payload`,
   eliminando a colisão de canal `(user_id, channel)` entre onboarding e agente principal.
2. **Conclusão determinística**: persistir `completed_at` no mesmo write transacional de
   `state=active`, com detecção explícita de drift; limpar `recent_turns` ao concluir.
3. **WorkingMemory assíncrona**: extrair a síntese inline do dispatcher para um consumidor do evento
   `OnboardingCompleted`, idempotente e com retry do outbox.
4. **Distribuição por objetivo**: classificação híbrida (hint do LLM no parse → fallback keyword →
   default) seleciona um de 5 perfis (basis points em `internal/onboarding`); o **cálculo cents é
   delegado a `internal/budgets`** (`AllocationDistributor`), removendo `buildAutoSplits` do agente.
5. **Coleta de cartão por dia de fechamento**: coletar `closing_day` (derivando `due_day`) honrando o
   contrato de `internal/card`; ajustar tool, scripts e mapeamento.
6. **Contratos de validação por módulo**: respeitar campos obrigatórios/tamanhos/ranges/enums de
   `internal/{onboarding,budgets,card,transactions,categories}` ao chamar (seção dedicada).

A abordagem preserva fronteiras: `internal/agent` continua chamando `internal/onboarding` via
binding→usecase e mantém os primitivos Mastra (Thread, Run, WorkingMemory, Pending Step) restritos
a `internal/agent` (R-AGENT-WF-001). Nenhuma regra de domínio entra em adapters (R-ADAPTER-001).

## Princípio de Fronteira (inegociável)

Cada **bounded context é dono do seu domínio e da sua persistência**; nenhum módulo toca a tabela de
outro. Integração entre módulos ocorre por **binding → usecase** ou por **eventos do outbox**. Mapa
de propriedade neste fluxo (ADR-006):

- `internal/budgets`: distribuição/alocação (`AllocationDistributor`, `BasisPoints`, `RootSlug`) e
  budget real (`CreateBudget`/`ActivateBudget`).
- `internal/card`: cartões. `internal/transactions`: lançamentos.
- `internal/onboarding`: **apenas o fluxo + estado de `onboarding_sessions`** (fase, objetivo,
  intenção de split, `recent_turns`, `welcome_sent_at`, `completed_at`); emite eventos de domínio.
- `internal/agent`: **ponte** — parse via LLM (OpenRouter), scripts/copy e primitivos Mastra próprios
  (Thread, Run, WorkingMemory, Pending Step, restritos a `internal/agent` por R-AGENT-WF-001). Nunca
  regra/persistência de outro módulo.

**Integração já existente (não recriar):** `internal/budgets` consome `onboarding.splits_calculated`
→ `CreateBudget`+`ActivateBudget`; `internal/card` consome `onboarding.card_registered` (onboarding
injeta `SynchronousCardCreator`); a 1ª transação vai por `agent → ExpenseRecorder →
internal/transactions`. Onboarding guardar a intenção (`custom_split`, card draft) no seu `payload`
**não é violação** — é estado de fluxo + fonte do evento.

**Única violação a corrigir:** `buildAutoSplits` (matemática basis points × renda → cents) está em
`internal/agent`. Essa matemática é domínio de `internal/budgets` (`AllocationDistributor`) e será
delegada a ele; o template **perfil → basis points** (ligado ao objetivo) permanece em
`internal/onboarding`. O agente também deixa de ler/gravar histórico de onboarding (vai para usecases
do onboarding).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**`internal/onboarding` — detém TODAS as regras e o estado do onboarding**

- `domain/entities/onboarding_session.go`: `OnboardingSessionPayload` ganha `RecentTurns
  []OnboardingTurn`, `WelcomeSentAt *time.Time`, `CompletedAt *time.Time`; novo tipo `OnboardingTurn{
  Role, Text, OccurredAt }`; novos métodos `WithCompletion(now)`, `WithAppendedTurn(...)` (append
  bounded a 3 pares), `WithWelcomeSent(now)`. `OnboardingCardDraft` carrega `ClosingDay` como dado
  primário.
- `domain/valueobjects/objective_profile.go` (novo): tipo fechado `ObjectiveProfile` + smart
  constructor `ParseObjectiveProfile(string)`; função pura `classifyByKeyword(objective)`; tabela
  perfil→**basis points** (política de recomendação ligada ao objetivo). **A matemática cents NÃO
  vive aqui** — é delegada a `internal/budgets` (ADR-006). DMMF/Decide* puro.
- `application/usecases/suggest_budget_split.go` (novo): recebe `objective`, `objectiveProfile`
  (hint do LLM, opcional) e `incomeCents`; resolve o perfil (`ParseObjectiveProfile` →
  `classifyByKeyword` → default), obtém os **basis points** do template e **delega o cálculo cents**
  ao binding de `internal/budgets` (`SuggestAllocation`). Sem IO de LLM; sem reimplementar a
  distribuição.

**`internal/budgets` — dono da distribuição/alocação**

- `application/usecases/suggest_allocation.go` (novo): encapsula o domain service
  `AllocationDistributor` (`Distribute(totalCents, []AllocationInput{RootSlug, BasisPoints}) →
  []AllocationResult{PlannedCents}`) num usecase exposto via binding, para o onboarding obter o split
  em cents sem tocar a persistência de budgets. A materialização do budget real permanece via evento
  `onboarding.splits_calculated` → `CreateBudget`+`ActivateBudget` (já existente).
- `application/usecases/append_onboarding_turn.go` + `load_onboarding_turns.go` + `mark_welcome_sent.go`
  (novos): gerenciam `recent_turns`/`welcome_sent_at` em `onboarding_sessions` (bounded; idempotente).
- `application/usecases/complete_onboarding_session.go`: grava `completed_at` no payload (via
  `WithCompletion`) no mesmo `uow.Do`; limpa `recent_turns`.
- `application/usecases/save_onboarding_card.go` + DTO: `ClosingDay` como campo coletado, propagado
  ao `SynchronousCardCreator` (→ `internal/card`, já injetado via `WithOnboardingCardCreator`) e ao
  evento `onboarding.card_registered`. O cartão real é criado em `internal/card` (não no onboarding).
- `infrastructure/repositories/postgres/onboarding_session_repository.go`: `onboardingSessionPayloadJSON`
  ganha `recent_turns`, `welcome_sent_at`, `completed_at`; detecção de drift no `Find`.
- `application/services/whatsapp_message_processor.go`: sem mudança funcional (já faz auto-start).
- `application/binding/`: expõe os novos usecases (suggest split, turnos, welcome) para o agente; e
  consome o binding de `internal/budgets` (`SuggestAllocation`) para o cálculo cents.

**`internal/card` — contrato closing_day-only (mudança no dono)**

- `input.CreateCard` + `valueobjects.NewBillingCycle`: `DueDay` passa a ser **opcional**; quando
  ausente, o `card` **deriva** o vencimento a partir de `closing_day` (regra dentro do
  `billing_cycle`/usecase do card). Retrocompatível com callers que ainda enviam `due_day` (GAP-V1).
- Mantém criação via `SynchronousCardCreator` + evento `onboarding.card_registered`.

**`internal/transactions` — sem mudança estrutural (já delegado)**

- 1ª transação: `agent → ExpenseRecorder → TransactionLoggerAdapter → internal/transactions`
  (`CreateTransaction`). `payment_method` ausente → default `pix` (`mapPaymentMethod`). Sem mudança.

**`internal/agent` — ponte (LLM parse + scripts + primitivos Mastra), só chama binding**

- `application/usecases/run_onboarding_turn.go`: remove qualquer cálculo de domínio. Substitui
  `buildAutoSplits(...)` por chamada ao binding `SuggestBudgetSplit`; substitui o
  `onboardingSessionReader` (que apontava para `agent_sessions`) por um `OnboardingHistoryGateway`
  cuja **implementação é um adapter de binding** para os usecases de turno do onboarding (sem SQL no
  agente); `emitWelcome` chama o binding `MarkWelcomeSent` (idempotência).
- `application/usecases/onboarding_tool_catalog.go`: tool `save_onboarding_card` exige `closing_day`
  (1..31); tool `save_onboarding_objective` ganha enum opcional `objective_profile` preenchido pelo
  LLM no parse (forwardado ao usecase do onboarding — o agente não decide o split).
- `application/usecases/onboarding_scripts.go`: `scriptCardQuestion`/`scriptCards` passam a dizer
  "dia de fechamento" (copy de apresentação — concern do agente).
- `infrastructure/onboarding/onboarding_tool_dispatcher.go`: remove `synthesizeAndStoreWM` inline;
  mapeia `closing_day`; o dispatcher continua chamando usecases do onboarding (já é o padrão).
- `infrastructure/.../onboarding_history_gateway.go` (novo): adapter fino que implementa
  `OnboardingHistoryGateway` chamando os usecases `AppendOnboardingTurn`/`LoadOnboardingTurns`/
  `MarkWelcomeSent` do onboarding via binding.
- `infrastructure/messaging/database/consumers/onboarding_completed_consumer.go` (novo): consome
  `OnboardingCompleted`, lê o contexto via binding `GetOnboardingContext` e faz `WorkingMemory.Upsert`
  (WM é primitivo do agente — agent-owned). Idempotente; erro → retry do outbox.
- `infrastructure/messaging/database/consumers/onboarding_bound_consumer.go`: GAP-1 — retorna **erro**
  quando a sessão ainda não existe/`InProgress=false`, forçando retry do outbox.

### Fluxo de Dados (resumido)

```
ATIVAR → ConsumeMagicToken → outbox(onboarding.subscription_bound{user_id,peer_e164})
  → [agent] OnboardingBoundConsumer → IntentRouter.RouteWhatsApp(__onboarding_welcome__)
      → RunOnboardingTurn.emitWelcome → binding MarkWelcomeSent (idempotente) → 1ª pergunta

turnos → RunOnboardingTurn (LLM parse no agente) → binding → usecases do onboarding
  (estado + recent_turns em onboarding_sessions; o agente NÃO acessa onboarding_sessions direto,
   NUNCA usa agent_sessions p/ onboarding)
  budget → binding SuggestBudgetSplit (onboarding decide os %) → preview

1ª transação → dispatcher → binding CompleteOnboardingSession (uow no onboarding: state=active +
  completed_at + limpa recent_turns + publish OnboardingCompleted) → outbox
  → [agent] OnboardingCompletedConsumer → binding GetOnboardingContext → WorkingMemory.Upsert (agent)
```

## Design de Implementação

### Interfaces Chave

**No `internal/budgets` (dono da distribuição/alocação):**

```go
type SuggestAllocationInput struct {
    TotalCents  int64
    Allocations []AllocationBP // { RootSlug string; BasisPoints int }
}
type SuggestAllocationResult struct { Allocations []AllocationCents } // { RootSlug; BasisPoints; PlannedCents }

type SuggestAllocation interface {
    Execute(ctx context.Context, in SuggestAllocationInput) (SuggestAllocationResult, error)
}
```

`SuggestAllocation` encapsula o domain service `AllocationDistributor` (puro). É a **única** fonte da
matemática cents — `internal/onboarding` e `internal/agent` não a reimplementam.

**No `internal/onboarding` (dono do fluxo; recomendação ligada ao objetivo):**

O usecase resolve o perfil (objetivo) → basis points e **delega o cálculo cents** ao binding de
budgets:

```go
type SuggestBudgetSplitInput struct {
    UserID           uuid.UUID
    ObjectiveProfile string // hint do LLM (enum), pode vir vazio
    Objective        string // texto livre, para fallback por keyword
    IncomeCents      int64
}
type SuggestBudgetSplitResult struct { Splits []OnboardingSplitView }

type SuggestBudgetSplit interface { // resolve perfil → basis points → budgets.SuggestAllocation
    Execute(ctx context.Context, in SuggestBudgetSplitInput) (SuggestBudgetSplitResult, error)
}

type BudgetAllocator interface { // seam no consumidor (onboarding) → binding p/ internal/budgets
    Suggest(ctx context.Context, totalCents int64, bp []budgets.AllocationBP) ([]budgets.AllocationCents, error)
}
```

Usecases de histórico/marcos do onboarding (operam em `onboarding_sessions`):

```go
type AppendOnboardingTurn interface {
    Execute(ctx context.Context, userID uuid.UUID, userMsg, assistantReply string) error
}
type LoadOnboardingTurns interface {
    Execute(ctx context.Context, userID uuid.UUID) ([]entities.OnboardingTurn, error)
}
type MarkWelcomeSent interface {
    Execute(ctx context.Context, userID uuid.UUID) (alreadySent bool, err error) // RF-29
}
```

**No `internal/agent` (interfaces no consumidor — R6.3 — implementadas por adapters de binding):**

```go
type OnboardingHistoryGateway interface {
    LoadTurns(ctx context.Context, userID uuid.UUID) ([]onbentities.OnboardingTurn, error)
    AppendTurn(ctx context.Context, userID uuid.UUID, userMsg, assistantReply string) error
    MarkWelcomeSent(ctx context.Context, userID uuid.UUID) (alreadySent bool, err error)
}
type BudgetSplitSuggester interface {
    Suggest(ctx context.Context, userID uuid.UUID, objectiveProfile, objective string, incomeCents int64) ([]onbusecases.OnboardingSplitView, error)
}
```

As implementações desses gateways são adapters finos que chamam os usecases do onboarding via
binding — **sem SQL e sem regra no agente** (R-ADAPTER-001). `MarkWelcomeSent` retorna
`alreadySent=true` quando `welcome_sent_at` já está preenchido; `emitWelcome` então não reenvia.

Consumidor de conclusão (novo) — escrita em WorkingMemory (agent-owned) após leitura via binding:

```go
type OnboardingContextReader interface {
    Execute(ctx context.Context, in onbusecases.GetOnboardingContextInput) (onbusecases.GetOnboardingContextResult, error)
}
type WorkingMemoryRepository interface { // já existe em internal/agent
    Get(ctx context.Context, userID uuid.UUID) (entities.WorkingMemory, bool, error)
    Upsert(ctx context.Context, wm entities.WorkingMemory) error
}
```

Classificação objetivo→perfil — **domínio do onboarding**, em `internal/onboarding/domain/valueobjects`
(DMMF state-as-type, Decide* puro; sem IO, sem LLM):

```go
type ObjectiveProfile int

const (
    ProfileOrganizeSpending ObjectiveProfile = iota + 1 // default/fallback (R5.8)
    ProfilePayoffDebt
    ProfileEmergencyFund
    ProfileInvest
    ProfileSpecificGoal
)

func ParseObjectiveProfile(raw string) (ObjectiveProfile, bool) // hint do LLM
func classifyByKeyword(objective string) (ObjectiveProfile, bool) // fallback determinístico
func SplitTemplate(p ObjectiveProfile) []SplitEntryBP            // basis points, soma 10000
```

A resolução híbrida (ADR-004) ocorre no usecase `SuggestBudgetSplit`: `ParseObjectiveProfile(hint)` →
se inválido, `classifyByKeyword(objective)` → se nada casar, `ProfileOrganizeSpending`.

### Modelos de Dados

`OnboardingSessionPayload` (domínio) — campos adicionados:

```go
type OnboardingTurn struct {
    Role       string    // "user" | "assistant"
    Text       string
    OccurredAt time.Time
}

type OnboardingSessionPayload struct {
    // ...campos atuais...
    RecentTurns   []OnboardingTurn
    WelcomeSentAt *time.Time
    CompletedAt   *time.Time
}
```

`onboardingSessionPayloadJSON` (persistência) — JSON estendido (sem migração de schema; a coluna
`payload` já é JSONB):

```go
RecentTurns      []onboardingTurnJSON `json:"recent_turns,omitempty"`
WelcomeSentAt    *time.Time           `json:"welcome_sent_at,omitempty"`
CompletedAt      *time.Time           `json:"completed_at,omitempty"`
ObjectiveProfile string               `json:"objective_profile,omitempty"`
```

`objective_profile` guarda o perfil **resolvido** (ADR-004) para reprodutibilidade do split — o
objetivo (texto) já é persistido; o perfil resolvido também, para auditoria e recálculo estável.

`recent_turns` é bounded em **3 pares** (6 entradas máx.), aplicado no append (DR-08/RF-20). Ao
concluir, `recent_turns` é zerado (RF-35).

Tabela de perfis de distribuição (RF-13, basis points, cada linha soma 10000):

| Perfil (`ObjectiveProfile`) | CF | Conh | Praz | Metas | LF |
|---|---|---|---|---|---|
| PayoffDebt (quitar dívidas) | 4500 | 500 | 1000 | 2500 | 1500 |
| EmergencyFund (reserva) | 4000 | 500 | 1000 | 1500 | 3000 |
| Invest (investir/patrimônio) | 4000 | 1000 | 1000 | 1000 | 3000 |
| SpecificGoal (meta específica) | 4000 | 500 | 1000 | 3000 | 1500 |
| OrganizeSpending (default) | 4000 | 1000 | 1500 | 2000 | 1500 |

O cálculo vive em `internal/onboarding` (usecase `SuggestBudgetSplit` + `SplitTemplate` no VO): o
perfil é resolvido (hint do LLM → keyword → default), o template é aplicado e a última categoria
absorve o resto para fechar exatamente o orçamento. O agente apenas chama o binding e renderiza o
preview; `buildAutoSplits` é removido de `internal/agent` (correção do Princípio de Fronteira).

### Cartão por dia de fechamento (DR-05)

- Tool `save_onboarding_card`: `required: [nickname, closing_day]`, `closing_day` inteiro 1..31.
- `SaveOnboardingCardInput`/use case: campo `ClosingDay`; persiste em `OnboardingCardDraft.ClosingDay`.
- Dispatcher: `ClosingDay: int(intArg(call.ArgumentsJSON, "closing_day"))`; resposta "fecha dia %d".
- `scriptCardQuestion`/`scriptCards`: "apelido + dia de fechamento".
- `due_day` e `limit_cents` **não** são coletados no onboarding (cartão skeleton; completados depois).

### Scripts e Copy (contrato — RF-09/10/11/16/17)

O texto literal vive nas constantes de `internal/agent/application/usecases/onboarding_scripts.go`
(copy de apresentação = concern da ponte). Itens **contratuais** (devem bater literalmente; sem
perguntas de confirmação — RF-09):

- **Bloco das 5 categorias (RF-10)** — uma única mensagem (fonte: `MeControla_Onboarding_V2.md`):
  `📊 Aqui no MeControla todo dinheiro é organizado em apenas 5 categorias:` seguido de
  `💰 Custo Fixo` / `🎓 Conhecimento` / `🎉 Prazeres` / `🎯 Metas` / `🏦 Liberdade Financeira`, cada
  uma com uma linha curta de descrição, encerrando com `Pronto 😊 Agora vamos montar seu plano.`
- **Indicador de progresso (RF-11)** — em toda interação de onboarding: `🔵 Etapa 1/4 — Objetivo`,
  `🔵 Etapa 2/4 — Orçamento`, `🔵 Etapa 3/4 — Cartões`, `🔵 Etapa 4/4 — Plano Financeiro`. A 1ª
  transação (RF-17) é continuação após a Etapa 4/4, sem numeração nova.
- **Cartões (RF-12/DR-05)**: `scriptCardQuestion`/`scriptCards` pedem "apelido + dia de fechamento"
  (`ex.: Nubank 13 / Inter 5 / Itaú 10`), aceitando "Não uso".
- **Resumo final enxuto (RF-16)**: apenas Objetivo, Orçamento, Cartões e Distribuição Final — sem
  recapitular categorias nem pedir confirmação.
- **Não encerrar (RF-17)**: após o resumo, emitir imediatamente o prompt de 1ª transação
  (`scriptFirstTx`).

Gate de fidelidade: os testes e o runbook comparam a narração determinística contra essas constantes
(memória [[feedback_runbook_fidelity_and_examples]]). Alterar o texto exige atualizar runbook + teste.

### WorkingMemory assíncrona (DR-02)

- Novo `OnboardingCompletedConsumer` (adapter fino, R-ADAPTER-001.2): desserializa o envelope, extrai
  `user_id`, chama `OnboardingContextReader` (binding), monta o markdown via função pura
  `buildWorkingMemory(context)` e faz `WorkingMemoryRepository.Upsert`. Idempotente: se já existe WM
  com conteúdo, não sobrescreve (retorna nil). Falha de leitura/escrita retorna **erro** para retry
  do outbox.
- `onboarding_tool_dispatcher.go`: remover as chamadas inline `synthesizeAndStoreWM` em
  `dispatchRecordTransaction` e `dispatchComplete`, e a dependência `wmWriter`/`contextReader` do
  dispatcher (passam a viver no consumidor).

### Conclusão determinística e drift (RF-23..25, RF-31)

`CompleteOnboardingSession.Execute` (já transacional via `uow.Do`):

```go
if session.IsActive() { return AlreadyActive }      // idempotente
if !session.HasFirstTransaction() { return ErrFirstTransactionRequired }
updated := session.WithCompletion(now)              // state=active + payload.CompletedAt=now + RecentTurns=nil
repo.Upsert(updated)
publisher.Publish(OnboardingCompleted{...})         // mesma TX
```

Drift (RF-31): no `Find`, se `state=active` e `CompletedAt == nil`, incrementar contador
`onboarding_state_drift_total` e logar warn (não tratar como erro de leitura nem como sucesso
silencioso).

### Idempotência da saudação e ordem de entrega (RF-29, EB-01, EB-02)

Decisão: marcar `welcome_sent_at` **após** o envio bem-sucedido (at-least-once), com `AgentDecision`
por `envelope.EventID` como segunda barreira anti-duplicação:

1. `OnboardingBoundConsumer` usa `MessageID = envelope.EventID.String()` (estável por evento).
2. Antes de rotear a saudação, verifica `AgentDecisionRepository.FindByMessage(user, channel,
   eventID)`; se já existe, é replay → não reenvia (retorna nil).
3. Se a sessão ainda não existe/`InProgress=false` → retorna **erro** (retry do outbox — GAP-1).
4. Roteia a saudação; **após** o gateway confirmar o envio, registra a `AgentDecision` (event_id) e
   chama o binding `MarkWelcomeSent` (grava `welcome_sent_at`).
5. Se o envio falhar antes de marcar, o retry do outbox reenvia (at-least-once). Se uma marcação
   falhar após o envio, a barreira `welcome_sent_at` (consultada por `emitWelcome`) evita duplicação
   no caminho de turno.

`emitWelcome` consulta `welcome_sent_at` (via snapshot/binding) e não reemite quando já presente —
cobrindo a reentrância pelo caminho de turno conversacional além do caminho do consumer.

### Degradação por falha de LLM (RF-08, RF-32, EB-03)

Decisão de MVP: **retry seguro, sem FSM completa**. Em falha do LLM no parse/interpretação de um
turno, `RunOnboardingTurn` retorna erro; o estado persistido em `onboarding_sessions` é **preservado
intacto** (nenhum `SetPhase`/`Save`/`Complete` é executado no caminho de erro), nada é concluído e o
agente principal não é contaminado. A "degradação" (RF-08) é o retry seguro: o usuário reenvia (ou o
caminho de saudação proativa é reprocessado pelo outbox). Garantias verificáveis:

- `RunOnboardingTurn` não persiste transição quando `interpreter.Interpret` retorna erro (teste).
- `CompleteOnboardingSession` nunca é chamado em caminho de erro de LLM (RF-32/EB-03).
- A FSM determinística (`onboarding_workflow.go`) **não** é reidratada no caminho LLM V2 neste MVP;
  fica como ativo existente, fora do escopo do V2 (registrado como contexto não carregado).

## Contratos de Validação por Módulo (inegociável ao chamar)

Cada bounded context **valida e é dono das suas regras**; o onboarding/agent devem respeitar esses
contratos ao chamar (campos obrigatórios, tamanhos, ranges, enums). Quebra → o módulo dono rejeita.

### `internal/onboarding` (próprio)
| Campo | Regra | Sentinel |
|---|---|---|
| Objetivo | não-vazio (trim), ≤ 280 runes | `ErrFinancialObjectiveEmpty/TooLong` |
| Renda | **[50000..10000000000] cents (R$500..R$1B)** | `ErrMonthlyIncomeBelow/AboveMaximum` |
| Card nickname (draft) | não-vazio (trim) | `ErrOnboardingCardNicknameRequired` |
| Card dia | 1..31 (`CardDueDay`/`CardClosingDay` VOs já existem) | `ErrCardDueDayOutOfRange`/`ErrCardClosingDayOutOfRange` |
| Budget allocation | exatamente 5 categorias, cada amount ≥ 0, **soma == income (exata)**, sem duplicata de Kind | `ErrBudgetAllocationWrongSize/OutOfRange/SumMismatch` |

### `internal/budgets` (alocação) — ao criar/sugerir
| Campo | Regra | Sentinel |
|---|---|---|
| `TotalCents` | > 0 | `ErrInput/CommandInvalidTotalCents` |
| `Competence` | formato `YYYY-MM`, mês 01..12 | `ErrCompetenceInvalid` |
| `Allocations` | não-vazio | `ErrInputAllocationsEmpty` |
| `RootSlug` | whitelist: `expense.{custo_fixo,conhecimento,prazeres,metas,liberdade_financeira}` | `ErrRootSlugUnknown` |
| `BasisPoints` | individual 0..10000; **Σ ≤ 10000 (create), = 10000 (edit)** | `ErrBasisPointsOutOfRange`/`ErrCommandInvalidAllocation` |

### `internal/card` (cartão real via `SynchronousCardCreator`)
| Campo | Regra | Sentinel |
|---|---|---|
| `Name` | 1..64 | `ErrInvalidCardName` |
| `Nickname` | **1..32** | `ErrInvalidNickname` |
| `ClosingDay` | 1..31 | `ErrInvalidClosingDay` |
| `DueDay` | **1..31 (obrigatório)** | `ErrInvalidDueDay` |
| `LimitCents` | 0..100000000 (R$1M) | `ErrCardLimitNegative/TooLarge` |

### `internal/transactions` (1ª transação via `ExpenseRecorder`)
| Campo | Regra | Sentinel |
|---|---|---|
| `Direction` | enum `income|outcome` | `ErrDirectionUnknown` |
| `PaymentMethod` | enum (`pix,ted,debit_in_account,debit_card,cash,boleto,credit_card`); **`doc` proibido no create** | `ErrPaymentMethodUnknown/DocReadOnly` |
| `AmountCents` | > 0 | `ErrMoneyMustBePositive` |
| `Description` | não-vazio, ≤ 500 | `ErrDescriptionEmpty/TooLong` |
| `CategoryID` | obrigatória; **outcome exige `SubcategoryID`** | `ErrInputCategoryIDRequired` + regra do usecase |

### `internal/categories`
| Item | Regra | Sentinel |
|---|---|---|
| Root slug | whitelist dos 5 `expense.*` | `ErrRootSlugUnknown` |
| Slug (VO) | kebab-case, 2..64, sem hífen nas pontas/duplo | `ErrSlug*` |

**Gaps de contrato a tratar no V2 (impactam tasks 7 e 9):**
- **GAP-V1 — Card exige `closing_day` E `due_day` (resolvido em `internal/card`)**: decisão — o
  **módulo `card` passa a exigir apenas `closing_day`**, tornando `DueDay` **opcional** em
  `input.CreateCard`/`NewBillingCycle` e **derivando-o internamente** a partir de `closing_day` quando
  ausente (o `billing_cycle.go` usa `due_day` para calcular o vencimento; a derivação fica no dono da
  regra). Retrocompatível: callers existentes (HTTP handler, daily) continuam podendo informar
  `due_day`. O onboarding/agent passam a enviar **somente `closing_day`** (Tarefa 13.0 no `card`;
  Tarefas 7.0/8.0 deixam de enviar `due_day`). Substitui a derivação inversa atual
  (`closing_day = due_day - 7`).
- **GAP-V2 — Nickname ≤ 32**: o draft do onboarding não limita nickname, mas `internal/card` exige
  1..32. O DTO/tool do V2 DEVE validar `nickname` ≤ 32 na fronteira (Tarefas 7.0/9.0) para evitar
  rejeição a jusante.
- **GAP-V3 — Renda mínima R$500**: se o usuário informar renda < R$500, o módulo rejeita
  (`ErrMonthlyIncomeBelowMinimum`); o fluxo DEVE tratar como re-pergunta amigável, não erro fatal
  (edge case EB-15).
- **GAP-V4 — Soma do split == income (exata)**: `BudgetAllocation` exige soma exata; o cálculo cents
  de `internal/budgets` (`AllocationDistributor`, que faz o último absorver o resto) satisfaz isso —
  o V2 NÃO deve arredondar por conta própria (Tarefas 4.0/5.0).
- **GAP-V5 — 1ª transação outcome exige subcategoria + payment_method válido**: a categorização do
  agente (`CategoryResolver`) já resolve; documentar que `doc` é proibido e outcome precisa de
  subcategoria (Tarefa 11.0/12.0 e2e).

## Pontos de Integração

- **Outbox** (`internal/platform/outbox`): novo registro de handler para
  `OnboardingCompleted.EventType()` no `internal/agent/module.go` (`buildEventHandlers`), ao lado do
  já existente para `onboarding.subscription_bound`.
- **Binding agent→onboarding**: o consumidor de conclusão usa o use case `GetOnboardingContext` já
  exposto pelo módulo onboarding (sem acesso a SQL nem a outra TX — memória
  [[feedback_agent_calls_modules_own_persistence]]).
- **OpenRouter / FallbackChain**: sem mudança — onboarding já usa `OnboardingModel` (primário) +
  `FallbackModels` (degradação) sobre a allowlist de 4 modelos (DR-01 satisfeito).

## Abordagem de Testes

### Testes Unitários

- `ParseObjectiveProfile`/`classifyByKeyword` (onboarding VO): tabela de objetivos (pt-br) → perfil;
  hint válido do LLM prevalece; ambíguo/desconhecido → `ProfileOrganizeSpending` (RF-13a/EB-13).
  Funções puras, sem mock.
- `SuggestBudgetSplit` (onboarding usecase) + `SplitTemplate`: cada perfil soma exatamente o
  orçamento; última categoria absorve resto; income 0 e valores não divisíveis; resolução híbrida.
- `OnboardingSessionPayload`/`WithCompletion`: `CompletedAt` setado, `RecentTurns` zerado, append
  bounded a 3 pares.
- `CompleteOnboardingSession` (suite testify, padrão R-TESTING-001): sucesso (active+completed_at+
  evento), `AlreadyActive` (idempotente), `ErrFirstTransactionRequired`, falha de upsert/publish.
- `OnboardingCompletedConsumer`: payload válido → Upsert chamado; WM já existente → no-op; erro de
  contexto/Upsert → retorna erro (retry); `user_id` inválido → `decodeFailed`+erro.
- `OnboardingHistoryGateway`: load/append/MarkWelcomeSent idempotente (`alreadySent`), bounded.
- `RunOnboardingTurn.emitWelcome`: segunda chamada com `welcome_sent_at` setado não reemite (RF-29).
- Card por `closing_day`: parse e persistência mapeiam para `ClosingDay`; `closing_day` fora de 1..31
  → re-pergunta (EB-14).
- **Isolamento**: garantir que `RunOnboardingTurn` não referencia mais `agent_sessions` (gate de
  revisão: grep do `onboardingSessionReader` removido).

### Testes de Integração

> Critérios atendidos: (1) fronteiras de IO críticas (Postgres JSONB, outbox) onde mock não garante
> correção; (2) risco de falso positivo na conclusão e de colisão entre tabelas. **Recomendados.**

Com testcontainers-go (`//go:build integration`), reaproveitando o padrão de
`subscription_bound_integration_test.go`:

- Isolamento: após um onboarding completo, `agent_sessions` não contém histórico/estado do
  onboarding; `onboarding_sessions.payload` contém `recent_turns` + estado (RF-19/21, EB-12).
- Conclusão determinística: `state=active` ⇒ `completed_at` presente; evento `OnboardingCompleted` na
  outbox (RF-24/25).
- Retomada: parar no meio e recarregar retoma a fase persistida com dados preservados (RF-30, EB-04).
- Idempotência do greeting: reprocessar `subscription_bound` não reemite saudação quando
  `welcome_sent_at` presente (RF-29, EB-02).
- Drift: `state=active` sem `completed_at` incrementa `onboarding_state_drift_total` (RF-31, EB-10).
- WM assíncrona: consumir `OnboardingCompleted` persiste WorkingMemory; reprocesso não duplica
  (RF-34, EB-11).

### Testes E2E

Estender `internal/onboarding/e2e/support_runtime_test.go`: fluxo `ATIVAR → intro → 4 etapas →
1ª transação → conclusão → handoff`, verificando que mensagem pós-conclusão é tratada pelo agente
principal (RF-28, EB-09) e que durante o onboarding o daily não recebe mensagens (RF-27, EB-12).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **[onboarding] Domínio do split** (VO `ObjectiveProfile`, `ParseObjectiveProfile`,
   `classifyByKeyword`, `SplitTemplate` → basis points) — puro. (ADR-004, ADR-006)
2. **[onboarding] Payload isolado** (`OnboardingTurn`, `RecentTurns/WelcomeSentAt/CompletedAt`,
   métodos `With*`). (ADR-001, ADR-002)
3. **[onboarding] Repository** (JSON dos novos campos + drift no `Find`). (ADR-001, ADR-002)
4. **[budgets] `SuggestAllocation`** (encapsula `AllocationDistributor`) + binding. (ADR-006)
5. **[onboarding] `SuggestBudgetSplit`** (resolve perfil → basis points → delega cents ao binding de
   budgets) + binding. (ADR-004, ADR-006)
6. **[onboarding] Lifecycle** (`AppendOnboardingTurn`, `LoadOnboardingTurns`, `MarkWelcomeSent`,
   `CompleteOnboardingSession` com completed_at/limpa turns) + binding. (ADR-001, ADR-002)
6b. **[card] Contrato closing_day-only** (`DueDay` opcional em `input.CreateCard`/`NewBillingCycle`;
   derivação interna quando ausente; retrocompatível). (ADR-005, GAP-V1) — Tarefa 13.0.
7. **[onboarding] Cartão por fechamento** (DTO/use case `ClosingDay`; envia só `closing_day` ao
   `SynchronousCardCreator` → internal/card, que deriva o resto). (ADR-005)
8. **[agent] Adapters de binding + `RunOnboardingTurn`** (remove `agent_sessions` e `buildAutoSplits`;
   chama `SuggestBudgetSplit`/`OnboardingHistoryGateway`; `emitWelcome` → `MarkWelcomeSent`). (ADR-001,
   ADR-004, ADR-006)
9. **[agent] Tools/scripts** (`closing_day`, enum `objective_profile`, copy "fechamento"). (ADR-004,
   ADR-005)
10. **[agent] Consumidor de conclusão** + wiring; remover síntese inline do dispatcher. (ADR-003)
11. **[agent] Hardening do greeting** (GAP-1: erro→retry + idempotência por event_id/`welcome_sent_at`).
12. **Validação** (integração → e2e) e gates de fronteira/conformidade.

### Dependências Técnicas

- Postgres (JSONB `payload`) — sem migração de schema (coluna já existe).
- Outbox dispatcher, binding `GetOnboardingContext`, `AllocationDistributor` em `internal/budgets`,
  `SynchronousCardCreator` (→ internal/card) e `ExpenseRecorder` (→ internal/transactions) — já
  disponíveis.

## Monitoramento e Observabilidade

Métricas (labels enums fechados — R-TXN-004/R-AGENT-WF-001.5; nunca `user_id`):

- `agent_onboarding_turn_total{phase,outcome}` — já existe.
- `onboarding_state_drift_total` — novo (RF-31).
- `agent_onboarding_welcome_dedup_total{result}` — novo (`result=sent|skipped`, RF-29).
- `agent_onboarding_completed_consumer_total{result}` e
  `agent_onboarding_completed_consumer_decode_failed_total` — novos (DR-02).
- `agent_onboarding_bound_consumer_decode_failed_total` — já existe.

**Cap de retry + dead-letter (decisão de robustez):** os consumidores de saudação (GAP-1, erro→retry)
e de WorkingMemory dependem do retry do outbox, que **já suporta** `attempts`/`max_attempts` +
backoff e marca o evento como dead após o teto (`dispatcher.go:136-137`). O V2 DEVE: (1) configurar
`max_attempts` adequado para `onboarding.subscription_bound` e `onboarding.completed`; (2) emitir
alerta/métrica `outbox_dead_letter_total{event_type}` quando um evento de onboarding for
dead-lettered (ex.: sessão nunca criada / contexto indisponível), evitando retry infinito silencioso.
Não requer nova infra — reusa o mecanismo existente do outbox.

Spans: `onboarding.usecase.complete_session`, `agent.consumer.onboarding_completed` (novo),
`agent.usecase.run_onboarding_turn`. Logs warn: `onboarding_not_started` (retry do greeting),
`onboarding.state_drift`.

## Considerações Técnicas

### Decisões Chave

- [ADR-001 — Persistência isolada do onboarding em `onboarding_sessions.payload`](adr-001-persistencia-isolada-onboarding.md)
- [ADR-002 — Conclusão determinística com `completed_at` e detecção de drift](adr-002-conclusao-deterministica.md)
- [ADR-003 — Síntese de WorkingMemory assíncrona via consumidor de `OnboardingCompleted`](adr-003-working-memory-assincrona.md)
- [ADR-004 — Distribuição por objetivo via perfis fixos determinísticos](adr-004-distribuicao-por-objetivo.md)
- [ADR-005 — Coleta de cartão por dia de fechamento](adr-005-cartao-dia-fechamento.md)
- [ADR-006 — `internal/agent` como ponte: zero domínio de outro módulo](adr-006-agent-como-ponte.md)

### Riscos Conhecidos

- **Migração de histórico em voo**: sessões iniciadas antes do deploy têm histórico em
  `agent_sessions`. Mitigação: histórico é efêmero (janela de 3 pares) e o agente principal não usa
  o histórico de onboarding; aceitar perda do histórico de turnos de onboarding em voo (estado
  funcional permanece em `onboarding_sessions`). Sem migração de dados.
- **Classificação de objetivo imprecisa**: heurística por palavra-chave pode classificar errado.
  Mitigação: fallback determinístico para `OrganizeSpending` e ajuste posterior em linguagem natural
  (RF-15); cobertura de testes da tabela de objetivos.
- **GAP-1 (race greeting × criação de sessão)**: hoje o consumidor não força retry. Mitigação na
  ordem 9 (retorna erro → retry do outbox); idempotência garantida por `welcome_sent_at`.
- **Dois consumidores para `subscription_bound`**: onboarding (`SubscriptionBoundSessionConsumer` →
  `StartBudgetConfiguration`, cria/resume sessão) e agente (`OnboardingBoundConsumer`, greeting)
  consomem o mesmo evento; além disso `HandleActivation` já cria a sessão sincronamente. A criação é
  idempotente (`StartBudgetConfiguration` faz Find→Resume) e a saudação é idempotente por
  `welcome_sent_at`, cobrindo qualquer ordem de dispatch.
- **Refator de fronteira (ADR-006)**: mover `buildAutoSplits` e o acesso a histórico de
  `internal/agent` para `internal/onboarding` toca código existente. Mitigação: cobertura de testes
  prévia do comportamento atual (split/retomada) e mudança incremental por usecase.

### Conformidade com Padrões

- **Princípio de Fronteira / ADR-006**: `internal/agent` não detém regra nem persistência de outro
  módulo; toda regra do onboarding vive em `internal/onboarding`, acessada por binding→usecase.
- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): Thread→Run, Tool fina, LLM só no parse,
  `ToolOutcome/RunStatus/AwaitingKind` fechados. O hint `objective_profile` é preenchido pelo LLM no
  parse (conforme R-AGENT-WF-001.4); o **cálculo** do split é determinístico e vive no onboarding.
  Mastra/Pending/WorkingMemory restritos a `internal/agent`.
- **DMMF** (`domain-modeling.md` — *Domain Modeling Made Functional*): aplicado em `internal/onboarding`.
  `ObjectiveProfile` é discriminated union/state-as-type (tornar estados ilegais irrepresentáveis);
  `ParseObjectiveProfile` é *parse, don't validate* (smart constructor); `SuggestBudgetSplit`/
  `WithCompletion` são `Decide*` puros (sem IO, determinísticos, testáveis sem mock); `OnboardingTurn`
  e `ClosingDay` são VOs imutáveis. Anti-padrões proibidos (Result/Either custom, currying, DSL de
  pipeline) **não** introduzidos.
- **shared-patterns.md** (R-PAT-001): Repository (port de domínio, sem SQL na interface), Factory
  Function (`New*(deps) (*T, error)` validando invariantes), Dependency Injection manual por
  construtor (interfaces no consumidor), Value Objects (`ClosingDay`, `ObjectiveProfile` — encapsular
  primitivo só quando carrega invariante), Error Handling cross-stack (sentinels + `%w`, converter
  erro de infra em erro de domínio na fronteira).
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): consumidores e gateways de binding são adapters
  finos (sem SQL/regra/branching de domínio); zero comentários em `.go`.
- `.claude/rules/go-testing.md` (R-TESTING-001): suites testify whitebox para use cases/consumers.
- `.claude/rules/input-dto-validate.md`: `SaveOnboardingCardInput` (closing_day) valida na fronteira
  com `errors.Join` e nome de campo.
- Skills obrigatórias na implementação: `go-implementation` (Etapas 1–5 + checklist R0–R7) e `mastra`.

### Arquivos Relevantes e Dependentes

Modificados — `internal/onboarding` (regras + estado):
- `domain/entities/onboarding_session.go` (RecentTurns/WelcomeSentAt/CompletedAt + métodos `With*`)
- `infrastructure/repositories/postgres/onboarding_session_repository.go` (JSON + drift no `Find`)
- `application/usecases/complete_onboarding_session.go` (completed_at + limpa turns)
- `application/usecases/save_onboarding_card.go` (+ DTO input: `ClosingDay`)
- `application/binding/` (expor novos usecases ao agente)

Novos — `internal/onboarding`:
- `domain/valueobjects/objective_profile.go` (`ObjectiveProfile`, `ParseObjectiveProfile`,
  `classifyByKeyword`, `SplitTemplate` → basis points)
- `application/usecases/suggest_budget_split.go` (delega cents ao binding de budgets)
- `application/usecases/append_onboarding_turn.go`, `load_onboarding_turns.go`, `mark_welcome_sent.go`
- `application/binding/` (seam `BudgetAllocator` → consome `internal/budgets`)

Novos/Modificados — `internal/budgets` (dono da distribuição):
- `application/usecases/suggest_allocation.go` (novo — encapsula `AllocationDistributor`) + binding
- `module.go` (expor `SuggestAllocation`)

Modificados — `internal/card` (contrato closing_day-only): `application/dtos/input/create_card.go`
(`DueDay` opcional + `Validate()`), `domain/valueobjects/billing_cycle.go` (deriva due quando ausente),
`application/usecases/create_card.go`. `internal/transactions`: sem mudança. Persistência nos donos.

Modificados — `internal/agent` (ponte; sem domínio de onboarding):
- `application/usecases/run_onboarding_turn.go` (remove `agent_sessions` e `buildAutoSplits`; usa
  binding `SuggestBudgetSplit`/`OnboardingHistoryGateway`; `emitWelcome`→`MarkWelcomeSent`)
- `application/usecases/onboarding_tool_catalog.go` (`closing_day`; enum `objective_profile`)
- `application/usecases/onboarding_scripts.go` (copy "dia de fechamento")
- `infrastructure/onboarding/onboarding_tool_dispatcher.go` (remove síntese inline; `closing_day`)
- `infrastructure/messaging/database/consumers/onboarding_bound_consumer.go` (GAP-1: erro→retry)
- `module.go` (wiring do novo consumer + adapters de binding; remoção de wmWriter do dispatcher)

Novos — `internal/agent` (adapters finos, sem regra):
- `infrastructure/.../onboarding_history_gateway.go` (impl. de `OnboardingHistoryGateway` via binding)
- `infrastructure/.../budget_split_suggester.go` (impl. de `BudgetSplitSuggester` via binding)
- `infrastructure/messaging/database/consumers/onboarding_completed_consumer.go` (síntese de WM)

Dependências (somente leitura): `internal/platform/outbox`, `internal/agent/domain/entities/working_memory.go`,
`internal/onboarding/application/usecases/get_onboarding_context.go`.

### Cobertura completa de Requisitos (RF-01..36)

Status: **P1** = Parte 1, já implementada (verificada no working tree); **V2** = escopo desta
techspec; **SCRIPT** = governado por copy/scripts; **ROUTE** = roteamento/prioridade existente.

| RF | Status | Onde / Decisão | Teste |
|---|---|---|---|
| RF-01 auto-start (3 mensagens) | P1 | `HandleActivation`+`startOnboarding` | e2e ativação |
| RF-02 não enviar reply FSM | P1 | `HandleActivation` (sem `startResult.Reply`) | e2e |
| RF-03 evento com peer_e164 | P1 | `SubscriptionBound` payload | integração subscription_bound |
| RF-04 consumer dispara saudação | P1 | `OnboardingBoundConsumer` | unit consumer |
| RF-05 retry se sessão ausente | V2 | GAP-1 (erro→retry) | unit consumer (InProgress=false) |
| RF-06 LLM sempre ativo | P1 | sem flag `OnboardingLLMEnabled` | configs test |
| RF-07 app não sobe sem modelo | P1 | `validateOnboarding` | configs test |
| RF-08 FSM como degradação | V2 | retry seguro (sem FSM completa) | unit erro de LLM preserva estado |
| RF-09 sem confirmações supérfluas | SCRIPT | scripts (contrato) | fidelidade de narração |
| RF-10 5 categorias em 1 msg | SCRIPT | `scriptWelcome` (bloco) | fidelidade |
| RF-11 indicador de progresso | SCRIPT | headers `🔵 Etapa x/4` | fidelidade |
| RF-12 cartão 1 msg (fechamento) | V2 | ADR-005 | unit card `closing_day`; e2e |
| RF-13 distribuição automática | V2 | ADR-004 (`SuggestBudgetSplit`) | unit perfis/split |
| RF-13a objetivo ambíguo→default | V2 | ADR-004 (fallback) | unit classify default |
| RF-14 nunca reiniciar distribuição | V2 | `financialPlanPhase` (ajuste incremental no onboarding) | unit ajuste preserva progresso |
| RF-15 ajuste em linguagem natural | V2 | fase plano financeiro (LLM parse → usecase) | unit/e2e ajuste |
| RF-16 resumo final enxuto | SCRIPT | copy (contrato) | fidelidade |
| RF-17 não encerrar → 1ª transação | SCRIPT/V2 | `scriptFirstTx` após resumo | e2e |
| RF-18 priorizar atrito/ativação | SCRIPT | princípio dos scripts | revisão |
| RF-19 estado em onboarding_sessions | V2 | ADR-001/006 | integração isolamento |
| RF-20 recent_turns bounded (3) | V2 | ADR-001 (DR-08) | unit append bounded |
| RF-21 não usar agent_sessions | V2 | ADR-001/006 | gate grep + integração |
| RF-22 welcome_sent_at/completed_at | V2 | ADR-001/002 | unit payload |
| RF-23 conclusão com pré-requisitos | V2 | ADR-002 | unit `CompleteOnboardingSession` |
| RF-24 write transacional | V2 | ADR-002 (`uow.Do`) | unit + integração |
| RF-25 evento após persistência | V2 | ADR-002 | unit publish ordering |
| RF-26 handoff por sinal determinístico | V2 | ADR-002/003 | e2e handoff |
| RF-27 mensagens só p/ onboarding | ROUTE | prioridade `IntentRouter` | e2e |
| RF-28 pós-conclusão não reabre | ROUTE | `IsActive` no roteamento | e2e |
| RF-29 saudação idempotente | V2 | welcome_sent_at + AgentDecision(event_id) | unit + integração |
| RF-30 retomada da fase | V2 | ADR-001 (estado persistido) | integração retomada |
| RF-31 drift explícito | V2 | ADR-002 (`Find`) | integração drift |
| RF-32 falha LLM sem corromper | V2 | retry seguro | unit erro não persiste |
| RF-33 agentes isolados | ROUTE/V2 | ADR-006 + roteamento | e2e + gate fronteira |
| RF-34 WM assíncrona | V2 | ADR-003 | unit consumer; integração |
| RF-35 limpar turns ao concluir | V2 | ADR-002 (`WithCompletion`) | unit |
| RF-36 off-topic→redireciona | V2/SCRIPT | system prompt + scripts | unit/e2e off-topic |

### Cobertura de Edge Cases (EB-01..14)

| EB | Tratamento | Decisão |
|---|---|---|
| EB-01 race saudação×sessão | consumer retorna erro→retry | GAP-1 |
| EB-02 reprocesso sem duplicar | welcome_sent_at + AgentDecision | RF-29 |
| EB-03 falha LLM no meio | retry seguro, estado preservado | RF-08/32 |
| EB-04 abandono/retomada | estado persistido em onboarding_sessions | ADR-001 |
| EB-05 off-topic | resposta breve + redireciona | RF-36 |
| EB-06 correção sem reiniciar | ajuste incremental | RF-14/15 |
| EB-07 "Não uso" (sem cartão) | lista vazia satisfaz pré-requisito | RF-12/23 |
| EB-08 conclusão sem 1ª transação | `ErrFirstTransactionRequired` | ADR-002 |
| EB-09 mensagem pós-conclusão | roteada ao agente principal | RF-26/28 |
| EB-10 drift state=active s/ completed_at | contador + warn | ADR-002 |
| EB-11 falha síntese WM | retry do outbox, não bloqueia | ADR-003 |
| EB-12 mensagens concorrentes | exclusividade do onboarding (InProgress) | RF-27/33 |
| EB-13 objetivo ambíguo | default OrganizeSpending | ADR-004 |
| EB-14 cartão dia inválido | re-pergunta da etapa | ADR-005 |
| EB-15 renda fora de R$500..R$1B | re-pergunta amigável (contrato onboarding) | GAP-V3 |
| EB-16 nickname > 32 | valida na fronteira + re-pergunta (contrato card) | GAP-V2 |

### Decisões → ADR

DR-01→conformidade (allowlist) · DR-02→ADR-003 · DR-03/RF-35→ADR-002 · DR-04/RF-36→V2 ·
DR-05→ADR-005 · DR-06/07→ADR-004 · DR-08→ADR-001 · Fronteira→ADR-006.
