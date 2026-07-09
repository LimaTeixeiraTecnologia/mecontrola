<!-- spec-hash-prd: 85c7c8eff5955982193ee5b5de602ae2378b1445432b491270e270741c714105 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Cadastro Conversacional de Cartão

## Resumo Executivo

O cadastro de cartão por conversa é adicionado ao consumidor agentivo `internal/agents` como uma nova
**tool fina** `create_card` (`tool.NewTool[I,O]`) que **não escreve diretamente**: ela faz slot-filling
conversacional (RF-05/RF-06), consulta o reconhecimento do banco (ADR-002) e, quando os dados estão
completos, inicia um **workflow de confirmação dedicado** `card-create-confirm` sobre o kernel
`internal/platform/workflow` (`Engine[CardCreateState]`). O estado de espera é um tipo fechado
persistido no `Snapshot` do kernel antes de perguntar, retomado por merge-patch antes do parse, com TTL
de 15 min, semântica sim/não/ambíguo e limpeza determinística (ADR-001). Na confirmação afirmativa, a
escrita é executada pelo `IdempotentWriter` dos agents (`operation="create_card"`), envolvendo
`CardManager.CreateCard` — o que ativa idempotência por `wamid` (RF-14) e a métrica
`agents_write_total{operation="create_card",outcome}` (RF-16) num único mecanismo (ADR-003).

O guardrail anti-alucinação (RF-13) é arquitetural: a tool está sempre registrada; a criação só ocorre
no step durável; a pergunta de confirmação e a mensagem final de sucesso/falha são texto determinístico
do workflow (surfaçado pelo continuer), não texto livre do LLM; e o harness real-LLM ≥ 0.90 + o teste
determinístico de regressão do incidente (RF-22) validam o contrato. O caminho de derivação do dia de
fechamento do módulo `internal/card` permanece inalterado para o onboarding (RF-09); a mudança é
aditiva (dia de fechamento explícito opcional para bancos não reconhecidos, ADR-002).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Novos (consumidor `internal/agents`):

- **Tool `create_card`** (`application/tools/create_card.go`, novo) — adapter fino; slot-filling,
  clarify de fechamento (banco não reconhecido), início do workflow de confirmação. Sem regra de
  negócio, SQL ou branching de domínio (R-ADAPTER-001.2 / R-AGENT-WF-001.2).
- **Workflow `card-create-confirm`** (`application/workflows/card_create_confirm_workflow.go`, novo) —
  `Definition[CardCreateState]`, step eval com suspend/resume, execução idempotente na resposta "sim".
- **Estado `CardCreateState` + enums** (`application/workflows/card_create_state.go`, novo) — tipo
  fechado (state-as-type). Reusa `AwaitingKind` (confirm_state.go); adiciona `CardCreateStatus`.
- **Decisão pura `DecideCardCreateConfirmation`** (`application/workflows/card_create_decisions.go`,
  novo) — pura, sem IO, testável sem mock (DMMF Decide*).
- **Continuer `CardCreateConfirmContinuer`** (`application/usecases/card_create_confirm_continuer.go`,
  novo) — abre/fecha `Run` auditável, resume por merge-patch, mapeia outcome→mensagem/métrica.
- **Reaper** — `workflow.NewStaleSuspendedReaper(store, CardCreateConfirmWorkflowID, 15m, batch, o11y)`
  fiado num job análogo ao `ConfirmReaperJob`.

Modificados:

- **`interfaces.NewCard`** (`application/interfaces/types.go:122`) — + `ClosingDay int`,
  `ClosingDayProvided bool`.
- **`interfaces.CardManager`** (`application/interfaces/card_manager.go`) — + `BankRecognized(ctx,
  bank) (bool, error)`.
- **`card_manager_adapter`** (`infrastructure/binding/card_manager_adapter.go:58`) — mapear novos
  campos em `cardinput.CreateCard`; implementar `BankRecognized`.
- **`mecontrola_agent.go`** (instruções) — registrar capacidade + proibir afirmar cadastro sem tool call.
- **`module.go`** (`buildFinancialTools` + composition root) — construir engine/def/continuer/reaper e
  registrar a tool com `agent.WithWriteToolSet`.
- **`WhatsAppInboundConsumer`** — inserir `tryContinueCardCreate` no resume chain antes do `ParseInbound`.

Modificados (módulo `internal/card`, aditivo — ADR-002):

