# Onboarding conduzido por LLM (V1) — conversa real + persistência sem falso positivo

> Plano de implementação. Skill **`go-implementation` obrigatória** (Etapas 1–5) para todo código Go.
> Restrições duras aplicáveis: R-ADAPTER-001.1 (zero comentários em `.go` de produção),
> R-ADAPTER-001.2 (adapters finos handler→usecase), pureza `Decide*` (DMMF), validação só em smart
> constructors, sem abstração de tempo (`time.Now().UTC()` inline), outbox idempotente por `event_id`.

## 1. Contexto e motivação

Objetivo: fazer o `internal/agent` se comportar conforme `MeControla_Onboarding_SystemPrompt_V1.md` —
conduzir o onboarding como jornada de 11 etapas (boas-vindas → metodologia das 5 categorias → objetivo
→ renda → cartões → distribuição do orçamento por valor → resumo/confirmação → transição → primeiro
lançamento → celebração → encerramento), em **conversa real** e com **persistência sem falso positivo**.

Tom V1: positivo, acolhedor, didático, motivador. Emojis permitidos: 👋 🎯 💰 📊 ✅ 🚀 🏆 💳 📅.

### Fato decisivo (verificado no código)

- O onboarding hoje é uma **FSM determinística pura** `OnboardingWorkflow.DecideNext` em
  `internal/onboarding/domain/services/onboarding_workflow.go`, com textos hardcoded e parsers regex
  frágeis (`parseMonetary`, `parseYesNo`, `parseDay`, `parseCardShortcut`).
- O LLM (`internal/agent`) só roda **depois** que a sessão fica `OnboardingStateActive` e **só como
  fallback** para intents `unknown`: `internal/agent/application/services/intent_router.go:245-251`
  chama PRIMEIRO `onboarding.Continue()`; se a sessão não está Active, a FSM responde e o LLM nem roda.
  O persona prompt é consumido só no fallback: `compose_conversational_reply.go:56`.
- **Consequência:** reescrever apenas o prompt da persona NÃO mudaria o onboarding ao vivo (a FSM
  intercepta) — seria falso positivo de entrega. Por isso o LLM precisa conduzir o onboarding.

### Persistência real verificada (downstream dos eventos)

Tudo commitado em transação única no use case (`process_onboarding_message.go`: `Find` → `DecideNext`
→ `Upsert(NewState,NewPayload)` → `Publish(DomainEvents)` no outbox), idempotência inbound por
`message_id` (tabela `channel_processed_messages`).

| Evento | EventType | Efeito downstream | Recurso real? |
|---|---|---|---|
| `IncomeRegistered` | `onboarding.income_registered` | só histórico (outbox) | não |
| `CardRegistered` | `onboarding.card_registered` | consumer `internal/card/.../consumers/onboarding_card_consumer.go:70-78` chama `createCard.Execute` | **sim, cartão consultável** |
| `SplitsCalculated` | `onboarding.splits_calculated` | consumer `internal/budgets/.../consumers/onboarding_budget_consumer.go:94-112` `createBudget` + `activateBudget` | **sim, orçamento criado e ATIVO** |
| `OnboardingCompleted` | `onboarding.completed` | nenhum consumer registrado | só histórico |

- **Objetivo financeiro: NÃO existe persistência** em nenhum módulo (não há tabela/entidade `goal`).
  `routeQueryGoal` (`intent_router.go:377-395`) só LÊ progresso da categoria Metas via `MonthlySummary`.
- **Distribuição por valor: NÃO existe** — só o split percentual padrão `SuggestDefaultSplit`
  (40/10/15/20/15) em `onboarding_workflow.go:582-590`.
- **Primeiro lançamento durante onboarding: não acontece.** O fluxo normal do agent persiste
  transações sem falso positivo (`intent_router.go:299-315` só diz "realizada" após `result.Persisted`).

### Estados atuais (`internal/onboarding/domain/valueobjects/onboarding_state.go`)

`AwaitingToken, AwaitingIncome, AwaitingCardDecision, AwaitingCardName, AwaitingCardLimit,
AwaitingCardClosingDay, AwaitingCardDueDay, AwaitingMoreCards, AwaitingSplitConfirm, Active` (terminal).

### Adapter/wiring da ponte agent→onboarding

