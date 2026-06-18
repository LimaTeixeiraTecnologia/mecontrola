# Plano E2E — Módulo `internal/identity`

**Data:** 2026-06-18
**Escopo:** Cobertura 100% com confiança operacional — pirâmide Unit + Integration + E2E godog
**Skills obrigatórias:** `go-implementation`, `agent-governance`

---

## 1. Inventário do Módulo (descoberto)

### HTTP Endpoints

| Método | Rota | Handler | Status Esperados |
|--------|------|---------|-----------------|
| POST | `/api/v1/identity/users` | `UpsertUserByWhatsAppHandler` | 200 OK, 400 Bad Request, 409 Conflict, 500 Internal |

**Middlewares:**
- `InjectPrincipalFromHeader` — extrai `X-User-ID`, injeta `auth.Principal` no contexto
- `RequireUser` — retorna 401 se Principal ausente
- `RequireGatewayAuth` — valida HMAC `X-Gateway-Auth` + janela de timestamp; retorna 401 em falha

### Use Cases

| Use Case | Arquivo | Portas Consumidas |
|----------|---------|-------------------|
| `UpsertUserByWhatsApp` | `upsert_user_by_whatsapp.go` | `UserRepository`, `RepositoryFactory`, UoW |
| `FindUserByID` | `find_user_by_id.go` | `UserRepository` |
| `FindUserByWhatsApp` | `find_user_by_whatsapp.go` | `UserRepository` |
| `EstablishPrincipal` | `establish_principal.go` | `UserIdentityRepository`, `UserRepository`, `RepositoryFactory`, `outbox.Publisher`, UoW |
| `ResolvePrincipalByIdentity` | `resolve_principal_by_identity.go` | `UserIdentityRepository`, `RepositoryFactory`, UoW |
| `MarkUserDeleted` | `mark_user_deleted.go` | `UserRepository`, `RepositoryFactory`, `outbox.Publisher`, UoW |
| `LinkChannelToUser` | `link_channel_to_user.go` | `UserIdentityRepository`, `RepositoryFactory`, UoW |
| `RecordGatewayAuthFailure` | `record_gateway_auth_failure.go` | `outbox.Publisher` |
| `ProjectAuthEvent` | `project_auth_event.go` | `AuthEventsRepository` |
| `ProjectSubscriptionEvent` | `project_subscription_event.go` | `EntitlementRepository`, `SubscriptionProjectionReader` |
| `CleanupAuthEvents` | `cleanup_auth_events.go` | `AuthEventsRepository` |
| `AnonymizeUserAuthEvents` | `anonymize_user_auth_events.go` | `AuthEventsRepository` |
| `ResolvePreferredChannel` | `resolve_preferred_channel.go` | `UserIdentityRepository` |

### Eventos de Domínio (Outbox)

| Tipo | Aggregate | Aggregate ID | Campos do Payload | Publicado por |
|------|-----------|-------------|------------------|---------------|
| `auth.principal_established` | `auth_event` | userID | `event_id, user_id, kind, source, occurred_at, request_id, client_ip` | `EstablishPrincipal` |
| `auth.unknown_user` | `auth_event` | eventID | `event_id, user_id:null, kind, source, occurred_at` | `EstablishPrincipal` |
| `auth.failed` | `auth_event` | userID | `event_id, user_id, kind, source, reason, occurred_at, request_id, client_ip` | `RecordGatewayAuthFailure` |
| `user.deleted` | `user` | userID | `event_id, user_id, deleted_at` | `MarkUserDeleted` |

### Consumers

| Consumer | Struct | Eventos Consumidos | Idempotência |
|----------|--------|--------------------|-------------|
| `AuthEventsConsumer` | `auth_events_consumer.go` | `auth.*`, `user.deleted` | `ON CONFLICT (id) DO NOTHING` em `auth_events` |
| `SubscriptionEventProjector` | `subscription_event_projector.go` | `billing.subscription.*` (5 tipos) | Upsert por `user_id` em `identity_entitlements` |
| `SubscriptionBoundProjector` | `subscription_bound_projector.go` | `onboarding.subscription_bound` | Upsert por `subscription_id` via `identity_entitlements_pending` |

### Jobs

| Job | Struct | Schedule | Idempotência |
|-----|--------|----------|-------------|
| `AuthEventsHousekeepingJob` | `auth_events_housekeeping_job.go` | `@monthly` (config) | Batch por `cutoff` — determinístico por tempo |