- **`input.CreateCard`** (`application/dtos/input/create_card.go`) — + `ClosingDay int`,
  `ClosingDayProvided bool`; `Validate()` valida `ClosingDay` 1..31 só quando provido.
- **`CreateCard.Execute`** (`application/usecases/create_card.go:87-101`) — branch: `ClosingDayProvided`
  → `NewBillingCycle(in.ClosingDay, in.DueDay)`; senão → derivação atual (inalterada).
- **Leitura de reconhecimento** — `IsBankRecognized(ctx, code) (bool, error)`
  (`SELECT EXISTS(...) FROM mecontrola.banks WHERE code = $1`), read-only, aditiva. `DaysBeforeDue`
  mantém assinatura e fallback (onboarding intocado — RF-09).

### Fluxo de Dados

```
WhatsApp inbound "cadastrar cartão Nu, Nubank, vencimento 10"
 -> WhatsAppInboundConsumer.Handle
    -> tryContinuePendingEntry        (status 0 -> não é resume)
    -> tryContinueDestructive         (status 0 -> não é resume)
    -> tryContinueCardCreate  [NOVO]  (status 0 -> não é resume)
    -> tryResolveOnboarding           (status 0)
    -> handleAgentInbound (ParseInbound + LLM loop)
       -> tool create_card(exec)
          -> RuntimeFrom(ctx) -> agent.InboundRequest{ResourceID, MessageID}
          -> slots completos? banco reconhecido?
             - falta slot / falta closing (banco não reconhecido) -> ToolOutcomeClarify (pergunta)
             - completo -> engine.Start(card-create-confirm, key=CardCreateKey(resourceID), state)
                           -> StepStatusSuspended (persist Snapshot) -> retorna confirmationPrompt

WhatsApp inbound "sim"
 -> tryContinueCardCreate [NOVO]
    -> CardCreateConfirmContinuer.Continue(userID, peer, "sim", wamid)
       -> openRun(RunStatusRunning)
       -> engine.Resume(def, key, merge-patch {"resumeText":"sim","incomingMessageId":wamid})
          -> DecideCardCreateConfirmation -> accept
             -> idem.Execute(userID, state.MessageID, 0, "create_card", "card", writeFn)
                -> CardManager.CreateCard -> card.usecase.create (deriva ou usa closing explícito)
             -> StepStatusCompleted (success text determinístico)
       -> closeRun(RunStatusSucceeded|Failed, err) -> reply determinístico
```

## Design de Implementação

### Interfaces Chave

Estado de espera fechado (state-as-type, R-AGENT-WF-001.3):

```go
type CardCreateStatus int

const (
    CardCreateStatusActive CardCreateStatus = iota + 1
    CardCreateStatusCompleted
    CardCreateStatusCancelled
    CardCreateStatusExpired
)

func (s CardCreateStatus) String() string { /* switch exaustivo */ }
func (s CardCreateStatus) IsValid() bool   { return s >= CardCreateStatusActive && s <= CardCreateStatusExpired }
func ParseCardCreateStatus(v string) (CardCreateStatus, error) { /* rejeita inválido */ }

type CardCreateState struct {
    Status             CardCreateStatus `json:"status"`
    Awaiting           AwaitingKind     `json:"awaiting"`
    UserID             uuid.UUID        `json:"userId"`
    Nickname           string           `json:"nickname"`
    Bank               string           `json:"bank"`
    DueDay             int              `json:"dueDay"`
    ClosingDay         int              `json:"closingDay"`
    ClosingDayProvided bool             `json:"closingDayProvided"`
    MessageID          string           `json:"messageId"`
    IncomingMessageID  string           `json:"incomingMessageId"`
    ProcessedMessageID string           `json:"processedMessageId"`
    ConfirmReprompt    int              `json:"confirmReprompt"`
    SuspendedAt        time.Time        `json:"suspendedAt"`
    ResumeText         string           `json:"resumeText"`
    ResponseText       string           `json:"responseText"`
    Expired            bool             `json:"expired"`
}
```

Decisão pura (DMMF Decide*, sem IO, sem `context.Context`):

```go
type CardConfirmAction int

const (
    CardConfirmAccept CardConfirmAction = iota + 1
    CardConfirmCancel
    CardConfirmReprompt
    CardConfirmExpire
    CardConfirmReplay
)

func DecideCardCreateConfirmation(state CardCreateState, msg PendingMessage, now time.Time) CardConfirmAction
```

Workflow builder e chave:

```go
const (
    CardCreateConfirmWorkflowID = "card-create-confirm"
    cardCreateConfirmTTL        = 15 * time.Minute
    cardCreateMaxReprompts      = 1
)

func CardCreateKey(resourceID string) string { return resourceID + ":card-create" }

func BuildCardCreateConfirmWorkflow(idem interfaces.IdempotentWriter, cards interfaces.CardManager) workflow.Definition[CardCreateState]
```

Tool (adapter fino):

```go
type CreateCardInput struct {
    Nickname   string `json:"nickname"`
    Bank       string `json:"bank"`
    DueDay     int    `json:"dueDay"`
    ClosingDay *int   `json:"closingDay,omitempty"`
}
```

Schema JSON da tool (input `Strict: false`, como `update_card.go:39`, por causa do campo opcional
`closingDay`) declara **range 1..31** em `dueDay` e `closingDay` via `minimum:1`/`maximum:31` —
validação declarativa de schema (permitida a adapters por R-AGENT-WF-001.2; **não** é regra de negócio
em código). Feedback rápido ao LLM/usuário antes de gastar a confirmação (RF-06/RF-10); a validação
**autoritativa e única** permanece nos smart constructors (`NewBillingCycle`) no momento do write — sem
duplicação de whitelist em Go (R-DTO-004).

```go

type CreateCardOutput struct {
    Outcome            string `json:"outcome"`            // needs_slot | needs_closing | needs_confirmation | pending_confirmation_exists
    ConfirmationPrompt string `json:"confirmationPrompt"`
    ClarifyPrompt      string `json:"clarifyPrompt"`
}

func BuildCreateCardTool(engine wf.Engine[workflows.CardCreateState], def wf.Definition[workflows.CardCreateState], cards interfaces.CardManager) tool.ToolHandle
```

Extensões de interface (consumidor):

```go
type NewCard struct {
    UserID             uuid.UUID
    Nickname           string
    Bank               string
    DueDay             int
    ClosingDay         int  // ADR-002
    ClosingDayProvided bool // ADR-002
}

type CardManager interface {
    CreateCard(ctx context.Context, in NewCard) (CardRef, error)
    BankRecognized(ctx context.Context, bank string) (bool, error) // ADR-002
    // ... métodos existentes
}
```

### Modelos de Dados

- **Sem novo schema.** Reusa `platform_runs`/`platform_workflow_snapshots`/`platform_workflow_steps`
  (kernel), `agents_write_ledger` (idempotência) e `mecontrola.cards` (índice único parcial
  `cards_user_nickname_active_uniq_idx`). O `CardCreateState` é serializado no `Snapshot.State`.
- **`input.CreateCard` (card):** + `ClosingDay int`, `ClosingDayProvided bool`. `Validate()`:
  quando `ClosingDayProvided`, exigir `ClosingDay` ∈ [1,31] (`errors.Join`, campo nomeado — R-DTO-001);
  demais validações inalteradas.

### Semântica de Confirmação (RF-03/RF-04)

`DecideCardCreateConfirmation` (pura):

- TTL expirado (`now - SuspendedAt > 15min`) → `CardConfirmExpire` → run concluído, `handled=false`,
  texto do usuário segue ao `ParseInbound`.
- `msg.MessageID == state.ProcessedMessageID` → `CardConfirmReplay` (dedup de reentrega da mesma msg).
- "sim/confirmar/confirma/ok/pode" → `CardConfirmAccept`.
- "não/nao/cancelar" → `CardConfirmCancel`.
- ambíguo com `ConfirmReprompt >= 1` → `CardConfirmCancel` (2ª ambiguidade cancela).
- ambíguo com `ConfirmReprompt == 0` → `CardConfirmReprompt` (re-pergunta uma vez, `ConfirmReprompt++`).

Após accept/cancel/expire → `StepStatusCompleted` e run concluído (`RunStatusSucceeded`); nunca
permanece suspenso (RF-21). Reaper purga runs suspensos além do TTL.

### Guardrail Anti-Alucinação (RF-13)

- A tool `create_card` está sempre registrada em `agent.WithWriteToolSet`; a criação só ocorre no step
  durável (nunca em texto do LLM).
- A tool retorna `ConfirmationPrompt`/`ClarifyPrompt` determinístico; as instruções do agente exigem
  relayar verbatim e proíbem afirmar "cadastrei/não consegui" sem tool call.
