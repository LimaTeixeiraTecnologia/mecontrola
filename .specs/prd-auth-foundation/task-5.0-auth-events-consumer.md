# Tarefa 5.0: auth_events_consumer (projeção idempotente + anonimização via user.deleted)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa o consumer do outbox que projeta os eventos de auth para a tabela `auth_events` e anonimiza linhas quando recebe `user.deleted`. Idempotência por `event_id` é o contrato obrigatório (padrão CLAUDE.md).

<requirements>
- RF-10: consumer processa `auth.principal_established`, `auth.failed{reason}`, `auth.unknown_user` e insere em `auth_events`; idempotência por `event_id` via `ON CONFLICT (id) DO NOTHING`.
- RF-11: consumer processa `user.deleted` e executa `UPDATE auth_events SET user_id = NULL WHERE user_id = $1`. Operação idempotente.
</requirements>

## Subtarefas

- [ ] 5.1 Criar `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go` seguindo padrão de `subscription_event_projector.go`.
- [ ] 5.2 Implementar handler para 3 tipos de evento `auth.*`: decode JSON do payload outbox → construir `AuthEvent` → `repository.Insert(ctx, ev)`.
- [ ] 5.3 Implementar handler para `user.deleted`: decode payload → `repository.AnonymizeByUserID(ctx, userID)`.
- [ ] 5.4 Registrar `EventHandlerRegistration` em `internal/identity/module.go` para os 4 tipos.
- [ ] 5.5 Criar `auth_events_consumer_test.go` com mocks (mockery): decode válido, decode inválido (payload corrupto), repositório falha (erro propagado).
- [ ] 5.6 Criar `auth_events_consumer_integration_test.go`: publica via outbox real → consumer processa → linha em `auth_events` ✓; reprocessa mesmo `event_id` → 1 linha; processa `user.deleted` → linhas anonimizadas; reprocessa mesmo `user.deleted` → no-op.

## Detalhes de Implementação

Ver techspec `## Pontos de Integração > Outbox`. Idempotência no `Insert` é `INSERT ... ON CONFLICT (id) DO NOTHING` (não usa `event_id` separado — `auth_events.id` **é** o event_id, conforme techspec `AggregateID=event_id`).

## Critérios de Sucesso

- Consumer processa 4 tipos de evento corretamente.
- Reprocessamento de mesmo `event_id` é idempotente (verificado por count após 2 invocações).
- `user.deleted` anonimiza todas as linhas do user + preserva outros campos (`occurred_at`, `kind`, `source`).
- Payload corrupto → erro logado + métrica `auth_events_consumer_decode_failed_total`; não trava o dispatcher do outbox (segue para próximo evento).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários com mockery (decode + repo)
- [ ] Testes de integração end-to-end via outbox real + PG real

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer.go` (criar) + `_test.go` + `_integration_test.go`
- `internal/identity/module.go` (atualizar — registrar `EventHandlerRegistration`)
