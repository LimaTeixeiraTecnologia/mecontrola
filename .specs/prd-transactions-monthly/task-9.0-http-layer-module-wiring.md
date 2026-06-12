# Tarefa 9.0: HTTP layer fino + `module.go` + wiring `cmd/api` + `cmd/worker` + feature flag

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa os 18 handlers HTTP finos, o router sob prefixo `/api/v1`, o `module.go` no padrão `BudgetsModule`, e o wiring nos dois binários: `cmd/api` registra **apenas** o router HTTP; `cmd/worker` registra **apenas** o consumer + 2 jobs. Feature flag `TransactionsConfig.Enabled` controla registro condicional. Idempotência via middleware com scope `transactions` e TTL 24h.

<requirements>
- 18 handlers em `infrastructure/http/server/handlers/`: 5 para Transaction, 6 para CardPurchase, 5 para RecurringTemplate, 2 para MonthlySummary. Todos finos: decode raw → use case → encode/MapError.
- `transactions_router.go` com `Register(r chi.Router)` montando rotas sob `/api/v1/...`; aplica `RequireUser` + `idempotency.Middleware{scope="transactions", TTL=24h}` (TTL cravado, audit fix).
- `responses.ErrorWithDetails` em todo handler para retornar `{message, code}` consistente (RF-44).
- Mapping de erros (RF-45): `validation_error` 400, `not_found` 404, `conflict`/`transaction_version_conflict`/`card_purchase_version_conflict` 409, `idempotency_conflict` 409, `card_lookup_failed` 502, `category_not_found` 404, default 500.
- `module.go` no padrão `BudgetsModule`: `NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule) (TransactionsModule, error)` retornando struct com campos `Router`, `MonthlySummaryRecomputeConsumer`, `RecurringMaterializerJob`, `MonthlySummaryReconcilerJob`, `EventHandlers []EventHandlerRegistration`. Quando `cfg.TransactionsConfig.Enabled == false`, todos os campos são zero value.
- `Boot(ctx)` do `CategoriesCache` chamado durante construção do módulo.
- `configs/config.go` ganha `TransactionsConfig` com: `Enabled bool`, `IdempotencyTTL time.Duration` (fixo 24h), `MonthlySummaryDebounceWindow time.Duration` (default 1500ms), `RecurringMaterializerCron string` (default `@daily`), `MonthlySummaryReconcilerCron string`, `MonthlySummaryReconcilerLookbackHours int` (default 48), `BrazilTimezone string` (default `America/Sao_Paulo`).
- `cmd/api/main.go`: instancia `TransactionsModule`; registra `Router.Register(...)` no chi global se `Enabled`. Não registra jobs nem consumer.
- `cmd/worker/main.go`: instancia `TransactionsModule`; registra consumer no `consumer.Registry` e jobs via `job.NewAdapter` no `WorkerManager`. Não registra router.
- Rollback "stop new writes + drain": flag off → novas chamadas a rotas retornam 404 (router não registrado em pod novo) ou continuam até pod reiniciar (escolha operacional); outbox dispatcher continua drenando.
- Estrutura do módulo segue R-ADAPTER-001.2: handlers finos.
</requirements>

## Subtarefas

- [ ] 9.1 18 handlers HTTP finos em `infrastructure/http/server/handlers/`.
- [ ] 9.2 `transactions_router.go` com `Register(chi.Router)` e middlewares.
- [ ] 9.3 `infrastructure/http/server/responses_mapper.go` (ou aproveitar `responses.MapError` global) para tradução erro → HTTP code.
- [ ] 9.4 `module.go` no padrão `BudgetsModule` com gating por `Enabled`.
- [ ] 9.5 `configs/config.go` ganha `TransactionsConfig` + parser Viper + validação no `Load()`.
- [ ] 9.6 Wiring em `cmd/api/main.go` (router apenas).
- [ ] 9.7 Wiring em `cmd/worker/main.go` (consumer + 2 jobs apenas).
- [ ] 9.8 Smoke tests E2E: `POST /api/v1/transactions` retorna 201 e materializa em DB + outbox; `Idempotency-Key` replay retorna mesmo body; flag off → 404 (em pod recém-iniciado).

## Detalhes de Implementação

Referência: techspec "Endpoints de API", "Visão Geral dos Componentes" / `infrastructure/http/server/`, "Contrato Inegociável de Adapters". RF-09, RF-10, RF-22, RF-30, RF-41, RF-44, RF-45, RF-46, RF-47, RT-07, RT-17.

## Critérios de Sucesso

- 18 handlers compilam e passam unit test individual (decode → use case mock → response).
- E2E test `POST /api/v1/transactions` em ambiente com Postgres + outbox dispatcher fake: 201 + linha em `transactions` + evento em `platform.outbox` na mesma TX.
- Replay com `Idempotency-Key` igual retorna o body cacheado (200/201) no segundo POST.
- Hash conflict de idempotência retorna 409 + `code=idempotency_conflict`.
- `cmd/api` rodando com `Enabled=false` não tem rotas `/api/v1/transactions*` registradas.
- `cmd/worker` rodando com `Enabled=false` não tem consumer nem jobs registrados.
- Zero comentários em `.go` de produção.
- `golangci-lint run ./...` limpo no escopo da task.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit test por handler (decode → mock usecase → encode).
- [ ] Unit test do `responses_mapper` cobrindo cada `code` para HTTP status.
- [ ] Integration test do router montado com middleware (smoke `POST /api/v1/transactions`).
- [ ] Integration test de feature flag off (módulo sem registro de rotas/consumer).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/infrastructure/http/server/handlers/*.go` (18 arquivos novos)
- `internal/transactions/infrastructure/http/server/transactions_router.go` (novo)
- `internal/transactions/infrastructure/http/server/responses_mapper.go` (novo se necessário)
- `internal/transactions/module.go` (novo)
- `configs/config.go` (modificado)
- `cmd/api/main.go` (modificado — wiring router)
- `cmd/worker/main.go` (modificado — wiring consumer + jobs)
- Testes `*_test.go` correspondentes.