`OnboardingContinuation.Continue` (`intent_router.go:108`) é implementado por
`onboardingContinuationAdapter` em `cmd/server/agent_wiring.go:57-114`, que delega a
`WhatsAppMessageProcessor.ProcessConversation` / `TelegramMessageProcessor.ProcessConversation` →
`ProcessOnboardingMessage.Execute` → `OnboardingWorkflow.DecideNext`.

## 2. Decisões de escopo confirmadas com o usuário

1. **Decisão arquitetural:** o **LLM conduz o V1 como conversa real** (não só reescrever prompt).
2. **Objetivo (Etapa 3):** sem entidade cross-módulo. O compromisso financeiro do objetivo é
   representado pelo **orçamento da categoria Metas**; o **texto do objetivo** vai no payload da sessão
   (commitado em tx → sem falso positivo) e é ecoado no resumo.
3. **Orçamento (Etapa 6):** **captura por valor (R$)** por categoria — soma, valida contra a renda
   (sobra/excesso), converte para basis_points, cria/ativa orçamento real.
4. **Primeiro lançamento (Etapa 9):** onboarding conclui e **convida**; registro real pelo fluxo
   normal do agent; **celebração** 🏆 dispara na confirmação do 1º lançamento.

## 3. Abordagem A1 — LLM na borda, domínio puro no centro

Separação por turno preservando DMMF e regras duras:

- **NLU (LLM, IO na application):** `(estado + payload + texto cru)` → `StructuredTurnInput` tipado e
  tolerante, em **uma única chamada** que também devolve `off_topic` + `answer` curto para dúvidas
  (economia: maioria dos turnos = 1 chamada LLM).
- **Decisão (domínio PURO):** `DecideNext(session, StructuredTurnInput, eventIDs, now)` continua sem
  IO/LLM/repo/`time.Now`/uuid aleatório; valida via smart constructors; decide transição + eventos.
  Os parsers regex saem do domínio e viram **fallback determinístico** na borda.
- **Persistência (tx):** `Upsert(sessão)` + `Publish(eventos)` — estrutura inalterada.
- **Reply:** confirmações de dado persistido = **template determinístico** (exatidão, zero alucinação,
  montado do `DecisionOutcome` já commitado); turnos abertos/educativos/dúvida = texto do LLM com
  fallback ao template. **O LLM nunca decide transição nem afirma persistência.**

Garantias anti-falso-positivo:
- Confirmação só é montada **após** o commit da tx; se `Upsert`/`Publish` falham → rollback → reply de
  instabilidade, sem afirmar persistência.
- Falha NLU → fallback regex; se nem regex extrai → `DecisionKindReplyOnly` re-perguntando (nunca trava).
- Falha NLG → retorna `OutboundText` (template). Circuit breaker já existente degrada p/ template/regex.
- Reprocesso do mesmo `message_id` barrado na borda; `event_id` vem do `idGen` (não do LLM); outbox
  idempotente.

### Portas LLM (consumer side, onboarding application)

- `TurnExtractor.Extract(ctx, ExtractInput{State, Payload, Text}) (StructuredTurnInput, error)` (NLU).
- `ReplyComposer.Compose(ctx, ComposeInput{State, Stage, Question, UserText}) (string, error)` (NLG/dúvida).
- `StructuredTurnInput` é DTO de application com campos opcionais por etapa: `IncomeCents *int64`,
  `YesNo *bool`, `Goal *string`, `CardName *string`, `CardLimitCents *int64`, `ClosingDay *int`,
  `DueDay *int`, `BudgetByValue map[CategoryKind]int64`, `Adjust bool`, `OffTopic bool`, `Answer string`,
  `RawText string`.

### Adapters e wiring

- Adapters em `internal/onboarding/infrastructure/llm/` reusam a `chain` do agent
  (`interfaces.LLMProvider.Interpret`): schema JSON estrito p/ NLU (espelha `decodeAndBuild` em
  `parse_inbound.go:159` e `ParseIntentJSONSchema` em `prompting/prompts.go:65`); `FreeText:true` p/ NLG.
  Adapters finos (R-ADAPTER-001.2), sem branching de domínio.
- `cmd/server`: extrair/expor a `chain` para injetar no agent e em `NewProcessOnboardingMessage`.
  **Risco a validar antes da Fase 1:** ordem de construção (onboarding é montado antes do agent) e
  ausência de import cycle — infra do onboarding deve depender só de `agent/application/interfaces`.

