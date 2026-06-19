# Onboarding Conversacional por IA — Plano de Implementação (MVP)

> Skill obrigatória para toda edição Go deste run: `.agents/skills/go-implementation/SKILL.md` (Etapas 1–5), com DMMF (`references/domain-modeling.md`) onde houver tipo/estado. Zero comentários em `.go` de produção (R-ADAPTER-001.1).

## Context

Hoje o onboarding do MeControla é uma **máquina de estados determinística** (`internal/onboarding`, `DecideNext`, regex/tokenizers) interceptada no início do `IntentRouter` via `onboardingContinuationAdapter`; o LLM não participa. O objetivo (`docs/prompts/prompt-onboarding-conversational.md`) é colocar a IA conduzindo um onboarding conversacional pelas Etapas 1–11 (boas-vindas → metodologia → objetivo → orçamento → cartões → splits → resumo → primeiro lançamento → conclusão), respondendo dúvidas e retomando de onde parou.

**Descoberta load-bearing:** o caminho de **tool-calling do LLM NÃO está ligado** ao fluxo vivo. `ParseInbound.Execute` (`internal/agent/application/usecases/parse_inbound.go:100-104`) envia só `JSONSchema`, nunca `Tools`. `AgentToolCatalog()`/`ToolCallToIntent()` existem mas são código morto no runtime. A infra do provider (`openrouter.buildRequestBody`/`parseToolCalls`) já suporta `Tools`/`ToolCalls`. Logo, esta feature **constrói um novo loop de execução de tools**.

**Requisito inegociável:** `internal/agent` é a interface direta com WhatsApp + LLM + módulos internos → robusto, resiliente, econômico e production-proof, **sem falso positivo**.

## Decisões aprovadas

1. **Flag `AGENT_ONBOARDING_LLM_ENABLED` (default ON), IA primária + determinístico como degradação resiliente.** A máquina determinística NÃO é deletada; vira rede de segurança quando o LLM falha/timeout. Rollback = desligar a flag. Router recebe `OnboardingRunner` XOR `Onboarding` conforme a flag.
2. **1 chamada de LLM por mensagem (batch único), narração determinística dos resultados de tool.** Loop infinito impossível por construção; sem 2ª ida ao LLM; sem mexer em `LLMRequest.Messages`/`buildRequestBody`. Confirmações e erros de validação são mensagens didáticas templadas; correção no próximo turno. Texto livre do LLM só quando NÃO há tool call.
3. **WhatsApp primeiro.** Telegram permanece no determinístico.

## Ajustes de UX e regras de conversa (obrigatórios)

- **"renda" → "orçamento"** em toda comunicação. Tipos de domínio (`MonthlyIncome`/`IncomeCents`/`IncomeRegistered`) permanecem internos e inalterados; muda só o texto exibido.
- **Cada etapa explica antes de pedir o input.**
- **5 categorias com exemplos concretos**, uma de cada vez: 💰 Custo Fixo (aluguel, água, luz, telefone); 🎓 Conhecimento (livros, cursos, estudos); 🎉 Prazeres (lazer, jantares, diversão); 🎯 Metas (objetivos curto/médio prazo); 🏦 Liberdade Financeira (investimentos e reserva).
- **Etapa 6 recebe input em R$** (não percentual). Agent converte para % no resumo. Regra: `sum(amount_cents) == orçamento` (100%). Sobra/excesso → mensagem didática.
- **Cartão pede só apelido + dia de vencimento.** Fechamento e limite zerados no draft (MVP).

## Governança obrigatória

- `go-implementation` (Etapas 1–5). **Zero comentários** em `.go` de produção. `context.Context` em fronteira IO; interface no consumidor; sem `init()`; sem `panic` (exceto `template.Must`); `errors.Join`/`fmt.Errorf("%w")`.
- **DMMF**: smart constructors, illegal states unrepresentable; validação só em VO/construtor.
- Adaptadores finos: `handler/dispatcher → usecase`. Sem SQL direto nem regra de domínio em adapter.

