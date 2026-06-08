# Tarefa 2.0: Criacao de checkout e consulta publica de estado do token

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o fluxo publico de criacao de sessao de checkout e o endpoint de consulta da thank-you page, incluindo rate limit por IP, CORS estrito, builder de URL Kiwify com `sck` e resposta booleana sem oracle de estado.

<requirements>
- Cobrir `RF-01`, `RF-02`, `RF-04`, `RF-05`, `RF-13`, `RF-17`.
- `POST /v1/onboarding/checkout` deve criar sempre token novo quando permitido, sem idempotency key.
- Rate limit de checkout deve ser 10/min/IP e nao criar token quando rejeitar.
- `GET /v1/onboarding/tokens/{token}/state` deve retornar apenas `ready_to_activate` e omitir motivo publico para estados invalidos.
- A resposta pronta deve incluir `wa_me_url` e `bot_number_display`; estados invalidos devem emitir metrica segmentada internamente.
- A execucao posterior deve carregar obrigatoriamente `go-implementation`, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 2.1 Implementar `CreateCheckoutSession` e `GetTokenState`.
- [ ] 2.2 Implementar `CheckoutURLBuilder` para Kiwify preservando query e validando host permitido.
- [ ] 2.3 Implementar middleware de rate limit com IP real por proxy confiavel.
- [ ] 2.4 Implementar handlers e router publico.
- [ ] 2.5 Emitir metricas de checkout criado, rate limit e invalid access da thank-you page.
- [ ] 2.6 Testar contratos JSON, CORS e ausencia de oracle publico.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 2, 5.1, 6.2, 6.5, 6.6, 8.4 e 8.6. O endpoint de state e contrato para a landing externa, nao implementacao da pagina Astro.

## Critérios de Sucesso

- Checkout retorna URL Kiwify com `sck=<token>` e persiste token `PENDING` com TTL de 7 dias.
- Plano desconhecido retorna erro claro sem criar token.
- Rate limit excedido retorna rejeicao clara e incrementa metrica.
- State endpoint nao distingue publicamente inexistente, pendente, expirado ou consumido.
- Testes demonstram que `wa_me_url` so aparece quando `ready_to_activate=true`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitarios de use cases.
- [ ] Testes de handler para JSON, CORS, rate limit e state sem oracle.
- [ ] `go test -race -count=1 ./internal/onboarding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/application/usecases/create_checkout_session.go`
- `internal/onboarding/application/usecases/get_token_state.go`
- `internal/onboarding/infrastructure/checkout/kiwify_url_builder.go`
- `internal/onboarding/infrastructure/http/server/`
- `configs/config.go`
