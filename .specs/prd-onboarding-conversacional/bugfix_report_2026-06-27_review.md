# Relatorio de Bugfix — Ciclo de Review 2026-06-27

- Total de bugs no escopo: 5 (F1=BUG-101, F2=BUG-102, F3=BUG-103, F4=BUG-104, F6=BUG-105)
- Corrigidos: 4
- Skipped (com diagnostico): 1 (BUG-104)
- Testes de regressao adicionados: 9
- Pendentes: []
- Estado final: done

## Bugs

- ID: BUG-101
- Severidade: major
- Origem: review finding F1 (RF-05, RF-06, RF-10, RF-13; Cap.08 ETAPAS 2/3/4/6; O1)
- Estado: fixed
- Causa raiz: `advance()` retornava `StepStatusCompleted` sem prompt; a unica saida ao usuario era a pergunta da proxima etapa, descartando as confirmacoes oficiais ("🎯 Perfeito!", "✅ Orçamento registrado R$ X", "✅ Cartão salvo … 📅 dia N", "✅ <Categoria> definido — R$ X").
- Correcao: campo `Ack` em `OnboardingState`; `advance(ctx,s,phase,ack)` carrega o ack e `suspend` faz prepend+clear antes de emitir o prompt da proxima etapa. Render methods deterministicos no tom oficial: `RenderObjectiveSaved`, `RenderBudgetSaved`, `RenderCardSaved`, `RenderValueSaved` (`onboarding_interpreter_render.go`). Steps objective/budget/cards/values passam o ack.
- Arquivos: `onboarding_state.go`, `onboarding_workflow.go`, `onboarding_steps_objective.go`, `onboarding_steps_budget.go`, `onboarding_steps_cards.go`, `onboarding_steps_values.go`, `onboarding_steps_summary.go`, `onboarding_interpreter_render.go`
- Teste de regressao: `TestWorkflow_ObjectiveToBudget`, `TestWorkflow_BudgetToCards`, `TestWorkflow_CardLoop` (card-saved ack), `TestWorkflow_ValuesToSummary_ConfirmsEachAndAdvances`
- Validacao: `go test ./internal/agent/application/workflow/...` -> OK; `go build ./...` -> OK

- ID: BUG-102
- Severidade: major
- Origem: review finding F2 (RF-12)
- Estado: fixed
- Causa raiz: o step de categorias chamava `advance` em ambos os ramos (confirmado e nao confirmado) sem renderizar esclarecimento; o ramo de clarify era inalcancavel.
- Correcao: ramo `!confirmed` agora avanca com ack `RenderCategoriesClarify` (esclarece brevemente no tom oficial e segue, RF-12); ramo confirmado avanca com ack `RenderCategoriesConfirmed`.
- Arquivos: `onboarding_steps_categories.go`, `onboarding_interpreter_render.go`, `onboarding_workflow.go`
- Teste de regressao: `TestWorkflow_CategoriesNotConfirmed_ClarifiesAndProceeds`, `TestWorkflow_CategoriesToValues`
- Validacao: `go test ./internal/agent/application/workflow/...` -> OK

- ID: BUG-103
- Severidade: major
- Origem: review finding F3 (FC-02, RT-04, RF-26; techspec linhas 26/103/181)
- Estado: fixed
- Causa raiz: o interpreter parseava objetivo/renda/cartao/categorias/valores deterministicamente (regex + whitelists), nunca via LLM; "claro"/"com certeza" caiam em re-prompt e off-topic na ETAPA 2 era salvo como objetivo.
- Correcao: `ParseObjective/ParseBudget/ParseCards/ParseCategoriesConfirm/ParseValue` passam a ser **LLM-first com fallback deterministico** (`onboarding_interpreter_llm.go`), via `interpreter.Interpret` com structured output strict por etapa. Money: LLM retorna `amount_reais` (numero), convertido com `math.Round`; fallback `ParseMoneyCents`. Whitelist do handshake de boas-vindas ampliada. `Decide*` permanece puro.
- Arquivos: `onboarding_interpreter_llm.go` (novo), `onboarding_interpreter.go` (parsers viram fallbacks `*Deterministic`), `onboarding_steps_welcome.go`
- Teste de regressao: `TestParseObjective_LLMFirst_Save`, `TestParseObjective_LLMFirst_Clarify`, `TestParseObjective_FallbackOnLLMError`, `TestParseBudget_LLMFirst_AmountText`, `TestParseCategoriesConfirm_LLMFirst`, `TestParseCards_LLMFirst_AddAnotherOnLoopZero`; **real OpenRouter**: `TestOnboardingInterpreter_RealLLM_ParsesInputs` (9 subcasos PASS, gemini-2.5-flash-lite)
- Validacao: `go test ./internal/agent/infrastructure/onboarding/...` -> OK; `RUN_REAL_LLM=1 go test -tags integration -run TestOnboardingInterpreter_RealLLM_ParsesInputs ./internal/agent/e2e/` -> PASS (9/9)

