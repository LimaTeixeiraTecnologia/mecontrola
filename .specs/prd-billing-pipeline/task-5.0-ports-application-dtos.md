# Tarefa 5.0: Ports application + DTOs input/output

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Declarar interfaces (ports) em `internal/billing/application/interfaces/` consumidas pelos use cases e implementadas pela infrastructure: `SubscriptionRepository` (com `FindActiveByUserIDForUpdate` para lock pessimista — ADR-012), `WebhookEventRepository`, `BillingProvider`, `EntitlementCache`, `IDGenerator`, `UserResolver` (segregação cross-module). Criar DTOs de input/output em `application/dtos/{input,output}/`. Sem implementação concreta nesta task.

<requirements>
- 6 interfaces em `application/interfaces/`
- DTOs (não exportam tipos de domínio diretamente onde input/output deve ser desacoplado)
- `SubscriptionRepository` inclui `FindActiveByUserIDForUpdate` (ADR-012 pessimist lock)
- `WebhookEventRepository.InsertIfNew` retorna `(bool, error)` distinguindo conflito (false, nil) de erro real
- `WebhookEventRepository.RecordApplication` mesmo padrão (idempotência ADR-009)
- `BillingProvider.VerifySignature(payload []byte, headers map[string]string) error` — sem parâmetro secret
- `UserResolver` é wrapper segregado de `identity.UserRepository.UpsertByWhatsAppNumber` (techspec §UserResolver)
- Documentação concisa nos godocs apenas onde regra/contrato não é óbvio do nome (sem comentário trivial)
- Sem implementação — apenas interfaces e DTOs
</requirements>

## Subtarefas

- [ ] 5.1 `application/interfaces/subscription_repository.go` com método `Upsert`, `FindActiveByUserID`, `FindActiveByUserIDForUpdate`, `FindByExternalID`, `ListByStatusInBatch(ctx, statuses, cursorCreatedAt, cursorID, limit)` (cursor composto para paginação estável).
- [ ] 5.2 `application/interfaces/webhook_event_repository.go` com `InsertIfNew`, `FindRawPayload(ctx, id) (json.RawMessage, error)`, `MarkProcessed`, `RecordApplication(ctx, eventID, subID, at) (bool, error)`, `ListPendingAnonymization`, `Anonymize`.
- [ ] 5.3 `application/interfaces/billing_provider.go` com `VerifySignature(payload []byte, headers map[string]string) error`, `ParseEvent(payload []byte) (services.CanonicalEvent, error)`, `FetchSubscription(ctx, externalSubscriptionID) (services.CanonicalSubscription, error)`.
- [ ] 5.4 `application/interfaces/entitlement_cache.go` com `Get(userID) (output.EntitlementDecision, bool)`, `Set(userID, decision, ttl)`, `Invalidate(userID)`.
- [ ] 5.5 `application/interfaces/id_generator.go` com `NewID() string` retornando UUID v4.
- [ ] 5.6 `application/interfaces/user_resolver.go` com `UpsertByWhatsAppNumber(ctx, number) (*identityentities.User, error)`.
- [ ] 5.7 DTOs input em `application/dtos/input/`: `IngestWebhookInput{RawBody []byte; Headers map[string]string; ReceivedAt time.Time}` com método `HeadersJSON() json.RawMessage`, `ProcessEventInput{WebhookEventID}`, `CheckEntitlementInput{UserID}`, `AnonymizeInput{OlderThan time.Time; BatchSize int}`.
- [ ] 5.8 DTOs output em `application/dtos/output/`: `IngestWebhookResult{Duplicate bool; WebhookEventID}`, `EntitlementDecision{Status; Reason string; SubscriptionID; ExpiresAt time.Time}`, `ReconciliationReport{Inspected, Diverged, Synced int}`, `AnonymizationReport{Processed, Errors int}`.
- [ ] 5.9 `application/doc.go` documentando o pacote (sem comentário trivial — apenas listar responsabilidades).

## Detalhes de Implementação

Ver techspec §Interfaces Chave. Layout segue convenção de `internal/identity/application/interfaces/`. Cada interface em arquivo dedicado. Tipos importados:
- `services.CanonicalEvent` / `services.CanonicalSubscription` de `internal/billing/domain/services` (task 3.0)
- `entities.Subscription` / `entities.SubscriptionID` de `internal/billing/domain/entities` (task 4.0)
- `valueobjects.*` de `internal/billing/domain/valueobjects` (task 2.0)
- `identityentities.User`, `identityentities.UserID`, `identityvo.WhatsAppNumber` de `internal/identity/...` (E1)

## Critérios de Sucesso

- `go build ./internal/billing/application/...` compila sem erro.
- `golangci-lint run ./internal/billing/application/...` verde — em especial `depguard` confirma que `application/` não importa `infrastructure/`.
- Cada interface tem ≥ 1 método; sem interfaces vazias.
- DTOs são structs simples sem dependência circular.
- `mockery --config .mockery.yml --dry-run` (após task 6.0 atualizar config) gera todos os 6 mocks.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Sem testes unitários esperados (interfaces declarativas) — validar via build.
- [ ] Lint `golangci-lint run ./internal/billing/application/...` deve retornar zero issues.
- [ ] Verificação manual: cada interface tem godoc curto (1-2 linhas) apenas para gatilho não-óbvio (e.g., "RecordApplication retorna (false, nil) em conflict").

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/application/doc.go` (novo)
- `internal/billing/application/interfaces/subscription_repository.go` (novo)
- `internal/billing/application/interfaces/webhook_event_repository.go` (novo)
- `internal/billing/application/interfaces/billing_provider.go` (novo)
- `internal/billing/application/interfaces/entitlement_cache.go` (novo)
- `internal/billing/application/interfaces/id_generator.go` (novo)
- `internal/billing/application/interfaces/user_resolver.go` (novo)
- `internal/billing/application/dtos/input/*.go` (novos: 4 arquivos)
- `internal/billing/application/dtos/output/*.go` (novos: 4 arquivos)
- Depende de: task 2.0 (VOs), task 3.0 (services), task 4.0 (entities)
- Importa: `internal/identity/{domain,application/interfaces}` — entregues por E1
