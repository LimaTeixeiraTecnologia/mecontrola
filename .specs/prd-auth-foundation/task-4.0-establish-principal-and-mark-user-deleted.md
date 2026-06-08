# Tarefa 4.0: EstablishPrincipal + TryFindActiveByWhatsApp + MarkUserDeleted publica user.deleted

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa o usecase transacional `EstablishPrincipal` que resolve `wa_id → user_id` e publica o evento outbox no mesmo UoW; estende `UserRepository` com `TryFindActiveByWhatsApp` evitando sentinel error no caminho quente; e garante que `MarkUserDeleted` publica `user.deleted` para que o consumer de 5.0 anonimize `auth_events`.

<requirements>
- RF-03: `EstablishPrincipal` retorna Principal ou `ErrUnknownUser`, publica `auth.principal_established`/`auth.unknown_user`, usa UoW.
- RF-26: `FindUserByWhatsApp` + outbox publish em **única transação curta** via `uow.UnitOfWork[T].Do`. Falha de outbox → rollback → erro tipado.
- RF-34: `MarkUserDeleted` publica `user.deleted{event_id, user_id, deleted_at}` com `Type="user.deleted"`, `AggregateType="user"`, `AggregateID=user_id`. Se publicação ausente, adicionar; se presente, validar payload.
</requirements>

## Subtarefas

- [ ] 4.1 Adicionar método `TryFindActiveByWhatsApp(ctx, wa) (entities.User, bool, error)` na interface `internal/identity/application/interfaces/user_repository.go` e na implementação Postgres.
- [ ] 4.2 Criar `internal/identity/application/dtos/input/establish_principal.go` (struct `EstablishPrincipalInput{WhatsAppNumber string}`).
- [ ] 4.3 Adicionar `ErrUnknownUser` em `internal/identity/application/errors.go` se ainda não existir.
- [ ] 4.4 Criar `internal/identity/application/usecases/establish_principal.go` seguindo techspec (UoW com `establishResult{Principal, Found}`; publica via `outbox.Publisher` dentro do callback).
- [ ] 4.5 Criar `internal/identity/application/usecases/establish_principal_test.go` com mocks (mockery): (a) usuário ativo; (b) inexistente; (c) PG falha; (d) outbox falha → rollback assertado.
- [ ] 4.6 Criar `internal/identity/application/usecases/establish_principal_integration_test.go` com PG real + outbox real cobrindo os 3 cenários principais.
- [ ] 4.7 Conforme PRE-04 de 1.0: se `MarkUserDeleted` ainda não publica `user.deleted`, adicionar publicação dentro do UoW atual. Payload e Type/AggregateType/AggregateID conforme techspec. Cobertura por integration test.
- [ ] 4.8 Atualizar `internal/identity/module.go` para registrar `EstablishPrincipal`.

## Detalhes de Implementação

Ver techspec `## Design de Implementação > Interfaces Chave > EstablishPrincipal` para esqueleto. Função `buildAuthEvent(uuid, *userID, kind, source, *reason) outbox.Event` é helper privado no mesmo arquivo. `outbox.Event.OccurredAt = time.Now().UTC()`. UUID gerado via `uuid.NewV7()` para alinhar com tabela `auth_events`.

## Critérios de Sucesso

- `Execute` retorna `Principal{UserID, SourceWhatsApp}` para user ativo + linha `platform_outbox_events` com Type `auth.principal_established` na mesma TX.
- `Execute` retorna `application.ErrUnknownUser` (sentinel comparável com `errors.Is`) para inexistente + linha outbox `auth.unknown_user`.
- Falha de `outbox.Publish` causa rollback observável (zero linha outbox + erro propagado).
- `MarkUserDeleted` publica `user.deleted` confirmado por integration test.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários com mockery cobrindo 4 cenários
- [ ] Testes de integração com PG + outbox real
- [ ] Integration test do `MarkUserDeleted` publicando `user.deleted`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/application/usecases/establish_principal.go` (criar) + `_test.go` + `_integration_test.go`
- `internal/identity/application/dtos/input/establish_principal.go` (criar)
- `internal/identity/application/errors.go` (atualizar — adicionar `ErrUnknownUser`)
- `internal/identity/application/interfaces/user_repository.go` (atualizar — `TryFindActiveByWhatsApp`)
- `internal/identity/infrastructure/repositories/postgres/user_repository.go` (atualizar — implementar)
- `internal/identity/application/usecases/mark_user_deleted.go` (atualizar — publicar `user.deleted` se ausente)
- `internal/identity/module.go` (atualizar — wiring de `EstablishPrincipal`)
