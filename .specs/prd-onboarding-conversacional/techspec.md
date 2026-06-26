<!-- spec-hash-prd: cdca23b5b8c2a440473e9d0ed3ab4ea609af53edfd522c70aa6b0cb2dfa6b5b7 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Onboarding Conversacional

> PRD: `.specs/prd-onboarding-conversacional/prd.md` (spec-version 2) · Mapeamento verbatim: `.specs/prd-onboarding-conversacional/mapeamento-verbatim-onboarding.md` · Oficial: `docs/oficial/2026_06_24_mecontrola_oficial.md` (Cap. 07/08/10/11).
> Governança: `AGENTS.md`, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001), `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001), `.claude/rules/go-adapters.md` (R-ADAPTER-001), `.claude/rules/go-testing.md` (R-TESTING-001), `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001). Skills: `go-implementation`, `mastra`. DMMF (Wlaschin) — state-as-type, smart constructors, `Decide*` puro, pipeline `parse→validate→decide→persist→publish`.

## Resumo Executivo

O onboarding conversacional é remodelado para as **8 etapas oficiais** (Cap. 08) executadas como um **workflow durável** sobre o kernel genérico `internal/platform/workflow` (`Engine[S]`), com **suspend/resume por etapa** e `Run` auditável. O loop de fases atual (`run_onboarding_turn.go`, "Etapa X/4", passo `first_tx`, auto-sugestão de split) é **substituído por completo** (ADR-001). A posição no fluxo passa a ser um **tipo fechado** `OnboardingPhase` (8 constantes), eliminando a `string` livre (ADR-002, DMMF state-as-type). A conclusão ocorre na ETAPA 8 após a confirmação do Resumo, **sem exigir primeira transação** (remove `FirstTxRecorded` de `IsReadyToComplete`).

O `internal/agent` é o **consumidor** do kernel: cada etapa é um `Step[OnboardingState]` que invoca **bindings → use cases** do `internal/onboarding` (dono do estado durável e dos eventos) — fronteira fina `adapter → usecase`, sem regra de negócio/SQL no step (R-ADAPTER-001 / R-AGENT-WF-001.2). O LLM aparece somente na cadeia dedicada de onboarding (exceção sancionada de R-AGENT-WF-001.4): interpreta a entrada do usuário por etapa e gera a mensagem no tom oficial (Cap. 03–06); **nunca** dentro do kernel (R-WF-KERNEL-001.5). O cartão coleta **apenas o vencimento** e o **fechamento é derivado** por offset configurável (ADR-003). O gate "Está tudo certo?" (ETAPA 7) e o desvio de comandos diários reusam os **primitivos genéricos de suspend/resume + estado fechado** do kernel, espelhando o padrão `AwaitingApproval` (ADR-004). A propagação para `card`/`budgets`/`agent` mantém os **domain events idempotentes** já existentes.

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos (em `internal/agent`):**
- `OnboardingWorkflow` (`internal/agent/application/workflow/onboarding_workflow.go`) — monta a `Definition[OnboardingState]` com a `Sequence` das 8 etapas e o loop de cartões; orquestra `Engine[OnboardingState].Start/Resume`.
- `Step[OnboardingState]` por etapa (`onboarding_steps_*.go`) — welcome, objective, budget, cards (com self-loop), categories, values, summary (gate HITL), conclusion.
- `OnboardingState` (`internal/agent/application/workflow/onboarding_state.go`) — estado de **orquestração** (não-canônico): `Phase`, `Awaiting`, `Inbound`, `CardLoop`, `Values`, `Correction`, `MessageID`, `RepromptCount`, `SuspendedAt`. É o `S` opaco do kernel.
- `OnboardingInterpreter` adapter (cadeia LLM de onboarding) — parse por etapa + geração da mensagem no tom oficial. Reusa o `interpreter` LLM já existente do onboarding.
- Decisões puras `Decide*` (`internal/agent/application/workflow/onboarding_decide.go`) — classificação de entrada (resposta da etapa vs comando diário vs dúvida), avanço/suspensão, validação da soma de valores, semântica de confirmação/correção. Sem IO.