### Tabelas Tocadas

| Tabela | Operações |
|--------|-----------|
| `mecontrola.users` | INSERT, UPDATE (upsert, soft-delete, reanimate) |
| `mecontrola.user_identities` | INSERT (link), UPDATE (unlink) |
| `mecontrola.auth_events` | INSERT (ON CONFLICT DO NOTHING), UPDATE (anonymize), DELETE (housekeeping) |
| `mecontrola.identity_entitlements` | UPSERT por user_id |
| `mecontrola.identity_entitlements_pending` | UPSERT por subscription_id |
| `mecontrola.user_whatsapp_history` | INSERT |
| `mecontrola.outbox_events` | INSERT via `outbox.Publisher` |

---

## 2. Gap Analysis — Estado Atual vs Cobertura Necessária

### O que já existe (54 arquivos de teste)

| Camada | Cobertura Atual |
|--------|----------------|
| Domínio (unit) | ✅ Completo — todas as entities, VOs, services, decisors |
| Use Cases (unit) | ✅ Completo — todos 13 use cases com mocks |
| HTTP Handlers (unit) | ✅ `upsert_user_by_whatsapp_handler_test.go` |
| Middlewares (unit + integration) | ✅ Todos os 3 middlewares |
| UserRepository (unit + integration) | ✅ `user_repository_test.go` + `user_repository_integration_test.go` |
| AuthEventsRepository (unit + integration) | ✅ Ambos |
| EntitlementRepository (integration) | ✅ `entitlement_repository_integration_test.go` |
| AuthEventsConsumer (unit + integration) | ✅ Ambos |
| SubscriptionEventProjector (unit) | ✅ `subscription_event_projector_test.go` |
| Job Housekeeping (unit + integration) | ✅ Ambos |
| EstablishPrincipal (integration) | ✅ `establish_principal_integration_test.go` |
| MarkUserDeleted (integration) | ✅ `mark_user_deleted_integration_test.go` |
| E2E godog f04–f08 | ✅ 24 cenários implementados com verificação de banco |

### Gaps Confirmados — O que Falta

| Gap | Tipo | Arquivo a Criar | Prioridade |
|-----|------|-----------------|------------|
| G1 | Integration | `user_identity_repository_integration_test.go` | Alta |
| G2 | Integration | `subscription_event_projector_integration_test.go` | Alta |
| G3 | Unit + Integration | `subscription_bound_projector_test.go` + `subscription_bound_projector_integration_test.go` | Alta |
| G4 | Integration | `resolve_principal_by_identity_integration_test.go` | Alta |
| G5 | Integration | `link_channel_to_user_integration_test.go` | Média |
| G6 | Integration | `project_auth_event_integration_test.go` | Média |
| G7 | Integration | `project_subscription_event_integration_test.go` | Média |
| G8 | E2E feature | Cenários de idempotência (consumer reprocessamento) | Alta |
| G9 | E2E feature | `ResolvePrincipalByIdentity` (canal Telegram) | Alta |
| G10 | E2E feature | `SubscriptionBoundProjector` (`onboarding.subscription_bound`) | Média |
| G11 | E2E feature | Housekeeping job E2E (deletar eventos antigos) | Média |
| G12 | E2E feature | Gateway auth 401 via HTTP (middleware integration) | Média |

---

## 3. Novos Arquivos de Integração — Especificação

### G1 — `user_identity_repository_integration_test.go`

**Localização:** `internal/identity/infrastructure/repositories/postgres/`

**Build tag:** `//go:build integration`

**Cenários obrigatórios:**
1. `Insert` → `TryFindActive` retorna a identidade com todos os campos corretos (channel, externalID, userID, verifiedAt)
2. `Insert` duplicado (mesmo channel+externalID ativo) → erro (unique index)
3. `Unlink` → `TryFindActive` retorna `found=false`; linha permanece com `unlinked_at IS NOT NULL`
4. `ListByUser` — retorna apenas identidades ativas (sem as desvinculadas)
5. `FindByUserAndChannel` — encontra identidade pelo par (userID, channel)