### Fluxo de um turno (em `ProcessOnboardingMessage`)

```
1. repo.Find(userID)
2. now := time.Now().UTC(); eventIDs := allocate(state)
3. structured := turnExtractor.Extract(...)        // NLU só em estados de dado; erro → fallback regex
4. decision := workflow.DecideNext(session, structured, eventIDs, now)   // PURO
5. tx: Upsert(NewState,NewPayload) + Publish(DomainEvents)               // já existe
6. reply := dado persistido ? decision.OutboundText (template)
                            : replyComposer.Compose(...) (fallback OutboundText)   // após o commit
```

Compor o NLG **após** o commit (reply textual não é transacional; falha de LLM não deve causar rollback
de transição já persistida) — mover a composição p/ depois de `Execute` retornar o outcome (no processor
ou em campo estruturado do result).

### Chamadas LLM por estado (economia)

| Estado | NLU | NLG | Nota |
|---|---|---|---|
| Welcome / metodologia | 0 | 0 (estático) / 1 se dúvida | roteirizado |
| Goal (objetivo) | 1 | 0 | eco em template |
| Income | 1 | 0 | confirmação template |
| CardDecision / MoreCards | 1 | 0 | sim/não tolerante |
| CardName/Limit/ClosingDay/DueDay | 1 | 0 | confirmação template (exatidão) |
| BudgetByValue | 1 | 0 | validação + eco template |
| SummaryConfirm | 1 | 0 | resumo 100% template |
| Active (1º lançamento) | — | — | já é agentRoute |
| Qualquer estado + dúvida | 1 (detecta `off_topic`+`answer`) | 0 (answer já vem no NLU) | 1 chamada |

Regra: estado sem dado = 0 NLU; estado de dado = 1 NLU; confirmação = 0 LLM; dúvida resolvida na mesma
chamada NLU (campos + `off_topic` + `answer`). Pior caso prático = 1 chamada/turno.

## 4. Novos estados, value objects e evento

### Estados novos (`onboarding_state.go`) — **não remover** os legados (sessões em voo); `ParseOnboardingState` aceita ambos

- `OnboardingStateAwaitingWelcome` — boas-vindas + metodologia das 5 categorias (apresentadas
  individualmente, "faz sentido?"). **Novo estado inicial pós-ativação** (hoje vai direto a
  `AwaitingIncome` via `start_budget_configuration.go`).
- `OnboardingStateAwaitingGoal` — objetivo financeiro. Posição: `Welcome → Goal → Income`.
- `OnboardingStateAwaitingBudgetByValue` — distribuição por R$ (substitui semanticamente
  `AwaitingSplitConfirm` no fluxo novo; manter `AwaitingSplitConfirm` no enum p/ default-split/legado).
- `OnboardingStateAwaitingSummaryConfirm` — resumo + ajustar/confirmar; antes de `Active`.

`transitionToSplit` passa a apontar p/ `AwaitingBudgetByValue`. Cartões inalterados (mantém
limite+vencimento já capturados, superset da spec que pede apelido+fechamento).

### Mudança de shape do `DecideNext`

`DecideNext(session, InboundMessage, ...)` → `DecideNext(session, StructuredTurnInput, ...)`. Caller
único de produção (`process_onboarding_message.go:109`); testes (33 usos) reescritos para passar o DTO
direto (domínio testável puro, sem LLM). Parsers regex migram para `internal/onboarding/application/turnparse/`
como fallback do adapter NLU (não no domínio).

### Value objects (smart constructors — invariantes no construtor, nunca no use case)

- **`FinancialGoal`** (`financial_goal.go`): texto não vazio após trim; `len <= 280` runes.
  Erros: `ErrFinancialGoalEmpty`, `ErrFinancialGoalTooLong`.