**Modificados:**
- `OnboardingAgent` (`internal/agent/application/services/onboarding_agent.go`) — passa a resolver o run suspenso via `Engine.Resume` **antes** do `ParseInbound` do agente diário; sem `case intent.Kind` novo.
- `internal/onboarding` use cases/VOs/eventos — `OnboardingPhase` tipado; coleta de **vencimento**; `IsReadyToComplete` sem `FirstTxRecorded`; remoção de `MarkFirstTransactionRecorded`/`ErrOnboardingFirstTransactionRequired` do caminho de conclusão.
- `internal/card` — `CreateCard` aceita `DueDay` obrigatório e `ClosingDay` derivado no seam (ADR-003).

**Removidos (legado morto — ADR-001):** `run_onboarding_turn.go` (loop de fases), `OnbPhaseFirstTx`/`firstTxPhase`, `buildAutoSplitPreview`/`suggest_budget_split` no caminho oficial, headers "Etapa X/4", schema `onboarding_first_tx`.

**Fluxo de dados (resumo):**
```
WhatsApp inbound → OnboardingAgent.Handle
  → (há run de onboarding suspenso para o user?)
     sim → Engine.Resume(def, key=user_id, resume={"Inbound":text,"MessageID":id})
     não, e sessão in_progress → Engine.Start(def, key=user_id, initial)
  → Step da etapa atual:
     parse LLM (entrada) → Decide* (classifica/valida)
       → binding → usecase onboarding (persiste + emite evento)
       → gera mensagem oficial (LLM) → StepOutput{Suspended, Prompt} | {Completed}
  → resposta enviada via WhatsApp gateway
Eventos: onboarding.card_registered → card | onboarding.splits_calculated → budgets
         onboarding.completed → agent (working memory) / identity
```

## Design de Implementação

### Interfaces Chave

**Estado do kernel (o `S`), tipos fechados (DMMF state-as-type):**
```go
// internal/onboarding/domain/valueobjects/onboarding_phase.go (substitui Phase string)
type OnboardingPhase int
const (
    PhaseWelcome OnboardingPhase = iota + 1 // ETAPA 1
    PhaseObjective                          // ETAPA 2
    PhaseBudget                             // ETAPA 3 (renda)
    PhaseCards                              // ETAPA 4
    PhaseCategories                         // ETAPA 5 (apresentação)
    PhaseValues                             // ETAPA 6 (valores por categoria)
    PhaseSummary                            // ETAPA 7 (resumo + gate)
    PhaseConclusion                         // ETAPA 8
)
func (p OnboardingPhase) String() string    // "welcome"... persistência TEXT
func ParseOnboardingPhase(s string) (OnboardingPhase, error)
func (p OnboardingPhase) IsValid() bool
```
```go
// internal/agent/application/workflow/onboarding_state.go
type OnboardingAwaiting int
const (
    AwaitingNone OnboardingAwaiting = iota + 1
    AwaitingText                        // resposta livre da etapa
    AwaitingConfirm                     // gate "Está tudo certo?" (ETAPA 7)
)
type CorrectionTarget int // none|objective|budget|cards|values (campo do resumo a corrigir)

type OnboardingState struct {
    Phase         valueobjects.OnboardingPhase
    Awaiting      OnboardingAwaiting
    Inbound       string                   // texto do usuário aplicado no resume (merge-patch)
    MessageID     string
    CardLoop      int                      // nº de cartões já cadastrados nesta etapa
    Values        map[string]int64         // root_slug -> cents coletados (ETAPA 6)
    Correction    CorrectionTarget
    RepromptCount int
    SuspendedAt   time.Time
}
```

**Step de etapa (consumidor do kernel; LLM/binding ficam no agent, nunca no kernel):**
```go
// internal/agent/application/workflow/onboarding_steps_objective.go (padrão de todas as etapas)
func newObjectiveStep(d objectiveDeps) workflow.Step[OnboardingState] {
    return workflow.NewStepFunc("onboarding.objective", func(ctx context.Context, s OnboardingState) (workflow.StepOutput[OnboardingState], error) {
        if s.Inbound == "" { // primeira entrada na etapa: emite a pergunta e suspende
            return suspend(s, PhaseObjective, AwaitingText, d.render(ctx, PhaseObjective, s))
        }
        parsed, err := d.interpret(ctx, PhaseObjective, s.Inbound) // LLM parse (cadeia onboarding)
        if err != nil { return suspend(s, PhaseObjective, AwaitingText, d.retry(PhaseObjective)) }
        switch DecideObjective(parsed) {                            // Decide* puro
        case OutcomeDeferred: return suspend(s, PhaseObjective, AwaitingText, d.redirectDaily(PhaseObjective))
        case OutcomeClarify:  return suspend(s, PhaseObjective, AwaitingText, parsed.Reply)
        case OutcomeAdvance:
            if err := d.save.Objective(ctx, s.userID(), parsed.Objective); err != nil { return fail(err) } // binding→usecase
            return advance(s, PhaseBudget) // StepStatusCompleted; próxima etapa via Sequence
        }
    })
}
```

