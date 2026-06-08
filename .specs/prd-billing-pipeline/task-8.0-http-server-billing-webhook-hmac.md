# Tarefa 8.0: HTTP server billing — router, handler, middleware HMAC e raw_body

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Expor `POST /api/v1/billing/webhooks/kiwify` substituindo o placeholder atual em `internal/billing/infrastructure/http/server/routes.go`. Implementar middleware `raw_body_buffer` (lê body uma vez e armazena em context), middleware `hmac_signature` (valida `X-Kiwify-Signature` em tempo constante com suporte a rotação via `KIWIFY_WEBHOOK_SECRET_NEXT`), handler que persiste `billing_kiwify_events` proativamente, faz dispatch por trigger para os use cases da Tarefa 5.0 e devolve códigos HTTP corretos (202/401/415/413/422). Router implementa `Register(chi.Router)` no mesmo padrão de `internal/identity/infrastructure/http/server/router.go`.

<requirements>
- Substituir `internal/billing/infrastructure/http/server/routes.go` (placeholder) e adicionar `router.go` + `handlers/` + `middleware/`.
- Body limite 256 KiB; reject `413` acima (techspec §8.2).
- Content-Type aceito: `application/json`; reject `415` se diverge.
- HMAC-SHA256 sobre `raw_body` (ADR-002); comparar em `hmac.Equal` (constant time).
- Suporte a rotação: aceitar `KIWIFY_WEBHOOK_SECRET` ∪ `KIWIFY_WEBHOOK_SECRET_NEXT`; `signature_status` registra `valid|invalid|rotated`.
- Persistência proativa em `billing_kiwify_events` antes do dispatch (auditoria).
- Dispatch por trigger: `order_approved`, `subscription_renewed`, `subscription_late`, `subscription_canceled`, `order_refunded`, `chargeback`. Triggers desconhecidos → 422 com `ErrUnknownTrigger`.
- `ErrFunnelTokenMissing` (RF-03) → 422.
- Provider hardcoded `kiwify` (RF-01); endpoint não aceita query param de provider.
- Resposta de sucesso ou no-op idempotente: `202 Accepted {"received":true}`.
</requirements>

## Subtarefas

- [ ] 8.1 `infrastructure/http/server/middleware/raw_body_buffer.go`: lê e armazena raw body no context, falha 413 se excede limite.
- [ ] 8.2 `infrastructure/http/server/middleware/hmac_signature.go`: computa expected HMAC, compara constant-time, suporta rotação, popula `signature_status` no context.
- [ ] 8.3 `infrastructure/http/server/handlers/kiwify_webhook_handler.go`: parse de envelope, persistência proativa em `kiwify_events`, dispatch por trigger, mapeamento de erros → HTTP status.
- [ ] 8.4 `infrastructure/http/server/router.go` (novo) com `WebhookRouter` implementando `Register(chi.Router)` registrando rota `POST /api/v1/billing/webhooks/kiwify` com a cadeia de middlewares.
- [ ] 8.5 Reescrever `infrastructure/http/server/routes.go` (atualmente placeholder vazio) para integrar com `WebhookRouter` ou removê-lo se redundante.
- [ ] 8.6 Unit tests: middleware (assinatura válida, inválida, rotacionada, comparação constant-time), handler (202/401/415/413/422), dispatch correto por trigger, dispatch de trigger desconhecido → 422.
- [ ] 8.7 Integration test webhook→use case→outbox (testcontainers): payload aprovado retorna 202 e gera 1 row em `billing_subscriptions`, 1 em `billing_processed_events`, 1 em `platform_outbox_events`.

## Detalhes de Implementação

- Fluxo detalhado em techspec §5.1 e §6.3.
- Validação HMAC em techspec §8.1; rotação via `KIWIFY_WEBHOOK_SECRET_NEXT`.
- Erro → HTTP mapping via `errors.Is` (R5.10) — não cair em 500 genérico para erros de negócio conhecidos.
- Persistência em `billing_kiwify_events` é fora da UoW do use case (auditoria proativa sobrevive a erro do use case).
- Sem PII em log (techspec §8.2 e §9.1); body bruto vive em `kiwify_events`, nunca em log.
- Padrão de router: comparar com `internal/identity/infrastructure/http/server/router.go` e `routes.go`.

## Critérios de Sucesso

- `go build ./internal/billing/infrastructure/http/server/...` verde.
- `go test -race -count=1 ./internal/billing/infrastructure/http/server/...` cobre todos os códigos HTTP esperados.
- Integ test confirma 202 + 1 sub + 1 processed_event + 1 outbox row.
- HMAC inválido retorna 401; HMAC com `secret_next` retorna 202 com `signature_status='rotated'` persistido.
- Payload sem token de funil retorna 422 sem criar subscription nem processed_event.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests do middleware HMAC (valida, rejeita, rotaciona).
- [ ] Unit tests do middleware `raw_body_buffer` (limite 256 KiB → 413).
- [ ] Unit tests do handler (202/401/415/413/422 + dispatch por trigger).
- [ ] Integration test webhook→use case→outbox.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/infrastructure/http/server/{router,routes}.go`
- `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_handler.go` + `_test.go`
- `internal/billing/infrastructure/http/server/middleware/{hmac_signature,raw_body_buffer}.go` + `_test.go`
- Referência: `internal/identity/infrastructure/http/server/router.go` e `routes.go`.
- Referência: techspec §5.1, §6.3, §8.