- **`BudgetByValue`** (`budget_by_value.go`) — VO central da Etapa 6:
  - input: `income int64` + `map[CategoryKind]int64` (cents por categoria).
  - invariantes: 5 categorias presentes (faltante → 0, explícito); cada valor `>= 0`;
    `soma > income` → `ErrBudgetByValueExceedsIncome` (carrega `excessoCents`); `soma < income` →
    permitido, expõe `LeftoverCents()`; `soma == income` → ok.
  - conversão: `bp_i = round(value_i * 10000 / income)` com **ajuste de resíduo** (maior categoria
    recebe o resto) garantindo `sum(bp) == 10000` (compatível com `BasisPoints` 0..10000 de budgets).
  - métodos: `Allocations() []CategoryAllocation` (basis_points), `LeftoverCents()`, `TotalCents()`.
  - erros: `ErrBudgetByValueWrongSize`, `ErrBudgetByValueNegative`, `ErrBudgetByValueExceedsIncome`.
  - DMMF: validação soma vs renda vive **aqui**. `DecideNext` chama `NewBudgetByValue`; excesso →
    `DecisionKindReplyOnly` com template "passou da renda em R$ X"; sobra → `AdvanceState` e o resumo
    menciona a sobra.

`CategorySplit` (percent, soma 100±1) permanece p/ a sugestão default; `BudgetByValue` é o caminho por
valor. Ambos convergem em allocations basis_points no evento.

### Evento de orçamento — `BudgetDistributed` (recomendado)

Reusar `SplitsCalculated` é **insuficiente**: seu campo `Percent int` perde precisão (32,47% = 3247 bp
não é múltiplo de 100). Criar evento `onboarding.budget_distributed` com `TotalCents int64` +
`Allocations []{RootSlug string, BasisPoints int}` (basis_points diretos). Novo consumer em budgets
(`onboarding_budget_distributed_consumer.go`) reusa `createBudget`+`activateBudget` (mesma idempotência
por `ErrBudgetConflict → nil`); registrar `{EventType:"onboarding.budget_distributed", Handler:...}` em
`budgets/module.go`. `SplitsCalculated` permanece p/ default-split/legado. Categoria "Metas" representa
o compromisso do objetivo (sem entidade nova). `allocateEventIDs`/`extractEventID`/`buildOutboxEvent`
em `process_onboarding_message.go` ganham o case do novo evento (`SummaryConfirm` aloca 2 IDs:
BudgetDistributed + OnboardingCompleted).

### Persistência do payload

Estender `OnboardingSessionPayload` (`onboarding_session.go`): `Goal string` e
`BudgetByValueCents map[string]int64` (key = `CategoryKind.String()`). Espelho JSON em
`onboarding_session_repository.go` (`goal`, `budget_by_value`). **Sem migration** — coluna `payload` é
JSON; campos novos default zero/nil em sessões antigas. Atualizar mapping `Find`/`Upsert` e helpers.

## 5. System prompts

Novo pacote `internal/onboarding/application/prompting/` (espelha o do agent):
- `turn_extract.system.tmpl` (NLU): "extraia APENAS os campos pedidos em JSON estrito; não invente; null
  se não houver" + schema dos campos do `StructuredTurnInput` (schema único, estado guia no system/user).
- `onboarding_nlg.system.tmpl` (NLG): persona V1 das 11 etapas (tom + emojis permitidos), regras
  anti-falso-positivo ("NUNCA afirme que salvou/registrou — isso é dito pelo sistema"; reaproveitar as
  cláusulas anti-alucinação de `agent/.../persona.system.tmpl:26-29`), responder dúvida em 1-2 frases e
  voltar à etapa.
- `JourneyHint` (`PersonaSystemData.JourneyHint`, `prompts.go:31`) passa a carregar o estado/etapa atual
  ("Etapa 6 de 11: distribuição por valor"). **Não reescrever** o `persona.system.tmpl` do agent (serve
  o pós-onboarding/Active) — evita regressão. NLG do onboarding usa template próprio
  (`OnboardingJourneyData{Stage, StageName, Question, UserText, payload echo}`).

## 6. Plano faseado (cada fase verificável; juntas entregam o V1 completo)