**Workflow (montagem; reuso de combinadores do kernel):**
```go
// internal/agent/application/workflow/onboarding_workflow.go
func BuildOnboardingDefinition(d onboardingDeps) workflow.Definition[OnboardingState] {
    return workflow.Definition[OnboardingState]{
        ID: "onboarding", Durable: true, MaxAttempts: 1,
        Root: workflow.Sequence(
            newWelcomeStep(d), newObjectiveStep(d), newBudgetStep(d),
            newCardsStep(d),       // self-loop interno: pergunta "outro cartão?" até negação
            newCategoriesStep(d),  // ETAPA 5 (apresentação + "Faz sentido?")
            newValuesStep(d),      // ETAPA 6 (5 valores, um a um)
            newSummaryStep(d),     // ETAPA 7 (gate AwaitingConfirm + correção guiada por LLM)
            newConclusionStep(d),  // ETAPA 8 (sem 1ª transação)
        ),
    }
}
```

**Resolução inbound (resume antes do parse — espelha o padrão de categoria/HITL):**
```go
// internal/agent/application/services/onboarding_agent.go
func (a *OnboardingAgent) Handle(ctx context.Context, userID uuid.UUID, channel, peer, text, messageID string) (RouteResult, bool) {
    snap, ok := a.onboarding.Snapshot(ctx, userID) // GetOnboardingContext
    if !ok || snap.Completed() { return RouteResult{}, false } // não está em onboarding → agente diário
    resume, _ := json.Marshal(map[string]any{"Inbound": text, "MessageID": messageID})
    res, err := a.engine.Resume(ctx, a.def, userID.String(), resume) // se não há run, Start idempotente
    ...
    return RouteResult{Reply: res.Suspend.Prompt /*ou conclusão*/, Kind: intent.KindConfigureBudget, Outcome: tools.OutcomeRouted}, true
}
```

**Cartão — derivação de fechamento (ADR-003), `Decide*` puro no domínio onboarding:**
```go
// internal/onboarding/domain/services/card_closing.go
func DeriveClosingDay(dueDay, offsetDays int) int { // wrap 1..31; sem IO
    d := ((dueDay-1-offsetDays)%31 + 31) % 31
    return d + 1
}
```
- `SaveOnboardingCardInput` passa a coletar `Nickname` + `DueDay` (1–31); `ClosingDay` **não** é coletado.
- `CardRegistered` carrega `DueDay` (coletado) e `ClosingDay` (derivado por `DeriveClosingDay(DueDay, offset)`); offset de config (`AGENT_ONBOARDING_CARD_CLOSING_OFFSET_DAYS`, default 10, documentado).
- `OnboardingCardConsumer` (módulo `card`) chama `card.CreateCard` com `DueDay` (obrigatório) e `ClosingDay` (derivado), mantendo a competência (Cap. 10) funcional. `card.CreateCard.Validate` passa a exigir `DueDay` quando criado por este caminho (ver Riscos).

### Modelos de Dados

- `onboarding_sessions.payload.Phase`: `string` → persistência de `OnboardingPhase` via `String()`/`Parse`. **Migração (ADR-002):** ao carregar sessão `in_progress` com `Phase` ausente/desconhecido para o novo enum, a sessão é **resetada** para `PhaseWelcome` (reaproveita `StartBudgetConfiguration` outcome `reset`); dados parciais antigos descartados.
- `onboarding_sessions.payload`: remove dependência de `FirstTxRecorded` para conclusão; campo pode permanecer para auditoria, mas não entra em `IsReadyToComplete`.
- Kernel `workflow_runs` (existente): hospeda o `Snapshot` do onboarding (`correlation_key = user_id`, `workflow = "onboarding"`), fonte única de verdade do **resume** (R-WF-KERNEL-001.7). Dados canônicos (objetivo/renda/cartões/splits) permanecem em `onboarding_sessions` via use cases.
- `IsReadyToComplete()` novo (puro): `Objective != "" && IncomeCents > 0 && len(CustomSplit) == 5`.

### Endpoints de API