- A mensagem final de sucesso/falha vem do continuer (determinística), não do LLM.
- Sem `switch case intent.Kind` (R-AGENT-WF-001.1) — roteamento por registry de tools/workflow.

### Reconhecimento de Banco e Dia de Fechamento (RF-07/08/09, ADR-002)

- **Normalização unificada:** `IsBankRecognized(ctx, bank)` aplica exatamente a mesma normalização do
  smart constructor `NewBankCode` (NFD + hyphen-join, lowercase) antes de
  `SELECT EXISTS(...) FROM mecontrola.banks WHERE code = $1`. Assim reconhecimento e derivação enxergam
  o **mesmo `code`** — elimina a divergência apontada no risco do ADR-002. Fonte única de normalização,
  sem duplicação.
- Tool consulta `cards.BankRecognized(ctx, bank)`:
  - reconhecido → `ClosingDayProvided=false` (ignora `closingDay` do LLM — RF-07 determinístico);
  - não reconhecido + `closingDay` ausente → `Outcome="needs_closing"` + `ClarifyPrompt` (RF-08, slot
    conversacional, sem estado durável — RF-06);
  - não reconhecido + `closingDay` presente → `ClosingDay`+`ClosingDayProvided=true`.
- Usecase `card` faz branch por `ClosingDayProvided` (ADR-002); onboarding inalterado (RF-09).

### Idempotência, Auditoria e Métrica (RF-14/15/16, ADR-003)

- `executeCreateCard` chama `idem.Execute(ctx, state.UserID, state.MessageID, 0, "create_card",
  "card", writeFn)`; `writeFn` → `cards.CreateCard`, retorna `(cardID, false, err)`.
- Replay do mesmo `wamid` → `ToolOutcomeReplay`, sem segundo cartão (RF-14).
- Métrica `agents_write_total{operation="create_card",outcome}` automática (RF-16), sem `user_id`.
- `ErrNicknameConflict`/validações → outcome de domínio → mensagem acionável + run concluído (RF-12).
- Erro de infra → retry transiente (`IsTransient`) e, persistindo, `RunStatusFailed` com erro na coluna
  do run + log `card.create.failed` (RF-15).

### Exclusão Mútua e Ordem de Resume (RF-18)

Ordem determinística no `WhatsAppInboundConsumer`: `pending_entry → destructive_confirm → card-create →
onboarding → ParseInbound`. Como cada resume consome a mensagem quando há run suspenso, um novo
`card-create` só é iniciado (via tool no último passo) quando nenhum outro gate está suspenso →
exclusão mútua natural. `engine.Start` com `ErrRunAlreadyExists` retorna
`Outcome="pending_confirmation_exists"` (mesma proteção do `update_card.go:144`).

### Mapeamento Requisito → Decisão → Teste

| RF | Decisão | Teste |
|----|---------|-------|
| RF-01 | Tool `create_card` fina (ADR-001) | unit tool: delega, sem SQL/regra |
| RF-02 | Workflow dedicado + Snapshot antes de perguntar (ADR-001) | unit workflow: suspend persiste estado |
| RF-03 | `DecideCardCreateConfirmation` pura | table-test sem mock (sim/não/ambíguo×2) |
| RF-04 | TTL 15min avaliado no resume | unit: expiração cancela, `handled=false` |
| RF-05 | Slot-filling conversacional | harness real-LLM |
| RF-06 | Clarify sem estado durável | unit tool: `needs_slot`/`needs_closing` |
| RF-07 | Derivação autoritativa, tool força `provided=false` | unit tool + unit usecase |
| RF-08 | `BankRecognized` + closing explícito (ADR-002) | unit usecase provided-path; harness |
| RF-09 | Aditivo; onboarding inalterado | unit usecase derive-path; regressão onboarding |
| RF-10 | Reuso smart constructors | unit usecase erros nomeados |
| RF-11 | Sem restrição cruzada | unit: closing/due independentes 1..31 |
| RF-12 | `ErrNicknameConflict` → mensagem | integration: apelido duplicado |
| RF-13 | Guardrail arquitetural | teste regressão + harness |
| RF-14 | `IdempotentWriter` wamid (ADR-003) | integration replay "sim" |
| RF-15 | Run auditável, erro persistido | integration: falha infra popula run.error |
| RF-16 | `agents_write_total{operation=create_card}` | unit continuer/metric |
| RF-17 | `user_id` do principal (RuntimeFrom) | unit tool: ignora user do conteúdo |
| RF-18 | Ordem resume + `ErrRunAlreadyExists` | integration mutex |
| RF-19 | Registro tool + instruções | build/lint; harness tool-call |
| RF-20 | `NewCard`/input com closing (ADR-002) | unit binding/adapter |
| RF-21 | Run concluído, reaper | unit: nunca `RunStatusSuspended` após decisão |
| RF-22 | Harness ≥0.90 + regressão | suite scorer + teste determinístico |