### Fase 1 — núcleo A1 (shippable)
Aplica A1 **aos estados existentes** (renda/cartões/split) — já entrega conversa real tolerante sem
novos estados.
1. `StructuredTurnInput` (application) + nova assinatura de `DecideNext`.
2. Migrar parsers regex p/ `application/turnparse` como fallback; `DecideNext` consome só o DTO.
3. Portas `TurnExtractor`/`ReplyComposer` + adapters em `infrastructure/llm/` reusando a `chain`.
4. Wiring em `cmd/server` (passar chain ao onboarding) + injeção em `NewProcessOnboardingMessage`.
5. Templates NLU/NLG mínimos; NLG só p/ `OffTopic`/dúvida; confirmações por template.
6. Fallback determinístico completo (LLM off = comportamento atual).
**Verificação:** `go build ./...`; `go test ./internal/onboarding/...` (DecideNext com DTO; adapters com
mock de chain); gate de comentários R-ADAPTER-001.1; smoke ativar→income/cartão/split com linguagem
natural confirmando persistência (evento outbox, cartão criado, budget ativado).

### Fase 2 — educativo + objetivo
1. Estados `AwaitingWelcome` + `AwaitingGoal`; estado inicial pós-ativação = `AwaitingWelcome`.
2. VO `FinancialGoal`; campo `Goal` no payload + JSON repo.
3. Metodologia das 5 categorias individual ("faz sentido?") — texto estático + NLG p/ dúvidas.
4. Eco do objetivo no resumo.
**Verificação:** build; testes `FinancialGoal` (vazio/longo/ok); transições `Welcome→Goal→Income`; gate;
smoke da jornada inicial.

### Fase 3 — orçamento por valor + resumo/confirmação + evento
1. Estados `AwaitingBudgetByValue` + `AwaitingSummaryConfirm`; `transitionToSplit`→`AwaitingBudgetByValue`.
2. VO `BudgetByValue` (soma/renda/sobra/excesso/→basis_points com resíduo).
3. Evento `BudgetDistributed` + consumer em budgets (reusa create+activate).
4. Resumo 100% template (renda, objetivo→Metas, cartões, distribuição) + ajustar/confirmar.
5. `allocateEventIDs`/`extractEventID`/`buildOutboxEvent` p/ o novo evento.
**Verificação:** build; testes `BudgetByValue` (excesso/sobra/exato/arredondamento fechando 10000);
consumer budgets (basis_points diretos → budget ativado, idempotência `ErrBudgetConflict`); round-trip
do payload no repo; gate; smoke distribuindo por R$ acima/abaixo/igual à renda.

### Fase 4 — transição, 1º lançamento, celebração, encerramento
1. `AwaitingSummaryConfirm → Active` + convite ao 1º lançamento (texto).
2. 1º lançamento real pelo fluxo do agent (já persiste sem falso positivo); celebração 🏆 na confirmação
   do lançamento (no `routeLogExpense`/`routeLogIncome` via flag de "primeiro lançamento" lida da sessão
   recém-Active).
3. Encerramento.
**Verificação:** build; testes do route de celebração; smoke fim-a-fim das 11 etapas; gate;
auditoria do módulo.

## 7. Arquivos

**Criar:**
- `internal/onboarding/application/interfaces/turn_extractor.go`, `reply_composer.go` (+ mocks em `.../mocks/`)
- `internal/onboarding/application/turnparse/parsers.go` (regex migrados) (+ `parsers_test.go`)
- `internal/onboarding/application/prompting/prompts.go`, `turn_extract.system.tmpl`,
  `onboarding_nlg.system.tmpl` (+ `persona_test.go`)
- `internal/onboarding/infrastructure/llm/turn_extractor_adapter.go`, `reply_composer_adapter.go`
  (+ tests com mock de chain)
- `internal/onboarding/domain/valueobjects/financial_goal.go` (+ test)
- `internal/onboarding/domain/valueobjects/budget_by_value.go` (+ test)
- `internal/budgets/infrastructure/messaging/database/consumers/onboarding_budget_distributed_consumer.go`
  (+ test)
- `cmd/server/onboarding_llm_wiring.go`

**Alterar:**
- `internal/onboarding/domain/services/onboarding_workflow.go` (+ `_test.go` reescrito p/ DTO)
- `internal/onboarding/domain/valueobjects/onboarding_state.go` (+4 estados, `String`/`Parse`)
- `internal/onboarding/domain/entities/onboarding_session.go` (payload `Goal`/`BudgetByValueCents`)
- `internal/onboarding/domain/entities/onboarding_session_events.go` (evento `BudgetDistributed`)
- `internal/onboarding/application/usecases/process_onboarding_message.go`
- `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository.go`
- `internal/onboarding/application/services/whatsapp_message_processor.go`, `telegram_message_processor.go`
- `internal/onboarding/application/usecases/start_budget_configuration.go` (estado inicial `AwaitingWelcome`)
- `internal/onboarding/module.go`, `internal/budgets/module.go`, `internal/agent/module.go` (expor `chain`)