Nenhum endpoint HTTP novo. A entrada é o webhook WhatsApp já existente (`internal/platform/whatsapp`); a saída é o gateway de envio. Não se cria porta HTTP como seam interno (R-ADAPTER-001 / RT-07).

## Pontos de Integração

- **`internal/onboarding`** (binding → use cases): `SaveOnboardingObjective`, `SaveOnboardingIncome`, `SaveOnboardingCard` (vencimento), `SaveOnboardingBudgetSplits`, `SetOnboardingPhase` (mirror tipado), `AppendOnboardingTurn`/`LoadOnboardingTurns`, `GetOnboardingContext`, `CompleteOnboardingSession`, `MarkWelcomeSent`.
- **`internal/card`** (via evento `onboarding.card_registered`): cria cartão com `DueDay` + `ClosingDay` derivado.
- **`internal/budgets`** (via evento `onboarding.splits_calculated`): cria/ativa orçamento.
- **`internal/agent`** (via evento `onboarding.completed`): consolida working memory.
- **Idempotência:** todos os consumidores idempotentes por `event_id` (RF-28, AGENTS.md Outbox). Inbound idempotente por `messageID` (RF-03).

## Abordagem de Testes

### Testes Unitários
- **`Decide*` puros (sem mock):** `DecideObjective/Budget/Cards/Values/Summary`, classificação resposta-da-etapa × comando-diário × dúvida, validação soma(valores)==renda, semântica confirmar/cancelar/corrigir/reprompt/TTL, `DeriveClosingDay` (wrap 1..31, offsets, bordas).
- **Steps (testify/suite, whitebox, `fake.NewProvider()`, mocks por IIFE — R-TESTING-001):** cada etapa: primeira entrada suspende com prompt; entrada válida persiste (mock do binding) e avança; entrada inválida re-suspende; comando diário → re-suspende sem persistir; mensagens via interpreter mockado.
- **Use cases onboarding alterados:** `SaveOnboardingCard` (vencimento), `complete_onboarding_session` (sem `FirstTxRecorded`), `OnboardingPhase` parse/serialização, migração-reset.
- **`card.CreateCard`:** `DueDay` obrigatório + `ClosingDay` derivado; validações de borda.

### Testes de Integração
> Critérios: (a) fronteiras de IO críticas (Postgres: `onboarding_sessions`, `workflow_runs`, outbox) onde mocks não garantem correção — **sim**; (b) risco de regressão na migração/substituição — **sim**. ⇒ **Integration tests recomendados** (testcontainers-go, `//go:build integration`).
- Resume durável: `Start` → suspende ETAPA 2 → `Resume` aplica merge-patch → avança; reinício de processo entre turnos preserva estado (snapshot).
- Propagação por evento: `splits_calculated`→budget ativo; `card_registered`→cartão com vencimento+fechamento derivado; `completed`→working memory.
- Migração-reset de sessão `in_progress` com `Phase` legado.

### Testes E2E
- Jornada completa das 8 etapas (feliz) via fluxo do agent: boas-vindas→…→conclusão, validando ordem, persistência e `onboarding.completed`.
- Casos de borda: correção no resumo (ETAPA 7), "não uso cartão", comando diário no meio (redirecionamento sem registro), retomada após interrupção.
- Reaproveitar a suíte e2e existente de onboarding (`internal/onboarding/e2e`).

## Sequenciamento de Desenvolvimento

### Ordem de Build
1. **Tipos fechados + domínio puro** (ADR-002/003): `OnboardingPhase`, `OnboardingAwaiting`, `CorrectionTarget`, `DeriveClosingDay`, `Decide*`. Sem IO; 100% testável. (Habilita o resto.)
2. **Use cases/eventos onboarding** alterados: vencimento, `IsReadyToComplete` sem `FirstTxRecorded`, migração-reset, `SetOnboardingPhase` tipado.
3. **`card.CreateCard`** + consumidor `onboarding.card_registered` (vencimento + fechamento derivado).
4. **Steps + Workflow** (`OnboardingWorkflow`, `Engine[OnboardingState]`), interpreter de onboarding, gate HITL do resumo.
5. **Wiring** `OnboardingAgent` (resume-antes-do-parse) + remoção do legado (`run_onboarding_turn.go`, `first_tx`, auto-sugestão, headers "X/4").
6. **Job de abandono** + métricas de funil.
7. **Integração + E2E**.

### Dependências Técnicas
- Kernel `internal/platform/workflow` (existente) e seu adapter Postgres (`workflow_runs`).
- Tabela `onboarding_sessions` (existente; migração de `Phase`).
- Cadeia LLM de onboarding (existente; modelo por config).