## Pontos de Integração

- **OpenRouter (LLM)** — via `internal/platform/llm`, apenas no loop do agent e no scorer LLM-judged;
  nunca no kernel nem no `exec` da tool (R-AGENT-WF-001.4).
- **Postgres** — kernel (`platform_workflow_*`, `platform_runs`), `agents_write_ledger`,
  `mecontrola.cards`, `mecontrola.banks`. SQL novo apenas no read `IsBankRecognized`
  (`infrastructure/postgres` do módulo card).

## Abordagem de Testes

### Testes Unitários

- `DecideCardCreateConfirmation` — table-test **sem mock** (pura): accept/cancel/reprompt→cancel/
  expire/replay; ida-e-volta `String()`↔`Parse` dos enums e erro no valor inválido.
- Tool `create_card` — suite testify (R-TESTING-001, whitebox, `fake.NewProvider()`, mocks do
  `.mockery.yml`): slots incompletos→`needs_slot`; banco não reconhecido sem closing→`needs_closing`;
  banco reconhecido ignora closing do LLM; completo→`engine.Start` chamado, `ErrRunAlreadyExists`→
  `pending_confirmation_exists`; identidade do `RuntimeFrom` (RF-17).
- Usecase `card.CreateCard` — provided-path (cycle direto) vs derive-path (inalterado); banco
  reconhecido ignora closing; validações nomeadas (RF-10/RF-11); regressão do onboarding.
- Continuer — abre/fecha Run; outcome→métrica; mapeamento domínio vs infra.

### Testes de Integração

Critérios atendidos: fronteiras de IO críticas (Postgres, kernel snapshot, idempotência) onde mock não
garante correção; incidente real com falha silenciosa. **Recomendado**, com `//go:build integration` e
testcontainers (padrão já usado em `onboarding_workflow_integration_test.go`).

- Ciclo completo suspend→resume "sim" cria cartão (banco reconhecido e não reconhecido).
- Replay de "sim" (mesmo wamid) não cria segundo cartão (RF-14).
- Apelido duplicado → `ErrNicknameConflict` → mensagem, sem duplicata (RF-12).
- Falha de infra → `run.error` preenchido, nunca silenciosa (RF-15).
- Exclusão mútua: `card-create` não inicia com outro gate suspenso (RF-18).
- Confirmação expirada por TTL → cancela, texto segue ao `ParseInbound` (RF-04).

### Testes E2E

- Harness real-LLM (`RUN_REAL_LLM=1` com `.env` OPENROUTER_*) dos cenários Gherkin do PRD, com gate
  estatístico ≥ 0.90 (RF-22), no padrão `pending_entry_harness_test.go`.
- Teste determinístico de regressão do incidente: pedido de cadastro nunca responde sucesso/falha sem
  tool call; falha sempre com erro persistido (RF-13/RF-15).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Módulo card (aditivo, ADR-002):** `input.CreateCard` + `Validate`; branch no `Execute`;
   `IsBankRecognized` (read + repo). Isola a mudança de menor risco e desbloqueia o binding.
2. **Interfaces agents:** `NewCard` + `CardManager.BankRecognized`; `card_manager_adapter`
   (mapear campos + implementar `BankRecognized`); atualizar `.mockery.yml` se necessário e `task mocks`.
3. **Estado + decisão pura:** `card_create_state.go` (enums fechados) + `card_create_decisions.go`
   (`DecideCardCreateConfirmation`) + testes puros.
4. **Workflow + execução idempotente:** `card_create_confirm_workflow.go`
   (`BuildCardCreateConfirmWorkflow`, `executeCreateCard` via `IdempotentWriter`).
5. **Continuer + reaper:** `card_create_confirm_continuer.go` + job de reaper.
6. **Tool `create_card`** + registro em `buildFinancialTools`/`WithWriteToolSet`.
7. **Wiring:** `module.go` (engine/def/continuer/reaper) + `WhatsAppInboundConsumer.tryContinueCardCreate`.
8. **Instruções do agente** (`mecontrola_agent.go`).
9. **Testes de integração + harness + regressão.**