## 8. Riscos

1. **Import cycle / ordem de wiring** ao compartilhar a `chain` — validar antes de codar a Fase 1
   (onboarding é construído antes do agent; infra do onboarding deve depender só de
   `agent/application/interfaces`).
2. **Precisão basis_points** (soma 9999/10001) — resíduo determinístico p/ a maior categoria; testar.
3. **Falso positivo via NLG** — confirmações sempre por template; teste garantindo que outcomes com
   evento de persistência usam `OutboundText`, não o composer.
4. **Sessões legadas** — manter estados antigos no enum e em `ParseOnboardingState`.
5. **Custo/latência** — NLU + dúvida numa única chamada (campos + `off_topic` + `answer`).
6. **Gate de comentários R-ADAPTER-001.1** — `.tmpl` fora; cuidado nos adapters novos.
7. **Idempotência do novo evento** — `event_id` do `idGen` (não do LLM); reprocesso barrado na borda.

## 9. Verificação consolidada

- `go build ./...`
- `go test ./internal/onboarding/... ./internal/budgets/... -count=1`
- Gate de comentários: `grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go"
  "^[[:space:]]*//" internal/onboarding internal/budgets | grep -Ev "(//go:|//nolint:|// Code generated)"`
  (deve ser vazio)
- Sanidade do `Sprintf` do prompt builder (sem `%!s(MISSING)`/`%!(EXTRA...)`)
- Smoke `AGENT_MODE=openrouter` + `OPENROUTER_API_KEY`: ativar conta e percorrer a jornada com linguagem
  natural ("uns 3 mil e meio", "pode ser", distribuir por R$ acima/abaixo da renda), confirmando
  persistência real (cartão criado, orçamento ativado, eventos no outbox).

---

# Apêndice A — Achados brutos de investigação (fonte: leitura de código)

## A.1 Roteamento inbound (WhatsApp/Telegram)

- Ponto de entrada HTTP: `internal/platform/whatsapp/handlers/inbound_handler.go:27` → `dispatcher.Route()`
  em `internal/platform/whatsapp/dispatcher/dispatcher.go:106-180`.
- Decisão de rota (`dispatcher.go:145-180`): contém comando `ATIVAR` → `onboardingRoute`; usuário
  desconhecido → `onboardingRoute` (fallback); usuário conhecido + rate-limit ok → `agentRoute` (LLM).
- `onboardingRoute` wiring: `cmd/server/whatsapp_wiring.go:30-44` (closure → `processor.HandleActivation()`
  / `processor.HandleFallback()`, processor = `WhatsAppMessageProcessor` do onboarding).
- `agentRoute`: `internal/agent/module.go:298-329` → `IntentRouter.RouteWhatsApp()` p/ usuários autenticados.
- **Agent vs Onboarding (temporal):** o agent roda DEPOIS do onboarding. Em `intent_router.go:245-250`,
  `route()` chama PRIMEIRO `onboarding.Continue()`; se a sessão não é `Active`, a FSM responde
  (`OutboundText` hardcoded) e o LLM nem roda. Só após `OnboardingStateActive` segue para `ParseInbound`.
- `persona.system.tmpl` consumido em `compose_conversational_reply.go:56` (`RenderPersonaSystem`) — usado
  só para conversa livre/fallback (`Fallback.Reply`), nunca persiste.
- `systemPromptTemplate`/`BuildSystemPrompt` (`prompt_builder.go:124-199`): prompt de PARSING de intent
  com contexto dinâmico (user_id, channel, permissions, categories, cards, date). Não usado no handler
  inbound atual (preparado p/ futuro).
- `JourneyHint` (`prompts.go:31-32`): definido, mas SEMPRE chamado com `PersonaSystemData{}` vazio —
  nunca preenchido hoje.

## A.2 Sessão de onboarding — entidade, payload, persistência