## Monitoramento e Observabilidade

- **Métricas (cardinalidade controlada — sem `user_id`/`correlation_key`/`category_id`; R-WF-KERNEL-001.4 / R-AGENT-WF-001.5):**
  - `onboarding_step_total{step,outcome}` (outcome ∈ advance|clarify|deferred|retry|confirm|cancel).
  - `onboarding_completed_total`, `onboarding_run_duration_seconds`.
  - `onboarding_step_abandoned_total{step}` (RF-30).
  - Herdadas do kernel: `workflow_runs_total`, `workflow_step_duration_seconds`, `workflow_suspend_total`, `workflow_resume_total`.
- **Run auditável (R-AGENT-WF-001.5):** `thread_id`, `run_id`, `workflow=onboarding`, `step`, `status` (`RunStatus`), `duration_ms`, `error`.
- **Logs:** transições de etapa, suspensões, reset de migração, derivação de fechamento (offset aplicado), abandono.
- **Job de abandono (QT-04):** worker periódico varre `workflow_runs` (workflow=onboarding, status=suspended, `updated_at` < now−TTL), emite `onboarding.step_abandoned{step}` e marca o run conforme política (mantém suspenso para retomada ou falha após TTL longo). TTL configurável.

## Considerações Técnicas

### Decisões Chave (ADRs)
- **ADR-001** — Onboarding como workflow durável no kernel (8 etapas, suspend/resume, substituição completa do loop atual, conclusão sem 1ª transação). `adr-001-onboarding-no-kernel-workflow.md`. (QT-01, RF-19/22/23/29)
- **ADR-002** — `OnboardingPhase` tipo fechado + migração por reset das sessões em andamento. `adr-002-onboarding-phase-tipo-fechado-e-migracao.md`. (QT-03, M-8)
- **ADR-003** — Cartão: coleta só vencimento; fechamento derivado por offset configurável. `adr-003-cartao-so-vencimento-fechamento-derivado.md`. (QT-08, RF-08)
- **ADR-004** — Gate HITL do Resumo e desvio de comando diário reusando primitivos do kernel. `adr-004-gate-hitl-resumo-e-desvio-diario.md`. (QT-02/06, RF-17/18/25)

**Decisões internas (sem ADR, registradas aqui):**
- **QT-05 (cálculo de %):** permanece no domínio do onboarding (basis points em `budget_allocation`/split events) — `Decide*` puro; sem duplicar em `budgets`. O resumo (ETAPA 7) exibe valor + percentual computado do domínio.
- **QT-07 (memória):** `recent_turns` limpos na conclusão (alinha decisão prévia do projeto); working memory consolidada em markdown (objetivo, renda, cartões, distribuição) via consumer de `onboarding.completed`.

### Riscos Conhecidos
- **R1 — Offset de fechamento uniforme (ADR-003):** `DeriveClosingDay` assume um offset único; cartões reais variam. Mitigação: offset configurável + documentado; competência aproximada aceita no MVP; reavaliar se houver evidência de erro de fatura. Limitação registrada no PRD (RF-08) e ADR-003.
- **R2 — Mudança de contrato `card.CreateCard`:** tornar `DueDay` obrigatório por este caminho pode impactar callers existentes (HTTP create, agent card creator). Mitigação: `DueDay` permanece `*int` no DTO público; a obrigatoriedade do vencimento é validada **no seam do onboarding** (consumer), não quebrando o contrato HTTP atual; `ClosingDay` continua aceito quando informado. Detalhar invariante em ADR-003.
- **R3 — Dupla fonte de estado (kernel snapshot × onboarding_sessions):** risco de drift entre posição de fluxo e dados. Mitigação: snapshot do kernel é fonte única do **resume**; `onboarding_sessions` é fonte única dos **dados**; `Phase` em onboarding é mirror derivado do estado do workflow (escrito pelo step), nunca lido para decidir resume.
- **R4 — Migração-reset (ADR-002):** usuários a meio do onboarding antigo reiniciam. Mitigação: aceitável pré-escala (PRD QT-03); logar contagem de resets; janela de deploy de baixa atividade.
- **R5 — Soma de valores ≠ renda (ETAPA 6):** invariante `budget_allocation` exige soma == renda. Mitigação: `Decide*` valida ao fechar as 5 categorias; em mismatch, re-pergunta (qual ajustar) antes do resumo — sem auto-completar.

