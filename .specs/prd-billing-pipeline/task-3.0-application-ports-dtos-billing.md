# Tarefa 3.0: Application ports + DTOs billing (interfaces consumer-defined)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Declarar todas as interfaces consumidas pelos use cases de billing em `internal/billing/application/interfaces/` (consumer-defined, conforme R6 do go-implementation) e materializar os DTOs de entrada/saída em `application/dtos/input/` e `application/dtos/output/`. Nenhuma implementação de IO entra aqui; apenas contratos e estruturas de dados.

<requirements>
- Interfaces no consumidor (billing application), não no produtor (infra).
- Interfaces devem aceitar `context.Context` na 1ª posição em toda fronteira de IO (R6).
- DTOs de input são structs imutáveis usadas como command objects pelos use cases.
- DTOs de output só são consumidos internamente; nada cross-module sai daqui (cross-module é via outbox events).
- Sem imports de `infrastructure`, `platform/postgres`, `chi`, `http`, SDK Kiwify ou JSON serialization.
</requirements>

## Subtarefas

- [ ] 3.1 `application/interfaces/subscription_repository.go` com métodos: `FindByOrderID`, `FindByUserID`, `UpsertByOrder`, `ExtendPeriod`, `ApplyTransition`.
- [ ] 3.2 `application/interfaces/processed_event_repository.go` com `MarkApplied(ctx, eventKey, trigger, recursoID, occurredAt)`; retorna sinal de "já aplicado" (sentinel error) em conflito.
- [ ] 3.3 `application/interfaces/kiwify_event_repository.go` com `Persist(ctx, envelopeID, trigger, rawBody, signatureStatus)`.
- [ ] 3.4 `application/interfaces/plan_repository.go` com `FindByKiwifyProductID`, `FindByCode`.
- [ ] 3.5 `application/interfaces/reconciliation_checkpoint_repository.go` com `Get(ctx, name)` e `Set(ctx, name, watermark)`.
- [ ] 3.6 `application/interfaces/kiwify_client.go` com `ListSalesUpdatedSince(ctx, windowStart, windowEnd, page)` e `GetSale(ctx, saleID)`.
- [ ] 3.7 `application/interfaces/notification_sender.go` com `NotifyTransition(ctx, payload)` — comportamento best-effort documentado em godoc; erro retornado é apenas para log.
- [ ] 3.8 `application/interfaces/subscription_event_publisher.go` com 5 métodos: `PublishActivated`, `PublishRenewed`, `PublishPastDue`, `PublishCanceled`, `PublishRefunded` — todos recebem `tx` (ou unit-of-work scope) e o agregado.
- [ ] 3.9 `application/interfaces/repository_factory.go` com factories para cada repositório acima (mesmo padrão de `identity`).
- [ ] 3.10 DTOs em `application/dtos/input/`: `ProcessSaleApprovedInput`, `ProcessSubscriptionRenewedInput`, `ProcessSubscriptionLateInput`, `ProcessSubscriptionCanceledInput`, `ProcessRefundOrChargebackInput`, `ReconcileSubscriptionsInput`.
- [ ] 3.11 DTOs em `application/dtos/output/`: `SubscriptionView` (uso interno) e `EntitlementDecisionInput` (DTO consumido cross-module via interface declarada em identity — verificar conflito de path e mover para identity em 10.0 se necessário; aqui só billing-side).

## Detalhes de Implementação

- Padrão de modulo: AGENTS.md §"Padrao Obrigatorio de Modulo" e §"Layout Obrigatorio por Modulo".
- Eventos emitidos por billing: techspec §6.6. Nomes de tipo: `billing.subscription.{activated,renewed,past_due,canceled,refunded}`.
- `NotificationSender` é stub no MVP (Q-01); a interface fica aqui, a implementação concreta WhatsApp não é desta tarefa nem do MVP.
- `RepositoryFactory` deve permitir injeção em UoW (compare com `internal/identity/application/interfaces/repository_factory.go`).

## Critérios de Sucesso

- `go build ./internal/billing/application/...` verde sem importar `infrastructure`.
- `go vet ./internal/billing/application/...` sem warnings.
- Verificação manual: grep em `internal/billing/application/interfaces/` por `database/sql`, `chi`, `http.`, `json.` deve retornar zero matches.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Verificação por `go vet` e `go build` sem dependências cíclicas.
- [ ] Teste de fronteira (grep + AST) confirmando ausência de imports proibidos.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/application/interfaces/{subscription_repository,processed_event_repository,kiwify_event_repository,plan_repository,reconciliation_checkpoint_repository,kiwify_client,notification_sender,subscription_event_publisher,repository_factory}.go`
- `internal/billing/application/dtos/input/*.go`
- `internal/billing/application/dtos/output/*.go`
- Referência: `internal/identity/application/interfaces/repository_factory.go` (padrão a seguir).
- Referência: techspec §4, §6.6.