**Verificação de banco obrigatória:**
```go
func countActiveIdentities(ctx context.Context, db *sqlx.DB, userID uuid.UUID) int {
    var n int
    _ = db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM mecontrola.user_identities WHERE user_id = $1 AND unlinked_at IS NULL`,
        userID).Scan(&n)
    return n
}
```

---

### G2 — `subscription_event_projector_integration_test.go`

**Localização:** `internal/identity/infrastructure/messaging/database/consumers/`

**Build tag:** `//go:build integration`

**Cenários obrigatórios:**
1. `billing.subscription.activated` → linha em `identity_entitlements` com `status=ACTIVE`
2. `billing.subscription.past_due` → linha atualizada com `status=PAST_DUE`
3. `billing.subscription.canceled` → linha com `status=EXPIRED`
4. `billing.subscription.refunded` → linha com `status=REFUNDED`
5. Reprocessar mesmo evento (mesmo `subscription_id`) → sem duplicata; COUNT permanece 1

**Verificação de banco:**
```go
func assertEntitlementStatus(ctx context.Context, db *sqlx.DB, userID uuid.UUID, expected string) error {
    var got string
    err := db.QueryRowContext(ctx,
        `SELECT status FROM mecontrola.identity_entitlements WHERE user_id = $1`, userID).Scan(&got)
    // assert got == expected
}
```

---

### G3 — `subscription_bound_projector_test.go` + `subscription_bound_projector_integration_test.go`

**Localização:** `internal/identity/infrastructure/messaging/database/consumers/`

**Unit (mock):**
1. Evento `onboarding.subscription_bound` com payload válido → chama use case `ProjectSubscriptionEvent`
2. Payload inválido → retorna erro sem chamar use case

**Integration:**
1. `onboarding.subscription_bound` → linha em `identity_entitlements_pending` com `subscription_id` correto
2. Reprocessar mesmo evento → `COUNT(*) = 1` (upsert idempotente)

---

### G4 — `resolve_principal_by_identity_integration_test.go`

**Localização:** `internal/identity/application/usecases/`

**Build tag:** `//go:build integration`

**Cenários:**
1. Canal existente (Telegram) → retorna Principal com `UserID` correto
2. Canal desconhecido → retorna erro `ErrUnknownUser` ou similar
3. Identidade desvinculada (`unlinked_at IS NOT NULL`) → retorna erro
4. Channel mismatch → retorna erro de resolução

---

### G5 — `link_channel_to_user_integration_test.go`

**Cenários:**
1. Vincular canal novo → linha em `user_identities` com `unlinked_at IS NULL`
2. Vincular mesmo canal+external_id ao mesmo usuário → `AlreadyLinked=true`, sem nova linha
3. Vincular external_id já usado por outro usuário → erro (unique constraint)

---

### G6 — `project_auth_event_integration_test.go`

**Cenários:**
1. Projetar `auth.principal_established` → linha em `auth_events` com `kind=principal_established` e `user_id` correto
2. Reprocessar mesmo `event_id` → `COUNT(*) = 1` (ON CONFLICT DO NOTHING)
3. Projetar `auth.unknown_user` → linha com `user_id IS NULL`
4. Projetar `auth.failed` com reason válido → linha com `reason` correto

---

### G7 — `project_subscription_event_integration_test.go`

**Cenários (via use case direto, com DB real):**
1. Upsert de entitlement ACTIVE
2. Transição ACTIVE → PAST_DUE
3. Transição para EXPIRED
4. Idempotência por `subscription_id`

---

## 4. Novos Cenários E2E Godog

### Feature f09 — Idempotência de Consumers

**Arquivo:** `internal/e2e/features/f09_identity_idempotencia.feature`

```gherkin
# language: pt
Funcionalidade: Idempotência no consumo de eventos de identity

  Cenário: Reprocessar auth.principal_established com mesmo event_id não duplica auth_event
    Dado que existe um usuário com whatsapp "+5511988880050" cadastrado no sistema
    Quando o evento de auth "auth.principal_established" é projetado para o usuário
    E o mesmo evento de auth "auth.principal_established" é projetado novamente para o usuário
    Então deve existir exatamente 1 auth_event do tipo "principal_established" para o usuário

  Cenário: Reprocessar billing.subscription.activated com mesmo subscription_id não duplica entitlement
    Dado que existe um usuário com whatsapp "+5511988880051" cadastrado no sistema
    Quando o evento de assinatura "billing.subscription.activated" é projetado para o usuário com status "ACTIVE"
    E o evento de assinatura "billing.subscription.activated" é projetado novamente para o usuário com status "ACTIVE"
    Então o entitlement do usuário deve estar salvo no banco com status "ACTIVE"
    E deve existir exatamente 1 entitlement para o usuário
```

