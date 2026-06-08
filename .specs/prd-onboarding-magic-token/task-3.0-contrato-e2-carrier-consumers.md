# Tarefa 3.0: Contrato E2, carrier sck e consumers de pagamento

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adaptar a fronteira Billing E2 para publicar eventos enriquecidos de pagamento aprovado, priorizar `tracking.sck` como carrier do magic token e criar consumers onboarding para token pago e pagamento aprovado sem token.

<requirements>
- Cobrir `RF-03`, `RF-13`, `RF-14`, `RF-16`, `RF-18`.
- `billing.subscription.activated` deve conter `subscription_id`, `funnel_token`, `plan_id`, `external_sale_id`, `customer_mobile_e164`, `customer_email`, `paid_at`, `period_end`.
- `billing.subscription.activated_without_token` deve ser publicado quando o pagamento aprovado vier sem token.
- Leitura de carrier deve priorizar `tracking.sck` e manter fallback legado conforme ADR-004.
- Consumers onboarding devem ser idempotentes e sinalizar `paid_without_token` em `support_signals`.
- A execucao posterior deve carregar obrigatoriamente `go-implementation`, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 3.1 Alterar extracao de funnel token em billing para priorizar `tracking.sck`.
- [ ] 3.2 Enriquecer entidade/payload de subscription activated com dados exigidos.
- [ ] 3.3 Criar publicacao de `billing.subscription.activated_without_token`.
- [ ] 3.4 Implementar `SubscriptionPaidConsumer` e `PaidWithoutTokenConsumer` em onboarding.
- [ ] 3.5 Implementar politica de retry/degradacao para token nao encontrado conforme techspec.
- [ ] 3.6 Cobrir mascaramento de PII em logs e metricas de paid/paid-without-token.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 2, 3, 5.3, 6.7, 6.10, 8.5, 9.1 e 9.2. Alteracoes em billing devem ser minimas e manter compatibilidade com o pipeline E2 existente.

## Critérios de Sucesso

- Eventos de billing carregam os campos necessarios para `MarkTokenPaid`.
- Pagamento aprovado sem token nunca e descartado silenciosamente.
- Consumer repetido para o mesmo token nao gera transicao indevida.
- Logs nao expoem email ou telefone em claro.
- Testes cobrem `sck`, fallback legado e ausencia de token.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitarios de extracao de carrier em billing.
- [ ] Testes de publisher para payload enriquecido e evento sem token.
- [ ] Testes de consumers onboarding com idempotencia e support signal.
- [ ] `go test -race -count=1 ./internal/billing/... ./internal/onboarding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/billing/application/usecases/funnel_token.go`
- `internal/billing/application/usecases/process_sale_approved.go`
- `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go`
- `internal/onboarding/infrastructure/messaging/database/consumers/`
- `internal/onboarding/application/usecases/mark_token_paid.go`
- `internal/onboarding/application/usecases/handle_paid_without_token.go`