- `internal/onboarding/domain/entities/onboarding_session.go`:
  - `OnboardingSession` (67-73): `userID`, `channel`, `state`, `payload`, `updatedAt`.
  - `OnboardingSessionPayload` (54-60): `IncomeCents int64`, `Cards []OnboardingCardDraft`,
    `PendingCard OnboardingCardDraft`, `HasPending bool`, `Split []OnboardingCardSplitEntry`.
  - `OnboardingCardDraft` (47-52): `Name`, `LimitCents`, `ClosingDay`, `DueDay`.
  - `OnboardingCardSplitEntry` (62-65): `Kind string`, `Percent int`.
  - **Não há** campo de objetivo nem de orçamento por valor.
- Repositório: `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository.go`:
  - `Find` (50-100): `SELECT ... WHERE user_id=$1`, desserializa JSON payload → `HydrateOnboardingSession`.
  - `Upsert` (102-139): serializa payload p/ JSON, `INSERT ... ON CONFLICT (user_id) DO UPDATE`
    (campos: `user_id`, `channel`, `state`, `payload`, `updated_at`).
- Ciclo de turno: `internal/onboarding/application/usecases/process_onboarding_message.go`:
  `executeInTx` → `repo.Find` (98) → `allocate event IDs` (107) → `workflow.DecideNext(session, msg,
  eventIDs, now)` (109) → `session.With(NewState, NewPayload, now)` + `repo.Upsert` (133-136) →
  para cada evento `buildOutboxEvent` + `publisher.Publish` (137-145).
- Idempotência: `ProcessOnboardingMessageInput` recebe `MessageID` (25-30); dedupe via tabela
  `mecontrola.channel_processed_messages` (`onboarding_cleanup_repository.go:131-149`); guarda extra de
  estado (`Active` → `NoOp`).
- Implementação de `OnboardingContinuation.Continue`: `onboardingContinuationAdapter` em
  `cmd/server/agent_wiring.go:57-114` → `WhatsAppMessageProcessor.ProcessConversation` (102-112) /
  `TelegramMessageProcessor.ProcessConversation` (84-100) → `ProcessOnboardingMessage.Execute` →
  `OnboardingWorkflow.DecideNext`.
- Criação inicial da sessão: `internal/onboarding/application/usecases/start_budget_configuration.go`:
  cria nova sessão em `OnboardingStateAwaitingIncome` (103-115); se `Active`, RESETA p/ `AwaitingIncome`
  zerando payload (121-136); se intermediário, RESUME (137-148). Ativação `ATIVAR <codigo>` via
  `consume_magic_token.go:174-191` (`SubscriptionBindingService.BindAndConsume`).
- Estados (`onboarding_state.go:9-20`, strings 22-47): `awaiting_token`, `awaiting_income`,
  `awaiting_card_decision`, `awaiting_card_name`, `awaiting_card_limit`, `awaiting_card_closing_day`,
  `awaiting_card_due_day`, `awaiting_more_cards`, `awaiting_split_confirm`, `active` (terminal, 49-51).

## A.3 O que os eventos fazem downstream (real vs histórico)

- Definições: `internal/onboarding/domain/entities/onboarding_session_events.go:13-59`
  (`IncomeRegistered` 13-21, `CardRegistered` 23-34, `SplitsCalculated` 36-50, `OnboardingCompleted`
  52-59). Todos publicados via outbox em `process_onboarding_message.go:137-145` (persistidos, não
  descartados).
- `CardRegistered` → `internal/card/infrastructure/messaging/database/consumers/onboarding_card_consumer.go:48-82`
  (extrai payload 70-78, chama `createCard.Execute`) → **cartão real consultável** (CardLister/GetCard).
- `SplitsCalculated` → `internal/budgets/infrastructure/messaging/database/consumers/onboarding_budget_consumer.go:64-114`
  (`createBudget.Execute` 94-99, `activateBudget.Execute` 107-112; mapa de alocações 132-147 converte
  splits p/ slugs reais, ex. `goals → expense.metas`) → **orçamento real criado e ATIVO**.
- `IncomeRegistered`: só histórico. `OnboardingCompleted`: **nenhum consumer registrado** — só histórico.
- **Goal/objetivo:** nenhuma tabela `goals`/`objectives`, nenhum struct `Goal`, nenhum repositório.
  "Metas" é apenas categoria de gasto (20% do split). `routeQueryGoal` (`intent_router.go:380-398`) LÊ
  progresso via `MonthlySummaryReader`; não há onde GRAVAR o objetivo declarado.
