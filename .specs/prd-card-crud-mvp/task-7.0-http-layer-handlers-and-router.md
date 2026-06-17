# Tarefa 7.0: HTTP layer do `card` — handlers thin, router com chain, redact helper, spans/logs

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Entregar a camada HTTP do `card`: 6 handlers thin (POST/GET list/GET id/PUT/DELETE/GET invoices), `CardRouter` com chain de middlewares (`InjectPrincipalFromHeader → RequireUserWithO11y → idempotency.Middleware` para POST/PUT/DELETE), helper `redactCardLogFields` para garantir ausência de PII (`name`/`nickname`) em logs e spans, mapping completo de erros para HTTP. Handlers obedecem `R-ADAPTER-001.2` HARD: APENAS decode → usecase → encode; zero SQL, zero branching de domínio.

<requirements>
- 6 endpoints conforme RF-21..26 sob `/api/v1/cards`.
- Chain de middlewares conforme techspec §"Fluxo de Dados".
- `idempotency.Middleware("card", storage, 24*time.Hour, o11y)` aplicada APENAS em POST/PUT/DELETE.
- Handler thin: `R-ADAPTER-001.2` — sem SQL direto, sem branching de domínio. Apenas decode → usecase → encode/erro.
- Mapping de erros conforme techspec §"Endpoints de API":
  - decode error → 400 `invalid_payload`
  - `ErrInvalidCardName|Nickname|ClosingDay|DueDay|PurchaseDate` → 400 com `code` semântico
  - `auth.FromContext` ausente → 401 (via `RequireUser` canônico)
  - `idempotency.ErrMissing` → 400 `missing_idempotency_key`
  - `idempotency.ErrHashMismatch` → 409 `idempotency_conflict`
  - `ErrNicknameConflict` → 409 `nickname_in_use`
  - `ErrCardNotFound` → 404 `card_not_found`
  - default → 500 `internal_error`
- Resposta de erro via `responses.ErrorWithDetails` (devkit-go) com mensagens em pt-BR.
- Spans OTel `card.handler.<op>` em cada handler com atributos `user_id`, `card_id`, `outcome`.
- Logs estruturados via `redactCardLogFields(card)` (SEM `name`/`nickname`).
- Eventos canônicos: `card.create.started|completed|failed`, `card.list.served`, `card.update.completed`, `card.delete.completed`, `card.invoice_for.computed`, `card.idempotency.replay`, `card.auth.rejected`.
- Header `Location: /api/v1/cards/{id}` em 201 do POST.
- Tests httptest com mocks de use case cobrindo todos os status codes mapeados.
- Test de regressão PII (M-07): inspeciona output do `slog.Handler` capturado e garante ausência de `name`/`nickname` em qualquer log emitido.
- Zero comentários em `.go` produção.
</requirements>

## Subtarefas

- [ ] 7.1 `infrastructure/observability/redact.go` — `redactCardLogFields(card entities.Card) []observability.Attribute` retornando apenas `card_id`, `user_id`, `closing_day`, `due_day`, `created_at`.
- [ ] 7.2 `infrastructure/http/server/handlers/create.go` + tests.
- [ ] 7.3 `infrastructure/http/server/handlers/list.go` + tests (paginação, cursor inválido).
- [ ] 7.4 `infrastructure/http/server/handlers/get.go` + tests.
- [ ] 7.5 `infrastructure/http/server/handlers/update.go` + tests (sparse JSON: campos opcionais).
- [ ] 7.6 `infrastructure/http/server/handlers/delete.go` + tests.
- [ ] 7.7 `infrastructure/http/server/handlers/invoice_for.go` + tests (parsing de `?for=YYYY-MM-DD`).
- [ ] 7.8 `infrastructure/http/server/router.go` — `CardRouter` + `Register(r chi.Router)` + chain.
- [ ] 7.9 Test de regressão PII: configura `slog.Handler` que captura todos os records emitidos; executa todos os endpoints; assert que nenhum record contém substring `name` ou `nickname` em `key=value` pairs.
- [ ] 7.10 Test do `CardRouter` cobrindo header ausente → 401, idempotency-key ausente em POST → 400.

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Card — bounded context novo" + §"Endpoints de API". Espelhar estilo de `internal/billing/.../kiwify_webhook_handler.go` (handler thin com `responses.Error*`).

## Critérios de Sucesso

- `go test -race -count=1 -cover ./internal/card/infrastructure/http/...` ≥ 90% nos handlers.
- Gate `R-ADAPTER-001.2`: `grep -rn 'QueryContext\|ExecContext' internal/card/infrastructure/http/server/handlers/` retorna vazio.
- Gate `R-ADAPTER-001.2`: revisão manual confirma ausência de branching de domínio (comparações tipo `card.Status == "X"`).
- Test M-07 verde: 0 ocorrência de `name`/`nickname` em logs em todos os endpoints.
- Mensagens de erro em pt-BR.
- p99 estimado < 300 ms (validado em 10.0 com k6; aqui apenas micro-benchmark indicativo).

### Definition of Done

- [ ] 6 handlers + router + redact helper criados.
- [ ] Tests httptest cobrindo todos os status codes mapeados.
- [ ] Test M-07 (regressão PII) verde.
- [ ] `go vet` + `golangci-lint run` limpos.
- [ ] Gate de zero comentários verde para `internal/card/infrastructure/http/`.
- [ ] RF-21..26, RF-33..36 explicitamente apontados no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: 6 handlers + redact helper + router chain.
- [ ] Testes de integração: contract tests rodam em 8.0 (golden files); aqui valida-se chain via `httptest` apenas.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/card/infrastructure/observability/redact.go` (novo)
- `internal/card/infrastructure/observability/redact_test.go` (novo)
- `internal/card/infrastructure/http/server/handlers/*.go` (novo, 6 handlers + tests)
- `internal/card/infrastructure/http/server/router.go` (novo)
- `internal/card/infrastructure/http/server/router_test.go` (novo)
- Referência: `internal/identity/infrastructure/http/server/middleware/require_user.go`