- ID: BUG-104
- Severidade: minor
- Origem: review finding F4 (techspec R3; BUG-002 partial)
- Estado: skipped (diagnostico)
- Diagnostico: a remocao proposta de `Values` do snapshot foi **rejeitada por degradar fidelidade**. Os splits canonicos em `onboarding_sessions` sao armazenados em **basis points** (lossy); reconstruir centavos para o resumo (ETAPA 7, "valor + percentual") a partir de basis points introduziria erro de ±1 centavo no display oficial. `Values` e estado legitimo de orquestracao: acumulacao parcial durante a ETAPA 6 (uma categoria por turno) que DEVE viver no snapshot duravel para o resume, e fonte dos centavos exatos no resumo. O vetor de drift e nulo na pratica: todo `SplitsSaver.Save` e espelhado em `s.Values`. Mantido com justificativa.
- Validacao: `go test ./internal/agent/application/workflow/...` -> OK (comportamento preservado)

- ID: BUG-105
- Severidade: minor
- Origem: review finding F6 (Cap.08 ETAPA 6; RF-13)
- Estado: fixed
- Causa raiz: mismatch soma≠renda retornava `RenderRetry` generico ("Não entendi o valor...") sem explicar a restricao.
- Correcao: `RenderValuesMismatch(sumCents, incomeCents)` explica a soma atual vs orcamento e pede ajuste; usado no ramo de mismatch do values step.
- Arquivos: `onboarding_steps_values.go`, `onboarding_interpreter_render.go`, `onboarding_workflow.go` (`sumValues`)
- Teste de regressao: `TestWorkflow_ValuesMismatch_ExplainsAndStays`, `TestValuesStep_Mismatch_Clarify`
- Validacao: `go test ./internal/agent/application/workflow/...` -> OK

- ID: BUG-201
- Severidade: critical
- Origem: prova de persistencia Postgres (review finding; RF-28/RF-03 idempotencia)
- Estado: fixed
- Causa raiz: `processedEventRepository.IsProcessed` executava `SELECT EXISTS(...)` para `exists` mas retornava `return true, nil` **incondicionalmente**, ignorando o valor escaneado. Resultado: todo `event_id` (inclusive nunca processado) era reportado como ja processado -> consumidores descartariam eventos reais (idempotencia quebrada na direcao perigosa). O scan estatico anterior marcou BUG-016 como corrigido por ver `SELECT EXISTS`, sem notar o retorno literal. So apareceu ao rodar o teste de integracao contra Postgres real.
- Correcao: `return exists, nil`.
- Arquivos: `internal/agent/infrastructure/repositories/postgres/processed_event_repository.go`
- Teste de regressao: `TestProcessedEventRepositorySuite/TestIsProcessed_ReturnsFalseWhenEventDoesNotExist` (integration, Postgres real) — antes FALHAVA, agora passa
- Validacao: `go test -tags integration ./internal/agent/infrastructure/repositories/postgres/...` -> PASS (3/3)

- ID: BUG-202
- Severidade: major
- Origem: prova de persistencia Postgres (CI/branch integrity)
- Estado: fixed
- Causa raiz: as suites de integracao/E2E `internal/onboarding/e2e`, `internal/agent/application/services/onboarding_workflow_integration_test.go` e `internal/agent/infrastructure/messaging/database/consumers/onboarding_completed_integration_test.go` **nao compilavam** contra HEAD: stubs `*Interpreter` com `ParseValue (int64,bool,error)` (assinatura antiga), faltando os render methods novos, e `e2eStateChecker.Check`/arity de `NewOnboardingAgent` (HistoryGateway) desatualizados. A suite E2E nunca rodou verde no branch (contradiz o relatorio de orquestracao).
- Correcao: stubs atualizados para a interface atual (`ParsedValue`, 7 render methods, `Check(...)->(bool,OnboardingPhase,error)`, `e2eHistoryGateway`). Sweep `go test -tags integration` e `-tags e2e` em `./...` compila 100%.
- Arquivos: `internal/onboarding/e2e/feature_onboarding_conversational_steps_test.go`, `internal/onboarding/e2e/support_runtime_test.go`, `internal/agent/application/services/onboarding_workflow_integration_test.go`, `internal/agent/infrastructure/messaging/database/consumers/onboarding_completed_integration_test.go`
- Teste de regressao: suites passam contra Postgres real (ver Validacao)
- Validacao: `go test -tags e2e ./internal/onboarding/e2e/...` -> PASS; `go test -tags integration ./internal/agent/application/services/... -run Onboarding` -> PASS (8/8); consumers -> PASS

## Comandos Executados

- `go build ./...` -> OK
- `go vet ./...` -> OK
- `go test ./internal/agent/application/workflow/... ./internal/agent/application/services/... ./internal/agent/infrastructure/onboarding/... ./internal/onboarding/...` -> OK
- `go test -race -count=1` (mesmos pacotes) -> OK
- `golangci-lint run ./internal/agent/application/workflow/... ./internal/agent/infrastructure/onboarding/...` -> 0 issues
- `gofmt -l` (arquivos tocados) -> vazio
- `RUN_REAL_LLM=1 go test -tags integration -run TestOnboardingInterpreter_RealLLM_ParsesInputs ./internal/agent/e2e/` -> PASS (9/9 contra OpenRouter real)

## Riscos Residuais

- Cada turno de onboarding agora faz 1 chamada LLM de parse (latencia/custo); mitigado por fallback deterministico e `Temperature=0`.
- Testes de integracao/E2E com Docker (testcontainers) nao executados neste ambiente; o fluxo via kernel e coberto pelos testes de workflow com `fakeStore`.
- BUG-104 mantido por decisao de fidelidade (ver diagnostico).
- F5 (allowlist de deadcode / scaffolding V2 em `e2e/support_test.go`) fora do escopo deste ciclo.