**Novos steps necessários:**
```go
// regex PT-BR, método em inglês
sc.Step(`^o mesmo evento de auth "([^"]*)" é projetado novamente para o usuário$`, e.replayLastAuthEvent)
sc.Step(`^deve existir exatamente 1 auth_event do tipo "([^"]*)" para o usuário$`, e.assertExactlyOneAuthEventKind)
sc.Step(`^deve existir exatamente 1 entitlement para o usuário$`, e.assertExactlyOneEntitlement)
```

---

### Feature f10 — ResolvePrincipalByIdentity (Telegram)

**Arquivo:** `internal/e2e/features/f10_identity_resolve_principal.feature`

```gherkin
# language: pt
Funcionalidade: Resolução de principal via canal de identidade (Telegram)

  Cenário: Resolver principal via canal Telegram vinculado
    Dado que existe um usuário com whatsapp "+5511988880060" cadastrado no sistema
    E o canal "telegram" com external_id "tg123456" é vinculado ao usuário
    Quando o principal é resolvido para o canal "telegram" e external_id "tg123456"
    Então o principal resolvido deve ter o UserID do usuário cadastrado

  Cenário: Canal desconhecido retorna erro de identidade não encontrada
    Quando o principal é resolvido para o canal "telegram" e external_id "tg_nao_existe"
    Então a resolução de principal deve retornar erro de usuário desconhecido

  Cenário: Canal desvinculado retorna erro na resolução
    Dado que existe um usuário com whatsapp "+5511988880061" cadastrado no sistema
    E o canal "telegram" com external_id "tg999999" é vinculado ao usuário
    E o canal "telegram" é desvinculado do usuário
    Quando o principal é resolvido para o canal "telegram" e external_id "tg999999"
    Então a resolução de principal deve retornar erro de identidade inativa
```

**Novos steps necessários:**
```go
sc.Step(`^o principal é resolvido para o canal "([^"]*)" e external_id "([^"]*)"$`, e.resolvePrincipalByIdentity)
sc.Step(`^o principal resolvido deve ter o UserID do usuário cadastrado$`, e.assertResolvedPrincipalMatchesUser)
sc.Step(`^a resolução de principal deve retornar erro de usuário desconhecido$`, e.assertPrincipalResolutionUnknownUser)
sc.Step(`^a resolução de principal deve retornar erro de identidade inativa$`, e.assertPrincipalResolutionInactiveIdentity)
sc.Step(`^o canal "([^"]*)" é desvinculado do usuário$`, e.unlinkChannelFromUser)
```

---

### Feature f11 — SubscriptionBoundProjector

**Arquivo:** `internal/e2e/features/f11_identity_subscription_bound.feature`

```gherkin
# language: pt
Funcionalidade: Projeção de vínculo de assinatura via evento onboarding

  Cenário: Evento onboarding.subscription_bound salva pending entitlement no banco
    Dado que existe um usuário com whatsapp "+5511988880070" cadastrado no sistema
    Quando o evento "onboarding.subscription_bound" é projetado com funnel_token "funnel-abc"
    Então um entitlement pendente deve existir no banco com funnel_token "funnel-abc"

  Cenário: Reprocessar onboarding.subscription_bound é idempotente
    Dado que existe um usuário com whatsapp "+5511988880071" cadastrado no sistema
    Quando o evento "onboarding.subscription_bound" é projetado com funnel_token "funnel-xyz"
    E o mesmo evento "onboarding.subscription_bound" é projetado novamente com funnel_token "funnel-xyz"
    Então deve existir exatamente 1 entitlement pendente com funnel_token "funnel-xyz"