### Dependências Técnicas

- Kernel `workflow.Engine`/`Store` Postgres (existentes).
- `IdempotentWriter` dos agents (existente, `usecases.NewIdempotentWrite`).
- Tabela `mecontrola.banks` populada (existente).

## Monitoramento e Observabilidade

- **Métricas:** `agents_write_total{operation="create_card",outcome}` (RF-16);
  `workflow_suspend_total`/`workflow_resume_total{workflow="card-create-confirm"}`;
  contador do continuer análogo a `agents_destructive_confirm_total`. Labels apenas enums fechados;
  proibido `user_id`/`category_id` (R-AGENT-WF-001.5 / R-TXN-004 / R-WF-KERNEL-001.4).
- **Logs:** `card.create.started|failed|completed` (existentes) + logs do continuer com `wamid`.
- **Traces:** spans `card.usecase.create`, `agents.binding.card_manager.create_card`,
  `agents.usecase.card_create_confirm_continuer`.
- **Dashboards/Alertas:** painel de escrita de cartão; alerta de run suspenso além do TTL.

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — Workflow de confirmação dedicado `card-create-confirm` (vs estender
  `destructive_confirm`). `adr-001-dedicated-card-create-confirm-workflow.md`.
- **ADR-002** — Dia de fechamento explícito opcional (sentinela `ClosingDay int` +
  `ClosingDayProvided bool`) e reconhecimento de banco tool-gated; onboarding intacto.
  `adr-002-closing-day-optional-modeling.md`.
- **ADR-003** — Idempotência + métrica via `IdempotentWriter` dos agents (`operation="create_card"`).
  `adr-003-idempotency-and-metric-via-agents-idempotent-write.md`.

### Riscos Conhecidos

- **LLM não relayar o `ConfirmationPrompt` verbatim ou não reenviar `closingDay`.** Mitigação:
  instruções estritas + harness ≥0.90 + a tool re-emite clarify enquanto faltar dado.
- **Divergência de reconhecimento** entre `IsBankRecognized` e `DaysBeforeDue` — RESOLVIDO: ambos
  aplicam a mesma normalização `NewBankCode` sobre a mesma tabela/coluna; teste com acento/espaço
  garante paridade.
- **Duplicação de máquina de confirmação** (ADR-001). Mitigação: extrair helpers `isSim/isNao` já
  existentes; revisão futura de unificação.
- **Corrida de dois "sim"** antes da conclusão do run. Mitigação: idempotência por `wamid` (ADR-003) +
  índice único de apelido.

### Conformidade com Padrões

- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001): tool fina, estados fechados, LLM só nas
  call-sites sancionadas, run auditável, pending step antes de confirmar, resume antes do parse.
- `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001): kernel genérico, sem domínio/SQL/LLM, resume
  por merge-patch, cardinalidade controlada.
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): zero comentários, adapter fino.
- `.claude/rules/input-dto-validate.md` (R-DTO-VALIDATE-001): `Validate()` no `input.CreateCard`.
- `.claude/rules/go-testing.md` (R-TESTING-001): testify/suite whitebox, mocks do `.mockery.yml`.
- `.claude/rules/governance.md`: DMMF state-as-type prevalece; sem `Result[T,E]`/currying/DSL.

### Arquivos Relevantes e Dependentes

Novos: `internal/agents/application/tools/create_card.go`;
`internal/agents/application/workflows/card_create_state.go`,
`card_create_decisions.go`, `card_create_confirm_workflow.go`;
`internal/agents/application/usecases/card_create_confirm_continuer.go`;
job de reaper em `internal/agents/infrastructure/jobs/handlers/`.

Modificados: `internal/agents/application/interfaces/types.go` (:122),
`internal/agents/application/interfaces/card_manager.go`,
`internal/agents/infrastructure/binding/card_manager_adapter.go` (:58),
`internal/agents/application/agents/mecontrola_agent.go`, `internal/agents/module.go`,
`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`,
`internal/card/application/dtos/input/create_card.go`,
`internal/card/application/usecases/create_card.go` (:87-101),
`internal/card/application/interfaces/*` + `infrastructure/postgres/*` (read `IsBankRecognized`),
`.mockery.yml` (se novas interfaces).

Dependências (inalteradas): `internal/platform/workflow` (kernel), `internal/platform/tool`,
`internal/platform/agent` (RuntimeFrom/InboundRequest), `internal/agents/application/usecases/idempotent_write.go`.
