# Tarefa 7.0: Cliente Kiwify Public API (OAuth + rate limit + retry)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o client HTTP outbound para a Kiwify Public API em `internal/billing/infrastructure/http/client/kiwify/`, cobrindo OAuth (`POST /v1/oauth/token`), token cache em-memória, rate limiter local (100 req/min com burst), retry para 5xx/429 com backoff exponencial e os endpoints `GET /v1/sales` (lista paginada por `updated_at_start_date/end_date`) e `GET /v1/sales/{id}`. Todo request deve passar pelo wrapper `internal/platform/httpclient` e injetar `Authorization: Bearer` + `x-kiwify-account-id`.

<requirements>
- Toda chamada outbound via `internal/platform/httpclient` (sem `&http.Client{}` direto).
- Cache do `access_token` por `expires_in - KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN`; refresh transparente.
- Rate limiter usa `golang.org/x/time/rate` configurado por `KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN` / 60 rps e `KIWIFY_RATE_LIMIT_BURST`; bloqueia local antes de exceder.
- Retry: 5xx + 429 → backoff exponencial até `KIWIFY_HTTP_RETRY_MAX_ATTEMPTS`; demais 4xx → erro imediato.
- Header `x-kiwify-account-id` injetado em todo request via interceptor/middleware do client.
- Sem panic; erros tipados (`ErrKiwifyAuth`, `ErrKiwifyRateLimited`, `ErrKiwifyServer`, `ErrKiwifyBadRequest`) wrappados com `fmt.Errorf("billing/kiwify: %w", err)`.
- Implementa a interface `application/interfaces/kiwify_client.go` declarada em 3.0.
</requirements>

## Subtarefas

- [ ] 7.1 `client/kiwify/models.go`: structs para `OAuthTokenResponse`, `SalesListResponse`, `Sale`, `Tracking`, `Customer` conforme docs oficiais.
- [ ] 7.2 `client/kiwify/auth.go`: `tokenProvider` com cache + refresh; serialização form-urlencoded de `client_id`/`client_secret`.
- [ ] 7.3 `client/kiwify/ratelimit.go`: wrapper sobre `golang.org/x/time/rate` aplicado antes de cada request.
- [ ] 7.4 `client/kiwify/client.go`: `Client` implementando `ListSalesUpdatedSince(ctx, windowStart, windowEnd, page)` e `GetSale(ctx, saleID)`; encapsula retry, headers, tracing.
- [ ] 7.5 Tratamento de paginação: respeitar `page_size`/`page_number`/`hasMore` da response oficial.
- [ ] 7.6 Unit tests com `httptest.Server`: cache de token (1 OAuth call em 2 chamadas dentro da janela), rate limit bloqueante, retry 5xx, abort 4xx, header `x-kiwify-account-id` presente.

## Detalhes de Implementação

- Detalhes de contrato em techspec §8.3 e nos links oficiais citados (lista em §"Evidências oficiais Kiwify usadas").
- Wrapper `internal/platform/httpclient` provê timeout, base URL e retry opcional — não duplicar essas responsabilidades dentro do client.
- `KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN` default 600s (conforme `configs.KiwifyConfig` já existente).
- O client é consumido em duas frentes: (a) job de reconciliação (Tarefa 9.0) e (b) potencialmente reconciliação manual via CLI futura (fora do MVP).
- L-02 (`updated_at_start_date`): teste de unidade deve documentar comportamento esperado; janela de 15min é responsabilidade do job, não do client.

## Critérios de Sucesso

- `go build ./internal/billing/infrastructure/http/client/kiwify/...` verde.
- `go test -race -count=1 ./internal/billing/infrastructure/http/client/kiwify/...` cobre cache OAuth, rate limit, retry e headers.
- Em test de carga ligeiro (50 requests em paralelo) o rate limiter não permite > 100/min.
- Sem `&http.Client{}` direto no diff (grep verifica).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests com `httptest.Server` cobrindo OAuth cache, rate limit, retry, headers, paginação.
- [ ] Teste verificando ausência de `&http.Client{}` no pacote.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/infrastructure/http/client/kiwify/{client,auth,ratelimit,models}.go` + `_test.go`
- Referência: `internal/platform/httpclient/client.go` (wrapper a usar).
- Referência: techspec §8.3 e os endpoints oficiais Kiwify citados.
- Referência: `configs.KiwifyConfig` em `configs/config.go`.