---

## Parte A — Domínio + Persistência (`internal/onboarding`)

Mudanças aditivas, JSONB retrocompatível (`omitempty`).

- **`domain/valueobjects/financial_objective.go` (novo):** `FinancialObjective` + `NewFinancialObjective(raw) (T, error)` — trim, colapso de espaços, bounds 1..280; `ErrFinancialObjectiveEmpty`/`ErrFinancialObjectiveTooLong`.
- **`domain/valueobjects/budget_allocation.go` (novo):** `BudgetAllocation` em basis points (5 categorias, bp 0..10000, soma 10000, sem duplicata). `NewBudgetAllocationFromAmounts(items []{Kind,AmountCents}, totalCents) (T, error)` valida `sum==total` e calcula bp; `ErrBudgetAllocationWrongSize`/`OutOfRange`/`SumMismatch`. Getter `Percent(kind)`.
- **`domain/entities/onboarding_session.go` (editar):** payload ganha `Objective string`, `CustomSplit []OnboardingBudgetAllocationEntry{Kind,BasisPoints}`, `FirstTxRecorded bool`. Métodos puros `WithObjective`/`WithIncome`/`WithAppendedCard`/`WithCustomSplit`/`WithFirstTransactionRecorded`/`HasFirstTransaction`/`IsReadyToComplete`. `NewOnboardingCardDraft(nickname, dueDay)` — só apelido + vencimento.
- **`domain/valueobjects/onboarding_state.go` (editar):** `OnboardingStateAwaitingFirstTransaction` (não-terminal) + `String()`/`ParseOnboardingState`.
- **`domain/services/onboarding_workflow.go` (editar):** `BuildSplitsCalculatedFromAllocation(...)` (bp→percent) reusando `SplitsCalculated`.
- **`infrastructure/repositories/postgres/onboarding_session_repository.go` (editar):** `onboardingSessionPayloadJSON` ganha `objective`/`custom_split`/`first_tx_recorded` (`omitempty`) + helpers `to/fromAllocationJSON`, ligados em `Find`/`Upsert`.

## Parte B — Usecases (`internal/onboarding/application/usecases`)

Shape de `ProcessOnboardingMessage`/`StartBudgetConfiguration` (uow.Do, factory, o11y, ctx first). Validação via VO.

- `GetOnboardingContext` (read; `Found:false` em not-found).
- `SaveOnboardingObjective` (VO → `WithObjective`).
- `SaveOnboardingIncome` (orçamento; `NewMonthlyIncome` → `WithIncome` → publica `IncomeRegistered`).
- `SaveOnboardingCard` (`NewOnboardingCardDraft(nickname,dueDay)` → `WithAppendedCard`, dedupe → publica `CardRegistered`).
- `SaveOnboardingBudgetSplits` (R$ por categoria + orçamento da sessão; `NewBudgetAllocationFromAmounts`; `ErrBudgetAllocationSumMismatch` didático; sucesso → `WithCustomSplit`, avança `AwaitingFirstTransaction`, publica `SplitsCalculated`).
- `MarkFirstTransactionRecorded` (flip idempotente).
- `CompleteOnboardingSession` (gate `HasFirstTransaction`; senão `ErrOnboardingFirstTransactionRequired`; senão `MarkActive` + `OnboardingCompleted`; `AlreadyActive` idempotente).
- **`internal/onboarding/module.go` (editar):** expor factory de sessão + usecases ao agent.

## Parte C — Agent (`internal/agent`)