```

**Novos steps necessários:**
```go
sc.Step(`^o evento "([^"]*)" é projetado com funnel_token "([^"]*)"$`, e.projectSubscriptionBoundEvent)
sc.Step(`^o mesmo evento "([^"]*)" é projetado novamente com funnel_token "([^"]*)"$`, e.replaySubscriptionBoundEvent)
sc.Step(`^um entitlement pendente deve existir no banco com funnel_token "([^"]*)"$`, e.assertPendingEntitlementWithFunnelToken)
sc.Step(`^deve existir exatamente 1 entitlement pendente com funnel_token "([^"]*)"$`, e.assertExactlyOnePendingEntitlement)
```

---

### Feature f12 — Housekeeping Job

**Arquivo:** `internal/e2e/features/f12_identity_housekeeping.feature`

```gherkin
# language: pt
Funcionalidade: Limpeza periódica de auth_events antigos via job

  Cenário: Job remove auth_events mais antigos que o período de retenção
    Dado que existe um auth_event com occurred_at superior ao período de retenção
    Quando o job de housekeeping de auth_events é executado
    Então o auth_event antigo não deve mais existir no banco

  Cenário: Job preserva auth_events dentro do período de retenção
    Dado que existe um auth_event recente no banco
    Quando o job de housekeeping de auth_events é executado
    Então o auth_event recente deve continuar existindo no banco

  Cenário: Executar o job duas vezes é idempotente
    Dado que existe um auth_event com occurred_at superior ao período de retenção
    Quando o job de housekeeping de auth_events é executado
    E o job de housekeeping de auth_events é executado novamente
    Então nenhum erro deve ocorrer na segunda execução
```

**Novos steps necessários:**
```go
sc.Step(`^existe um auth_event com occurred_at superior ao período de retenção$`, e.seedExpiredAuthEvent)
sc.Step(`^existe um auth_event recente no banco$`, e.seedRecentAuthEvent)
sc.Step(`^o job de housekeeping de auth_events é executado$`, e.runAuthEventsHousekeepingJob)
sc.Step(`^o auth_event antigo não deve mais existir no banco$`, e.assertExpiredAuthEventDeleted)
sc.Step(`^o auth_event recente deve continuar existindo no banco$`, e.assertRecentAuthEventPreserved)
sc.Step(`^nenhum erro deve ocorrer na segunda execução$`, e.assertNoErrorOnSecondRun)
```

---

### Feature f13 — Gateway Auth 401 (HTTP)

**Arquivo:** `internal/e2e/features/f13_identity_gateway_auth.feature`

```gherkin
# language: pt
Funcionalidade: Autenticação de gateway HMAC nas rotas protegidas

  Cenário: Requisição sem header X-Gateway-Auth retorna 401
    Quando uma requisição é feita sem o header de autenticação de gateway
    Então a resposta HTTP deve ter status 401

  Cenário: Requisição com assinatura inválida retorna 401
    Quando uma requisição é feita com assinatura de gateway inválida
    Então a resposta HTTP deve ter status 401

  Cenário: Requisição com timestamp fora da janela retorna 401
    Quando uma requisição é feita com timestamp de gateway expirado
    Então a resposta HTTP deve ter status 401

  Cenário: Requisição com assinatura válida passa na autenticação
    Quando uma requisição é feita com assinatura de gateway válida
    Então a resposta HTTP deve ter status diferente de 401
```

---

## 5. Estrutura de Pastas

Os cenários E2E de identity ficam no pacote compartilhado `internal/e2e/` (padrão do repositório).

```
internal/e2e/
  features/
    f04_identity_cadastro_usuario.feature     ✅ existe (8 cenários)
    f05_identity_vinculacao_canal.feature     ✅ existe (4 cenários)
    f06_identity_entitlements.feature         ✅ existe (4 cenários)
    f07_identity_eventos_outbox.feature       ✅ existe (4 cenários)
    f08_identity_auth_events_consumer.feature ✅ existe (4 cenários)
    f09_identity_idempotencia.feature         ❌ CRIAR (2 cenários)
    f10_identity_resolve_principal.feature    ❌ CRIAR (3 cenários)
    f11_identity_subscription_bound.feature   ❌ CRIAR (2 cenários)
    f12_identity_housekeeping.feature         ❌ CRIAR (3 cenários)
    f13_identity_gateway_auth.feature         ❌ CRIAR (4 cenários)
  identity_steps_test.go                      ✅ 561 linhas — adicionar novos steps
  suite_test.go                               ✅ existe — adicionar campo e2eRuntime se necessário

internal/identity/infrastructure/repositories/postgres/
  user_identity_repository_integration_test.go   ❌ CRIAR

internal/identity/infrastructure/messaging/database/consumers/
  subscription_event_projector_integration_test.go  ❌ CRIAR
  subscription_bound_projector_test.go               ❌ CRIAR
  subscription_bound_projector_integration_test.go   ❌ CRIAR