### Conformidade com Padrões
- **R-AGENT-WF-001:** roteamento via workflow/registry; sem novo `case intent.Kind`; Tool/Step finos; `Run` auditável; LLM só na cadeia de onboarding (exceção sancionada .4); resume antes do parse (.7-A); WorkingMemory exclusiva do agent (.8).
- **R-WF-KERNEL-001:** kernel permanece genérico — steps/LLM/binding vivem em `internal/agent`; nada de domínio/LLM/SQL no kernel; estados fechados; resume por merge-patch (.7).
- **R-ADAPTER-001:** steps e consumers finos `adapter → usecase`; zero comentários em `.go`; sem SQL direto.
- **R-DTO-VALIDATE-001:** inputs com `Validate()` (`errors.Join`, campo nomeado), validação após abrir span.
- **R-TESTING-001:** suites whitebox, `fake.NewProvider()`, mocks por IIFE.
- **DMMF:** state-as-type (`OnboardingPhase`/`OnboardingAwaiting`/`OperationKind`), smart constructors (VOs de objetivo/renda/cartão/alocação), `Decide*` puro, pipeline linear; anti-padrões proibidos (Result/Either custom, currying, DSL, mônada) não introduzidos.

### Arquivos Relevantes e Dependentes
- Novos: `internal/agent/application/workflow/onboarding_workflow.go`, `onboarding_state.go`, `onboarding_decide.go`, `onboarding_steps_*.go`; `internal/onboarding/domain/valueobjects/onboarding_phase.go`; `internal/onboarding/domain/services/card_closing.go`.
- Modificados: `internal/agent/application/services/onboarding_agent.go`, `.../module.go` (wiring/remoção de legado); `internal/onboarding/.../save_onboarding_card*.go`, `onboarding_session.go` (`IsReadyToComplete`, `Phase`), `complete_onboarding_session.go`, `onboarding_session_events.go`; `internal/card/application/dtos/input/create_card.go` + consumer `onboarding_card_consumer.go`.
- Removidos: `internal/agent/application/usecases/run_onboarding_turn.go`, `onboarding_scripts.go` (partes "X/4"/auto-split), `onboarding_structured_schema.go` (`onboarding_first_tx`), `mark_first_transaction_recorded.go` (caminho de conclusão).

## Mapeamento Requisito → Decisão → Teste

| RF | Decisão de design | Teste |
|---|---|---|
| RF-01/02/03 | `OnboardingAgent` resolve thread/run via runtime; resume idempotente por `messageID` | unit (idempotência) + integração (resume) |
| RF-04 | `newWelcomeStep` + handshake (suspende `AwaitingText`, aguarda "Sim") | unit step + e2e |
| RF-05 | `newObjectiveStep` + `SaveOnboardingObjective` | unit + e2e |
| RF-06/07 | `newBudgetStep` + `SaveOnboardingIncome`; clarify sem avançar | unit (Decide) + e2e |
| RF-08/09/10 | `newCardsStep` (só vencimento, self-loop, "não uso"); `DeriveClosingDay`; evento | unit (`DeriveClosingDay`, loop) + integração (card) |
| RF-11/12 | `newCategoriesStep` (apresentação + "Faz sentido?") | unit step + e2e |
| RF-13/14/15 | `newValuesStep` (5 valores, um a um, sem auto-sugestão); % no domínio; `splits_calculated` | unit (soma==renda) + integração (budget) |
| RF-16/17/18 | `newSummaryStep` (valor+%, `AwaitingConfirm`, correção LLM) | unit (Decide confirm/correct) + e2e |
| RF-19/20/21 | `newConclusionStep`; `IsReadyToComplete` sem `FirstTxRecorded`; `completed`→WM | unit + integração (WM) |
| RF-22/23 | `OnboardingPhase` fechado; resume por snapshot/merge-patch | unit (parse) + integração (resume) |
| RF-24 | `recent_turns` (load/append; limpeza na conclusão) | unit |
| RF-25 | `Decide*` → `OutcomeDeferred` (redireciona, não registra) | unit + e2e |
| RF-26 | clarify por etapa no tom oficial | unit |
| RF-27/28 | seam por evento idempotente + interface no consumidor | integração |
| RF-29 | `Run` auditável (campos mínimos) | integração/observabilidade |
| RF-30 | job de abandono + `onboarding_step_abandoned_total{step}` | unit (job) + integração |