- **`application/usecases/onboarding_tool_catalog.go` (novo):** `OnboardingToolCatalog()` com 6 tools + `record_transaction`. `save_onboarding_card({nickname,due_day})`; `save_onboarding_budget_splits({allocations:[{root_slug,amount_cents}]})` (R$); `save_onboarding_income({income_cents})`; `save_onboarding_objective({objective})`; `get_onboarding_context`; `complete_onboarding_session`.
- **`application/usecases/run_onboarding_turn.go` (novo):** `RunOnboardingTurn` (1 LLM/turno). Interfaces consumer `OnboardingStateReader.Load`, `OnboardingToolDispatcher.Dispatch(ctx,userID,channel,call)`. Snapshot → `RenderOnboardingSystem` → `Interpret(Tools,ToolChoice:auto)` → sem tool call: texto livre; com tool calls: executa + narra determinístico. Timeout/erro provider → `Handled:false`/mensagem suave.
- **`infrastructure/onboarding/onboarding_state_reader.go` (novo):** adapter → repo; not-found → `InProgress:false`.
- **`infrastructure/onboarding/onboarding_tool_dispatcher.go` (novo):** Facade fino tool→usecase. `record_transaction` durante onboarding: `LogTransactionFromAgent`; só em `Persisted==true` chama `MarkFirstTransactionRecorded`. Erros de validação → `ToolExecResult` recuperável.
- **`application/prompting/onboarding.system.tmpl` (novo) + `prompts.go` (editar):** prompt V2 adaptado às regras de UX (orçamento, explica-antes-de-perguntar, categorias com exemplos, Etapa 6 em R$). `RenderOnboardingSystem(OnboardingSystemData)`.
- **`application/services/intent_router.go` (editar):** dep `OnboardingTurnRunner`; em `route()` antes do `Continue` determinístico; flag ON → roda IA; degrada se falhar; flag OFF → `Continue()` inalterado. Métrica de caminho.
- **`module.go`/`cmd/server/server.go` (editar):** `attachOnboardingLLM` (espelha `attachBudgetConfigSession`); `deps.OnboardingRunner` XOR `deps.Onboarding` por `cfg.AgentConfig.OnboardingLLMEnabled`. Config `OnboardingLLMEnabled` (env `AGENT_ONBOARDING_LLM_ENABLED`, default true) + `OnboardingMaxToolRounds`.

## Parte D — E2E (`internal/agent/e2e`, Godog)

- **`features/f06_whatsapp_onboarding_conversational.feature` (novo, `@onboarding`):** jornada 1–11 conclui só após 1º lançamento; não conclui antes; splits acima/abaixo → mensagem didática + sem persistência + teto de tool-calls (anti-loop); dúvida-no-meio retoma etapa.
- **`onboarding_mock_llm_test.go` (novo, `//go:build e2e`):** `scriptedOpenRouterServer` (mutex, cursor por inbound, teto callCount) devolvendo `tool_calls` ou texto; plugado via `openrouter.NewProvider`+`FallbackChain`, `Temperature:0`.
- **`onboarding_steps_test.go` (novo, `//go:build e2e`):** steps de estado/objetivo/orçamento/cartões/splits/transações/teto. Reusa steps existentes.
- **`onboarding_suite_test.go` (novo, `//go:build e2e`):** `TestOnboardingE2E` `Tags:"@onboarding"`; `TestE2E` ganha `Tags:"~@onboarding"`. Reset trunca `onboarding_sessions` + cursor/callCount.

---

## Sequência de execução

1. Salvar plano (este arquivo) + runbook `docs/runbooks/onboarding-conversational.md`.
2. Parte A + testes unitários puros.
3. Parte B + exportar factory + testes.
4. Parte C + testes.
5. Parte D (e2e).
6. Validação.

## Verificação

- `go build ./...`
- `go test -race -count=1 ./internal/onboarding/... ./internal/agent/...`
- `go test -race -tags e2e ./internal/agent/e2e/... -run TestOnboardingE2E`
- `golangci-lint run`
- Gate comentários vazio: `grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" "^[[:space:]]*//" internal/agent internal/onboarding | grep -Ev "(//go:|//nolint:|// Code generated)"`
- `task mocks` sem diff inesperado.
- Checklist R0–R7 reportado.
- Comportamento: flag ON conclui só após 1º lançamento; erro de split didático sem loop; flag OFF inalterado.