internal/identity/application/usecases/
  resolve_principal_by_identity_integration_test.go  ❌ CRIAR
  link_channel_to_user_integration_test.go            ❌ CRIAR
  project_auth_event_integration_test.go              ❌ CRIAR
  project_subscription_event_integration_test.go      ❌ CRIAR
```

---

## 6. Estratégia de Evidência de Validação

### 6.1 Banco de Dados (Verificação Obrigatória)

Todo teste de integração e E2E DEVE verificar estado do banco via `QueryRowContext` **após** a operação:

| Operação | Query de Verificação | Afirmação |
|----------|---------------------|-----------|
| Criar usuário | `SELECT status FROM users WHERE whatsapp_number = $1 AND deleted_at IS NULL` | `status = 'ACTIVE'` |
| Soft-delete | `SELECT deleted_at FROM users WHERE id = $1` | `deleted_at IS NOT NULL` |
| Reanimar | `SELECT status, deleted_at FROM users WHERE id = $1` | `status = 'ACTIVE'` e `deleted_at IS NULL` |
| Vincular canal | `SELECT COUNT(*) FROM user_identities WHERE user_id=$1 AND channel=$2 AND unlinked_at IS NULL` | `COUNT = 1` |
| Desvincular canal | `SELECT unlinked_at FROM user_identities WHERE id = $1` | `unlinked_at IS NOT NULL` |
| Inserir auth_event | `SELECT COUNT(*) FROM auth_events WHERE id = $1` | `COUNT = 1` |
| Idempotência auth_event | `SELECT COUNT(*) FROM auth_events WHERE id = $1` | `COUNT = 1` (após 2 inserts) |
| Anonimizar | `SELECT COUNT(*) FROM auth_events WHERE user_id = $1` | `COUNT = 0` |
| Upsert entitlement | `SELECT status FROM identity_entitlements WHERE user_id = $1` | status correto |
| Idempotência entitlement | `SELECT COUNT(*) FROM identity_entitlements WHERE user_id = $1` | `COUNT = 1` |
| Outbox publish | `SELECT COUNT(*) FROM outbox_events WHERE event_type = $1 AND aggregate_user_id = $2` | `COUNT >= 1` |
| Outbox sem usuário | `SELECT COUNT(*) FROM outbox_events WHERE event_type = $1 AND aggregate_user_id IS NULL AND created_at >= $2` | `COUNT >= 1` |
| Housekeeping | `SELECT COUNT(*) FROM auth_events WHERE occurred_at < $1` | `COUNT = 0` |

### 6.2 Outbox — Validação de Envelope

Para cada evento publicado via `outbox.Publisher`, o teste de integração DEVE verificar:

```go
type outboxRow struct {
    EventType       string
    AggregateID     string
    AggregateUserID *string
    Payload         json.RawMessage
}