- `configure_budget`: `intent_router.go:286` `routeConfigureBudget` → `budgetConfig.Start` =
  `StartBudgetConfiguration` (`start_budget_configuration.go:72-149`) — apenas INICIA/RETOMA sessão; o
  fluxo segue via `ProcessOnboardingMessage`.
- Ao completar (`onboarding_workflow.go:389-401`): emite `SplitsCalculated` + `OnboardingCompleted`,
  estado → `Active`. Recursos consultáveis: orçamento ativo (budgets), cartões (card), alocações; objetivo
  declarado NÃO; histórico de eventos no outbox.

# Apêndice B — Notas de design DMMF e alternativas avaliadas

## B.1 Evento de orçamento: por que `BudgetDistributed` e não reusar `SplitsCalculated`

- O consumer de budgets atual deriva `BasisPoints = Percent*100` e monta
  `CreateBudgetInput{TotalCents, Allocations:[{RootSlug, BasisPoints}]}`.
- **Opção A (rejeitada):** reusar `SplitsCalculated` populando `Allocations` via `Percent int`. Captura
  por valor gera basis_points que raramente são múltiplos de 100 (ex. 32,47% = 3247 bp) → `Percent int`
  perde precisão. Insuficiente.
- **Opção B (adotada):** novo evento `onboarding.budget_distributed` com `TotalCents int64` +
  `Allocations []{RootSlug string, BasisPoints int}` (basis_points diretos, sem perda). Novo consumer em
  budgets reusa `createBudget`+`activateBudget` (idempotência por `ErrBudgetConflict → nil`); registrar
  o handler em `budgets/module.go`. `SplitsCalculated` mantido p/ default-split/legado.

## B.2 Parsers regex: migração do domínio para a borda

- Hoje em `onboarding_workflow.go`: `parseMonetary` (592), `normalizeMonetaryInput` (608),
  `normalizeMonetarySeparators` (622), `parseDay` (664), `parseYesNo` (686), `isAffirmation` (714),
  `parseCardShortcut` (404) etc.
- Com A1, o domínio recebe `StructuredTurnInput` e NÃO parseia texto cru. Os parsers migram para
  `internal/onboarding/application/turnparse/` e servem de **fallback determinístico** do
  `TurnExtractorAdapter` quando o LLM falha/retorna JSON inválido — garantindo que o onboarding nunca
  trava sem o LLM (comportamento equivalente ao atual).

## B.3 Pureza `Decide*` e fronteiras (regras duras)

- `DecideNext` e todos os `decide*` permanecem PUROS: sem IO/LLM/repo, `now` recebido por parâmetro,
  `eventIDs []uuid.UUID` recebidos e consumidos em ordem (sem geração aleatória), sem logging.
- Validação de invariante (objetivo não vazio; 5 categorias; soma vs renda; basis_points fechando 10000)
  vive SOMENTE em smart constructors dos value objects (`FinancialGoal`, `BudgetByValue`), nunca em use
  case/handler/adapter.
- Adapters LLM (`infrastructure/llm/`) e wiring são finos (R-ADAPTER-001.2): só (de)serialização e
  chamada da `chain`; sem branching de domínio, sem SQL.
- Zero comentários em `.go` de produção (R-ADAPTER-001.1); `.tmpl` ficam fora do gate.
- Sem abstração de tempo: `time.Now().UTC()` inline no use case (não injetar Clock).
- Outbox idempotente por `event_id`; `event_id` vem do `idGen` do use case, nunca do LLM.

## B.4 Onde o reply NLG é composto (fronteira transacional)

- Confirmação de dado persistido = `decision.OutboundText` (template), montada a partir do
  `DecisionOutcome` retornado APÓS o commit da tx. Se `Upsert`/`Publish` falham → rollback → `Execute`
  retorna erro → processor responde instabilidade (sem afirmar persistência).
- NLG (frasear pergunta/educação/dúvida) roda APÓS o commit (reply textual não é transacional); falha de
  LLM não causa rollback de transição já persistida. Mover a composição p/ após `Execute` retornar o
  outcome (no processor ou em campo estruturado do result). Teste dedicado garante que outcomes com
  evento de persistência usam `OutboundText` e não passam pelo composer (anti-falso-positivo).
</content>
</invoke>