func queryOutboxByType(ctx context.Context, db *sqlx.DB, eventType, userID string) (outboxRow, error) {
    var row outboxRow
    err := db.QueryRowContext(ctx, `
        SELECT event_type, aggregate_id, aggregate_user_id, payload
        FROM mecontrola.outbox_events
        WHERE event_type = $1 AND aggregate_user_id = $2
        ORDER BY created_at DESC LIMIT 1`,
        eventType, userID).Scan(&row.EventType, &row.AggregateID, &row.AggregateUserID, &row.Payload)
    return row, err
}
```

Campos obrigatórios no payload: `event_id` (UUID v7), `occurred_at` (RFC3339), tipo-específicos.

### 6.3 Idempotência — Padrão de Verificação

```go
// Padrão: chamar 2x com mesmo input, verificar que COUNT não aumenta
firstCount := queryCount(ctx, db, ...)
_ = consumer.Handle(ctx, sameEvent)    // primeira vez
_ = consumer.Handle(ctx, sameEvent)    // segunda vez (mesmo event_id)
secondCount := queryCount(ctx, db, ...)
require.Equal(t, firstCount+1, secondCount) // cresceu exatamente 1, não 2
```

---

## 7. Orquestração com Subagents (Execução Paralela)

Ao executar a implementação, spawnar **1 subagent por linha da Matriz de Cobertura**:

| Subagent | Responsabilidade | Arquivos a Criar |
|----------|-----------------|-----------------|
| **SA-1: Repository** | `user_identity_repository_integration_test.go` | 1 arquivo |
| **SA-2: SubscriptionProjectors** | `subscription_event_projector_integration_test.go` + `subscription_bound_projector_test.go` + `subscription_bound_projector_integration_test.go` | 3 arquivos |
| **SA-3: UseCase Integration** | `resolve_principal_by_identity_integration_test.go`, `link_channel_to_user_integration_test.go`, `project_auth_event_integration_test.go`, `project_subscription_event_integration_test.go` | 4 arquivos |
| **SA-4: E2E f09+f10** | Feature f09 (idempotência) + f10 (resolve principal) + steps | 2 features + steps |
| **SA-5: E2E f11+f12+f13** | Feature f11 (subscription bound) + f12 (housekeeping) + f13 (gateway auth) + steps | 3 features + steps |

Cada subagent recebe:
- `AGENTS.md`, `go-implementation/SKILL.md`, `go-adapters.md` como contexto obrigatório
- O arquivo canônico de referência (ex: `auth_events_consumer_integration_test.go` para SA-2)
- A especificação desta seção do plano

Síntese final: consolidar evidências de todos os subagents (testes passando, gates limpos) antes de declarar fechado.

---

## 8. Definition of Done — Gates Obrigatórios

Antes de afirmar "módulo identity 100% coberto", TODOS os gates devem passar:

### Gate 1 — Testes Verdes
```bash
go test -race -count=1 -tags=unit ./internal/identity/...
go test -race -count=1 -tags=integration ./internal/identity/...
go test -race -count=1 -tags=e2e ./internal/e2e/...
```

### Gate 2 — Zero Comentários (R-ADAPTER-001.1)
```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/identity/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários proibidos" && exit 1 || true
```

### Gate 3 — Sem SQL Direto em Adapters (R-ADAPTER-001.2)
```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/identity/infrastructure/http/server/handlers/ \
  internal/identity/infrastructure/messaging/database/consumers/ \
  internal/identity/infrastructure/messaging/database/producers/ \
  internal/identity/infrastructure/jobs/handlers/ \
  && echo "FAIL: SQL direto em adapter" && exit 1 || true
```

### Gate 4 — Lint
```bash
golangci-lint run ./internal/identity/...
go vet ./internal/identity/...
```

### Gate 5 — Idempotência Validada
Cada consumer (`AuthEventsConsumer`, `SubscriptionEventProjector`, `SubscriptionBoundProjector`) deve ter pelo menos um teste que:
1. Processa o mesmo evento 2x com o mesmo `event_id`/`subscription_id`
2. Verifica via `SELECT COUNT(*)` que o banco não duplicou

### Gate 6 — Sem Falso Positivo (inegociável)
- Se um teste quebrar durante implementação: encontrar a causa raiz e corrigir o **código de produção**
- Nunca relaxar, pular ou comentar asserção para "passar"

---

## 9. Contexto de Integração — Arquivos Canônicos de Referência

Antes de implementar, cada subagent deve ler os seguintes arquivos como referência canônica:

| Tipo de Teste | Arquivo de Referência |
|--------------|----------------------|
| Repository integration | `internal/identity/infrastructure/repositories/postgres/user_repository_integration_test.go` |
| Consumer integration | `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer_integration_test.go` |
| Use case integration | `internal/identity/application/usecases/establish_principal_integration_test.go` |
| E2E steps | `internal/e2e/identity_steps_test.go` |
| E2E suite | `internal/e2e/suite_test.go` |
| Test helper DB | `internal/platform/database/postgres/test_helper.go` |

---

## 10. Resumo Executivo

**Estado atual:** 54 arquivos de teste, 24 cenários E2E de identity já implementados e passando.

**Gaps identificados:** 7 arquivos de integração ausentes + 5 feature files novos (14 cenários adicionais).

**Estimativa de execução:**
- 5 subagents em paralelo (SA-1 a SA-5)
- Cada subagent: ~2-4 arquivos de teste
- Nenhuma alteração em código de produção esperada (apenas testes)

**Resultado esperado ao final:**
- Pirâmide completa: Unit ✅ + Integration (todos os adapters/repos/consumers/jobs) + E2E (38 cenários total)
- Confiança operacional: cada camada falha quando o comportamento prometido quebra
- Todos os gates passando com evidência reportada
