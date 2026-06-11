# Plano — Refactor `internal/identity`

Prompt fonte: `docs/refactors/identity.md`

## Contexto

O módulo `internal/identity` concentra parsing, decisão e construção de eventos em poucos use cases. Três hotspots reais foram confirmados por leitura direta:

1. **`establish_principal.go` (233 linhas)** — duas ramificações (`found` / `!found`) repetem o mesmo shape: gerar UUID v7 → montar `outbox.Event` de auth → publicar → embrulhar erro com `errOutboxPublish`. A struct `authEventPayload` e a função `buildAuthEvent` são duplicadas semanticamente em `project_auth_event.go`.
2. **`project_subscription_event.go` (175 linhas)** — 5 structs idênticas (`activatedPayload`, `renewedPayload`, `pastDuePayload`, `canceledPayload`, `refundedPayload`) e 5 métodos quase iguais cujo corpo é `Unmarshal → projectCurrent`. `projectCurrent` mistura decisão (`UserID == ""` → `UpsertPending`, senão `Upsert`) com persistência.
3. **`project_auth_event.go` (108 linhas)** — cascata `Unmarshal → Parse(eventID) → Parse(occurredAt) → Parse(userID)` com 4 blocos de logger por campo. O contexto do erro (via `fmt.Errorf("ctx: %w", err)`) já carrega a informação que os loggers replicam.

`upsert_user_by_whatsapp.go` e `module.go` já estão dentro do padrão e ficam fora do escopo.

O objetivo é deixar os use cases menores e mais explícitos via funções puras locais e factory functions concretas, sem introduzir interfaces especulativas, builder genérico ou facade. DI manual em `module.go` é preservada.

## Skills obrigatórias (R-ADAPTER-001.3 e SKILL.md)

Carregamento obrigatório antes de editar:

- `AGENTS.md`
- `.claude/skills/go-implementation/SKILL.md`
- `.claude/skills/go-implementation/references/architecture.md`
- `.claude/skills/go-implementation/references/interfaces.md`
- `.claude/skills/go-implementation/references/examples-domain-flow.md`
- `.claude/skills/go-implementation/references/testing.md` — apenas se reescrever suites (esperado: ajustes proporcionais, não reescrita).

Máximo de 4 simultâneas. R0–R7 do `build.md` aplicáveis na validação.

## Mudanças

### 1. Novo arquivo: `internal/identity/application/usecases/auth_event_payload.go`

Centraliza a única struct de payload de auth event e duas funções puras consumidas tanto pelo produtor (`establish_principal.go`) quanto pelo projector (`project_auth_event.go`):

- `type authEventPayload struct { ... }` — única fonte da forma JSON.
- `func newAuthEventOutbox(eventID, userID, kind, source, reason string, now time.Time) (outbox.Event, error)` — substitui `buildAuthEvent` de `establish_principal.go`. Encapsula marshal e a regra de `AggregateID = userID ?: eventID`.
- `func parseAuthEvent(raw []byte) (entities.AuthEvent, error)` — substitui as 4 cascatas de parse + logger em `project_auth_event.go`. Retorna `entities.AuthEvent` hidratada via `entities.HydrateAuthEvent`.

Funções puras (sem `context.Context`, sem IO). Zero comentários.

### 2. Refactor: `establish_principal.go`

- Remove `authEventPayload` e `buildAuthEvent` (movidas para `auth_event_payload.go`).
- Adiciona duas factory functions concretas no mesmo arquivo (logo após os tipos de erro):
  - `func newPrincipalEstablishedEvent(userID string) (outbox.Event, error)` — gera UUID v7, monta payload com `kind="principal_established"`, `source="whatsapp"`.
  - `func newUnknownUserEvent() (outbox.Event, error)` — gera UUID v7, monta payload com `kind="unknown_user"`, sem `userID`.
- Cria método privado `(u *EstablishPrincipal) publishAuthOutcome(ctx, ev outbox.Event) error` que faz `u.publisher.Publish(ctx, ev)` e embrulha falha com `errOutboxPublish{wrapped: fmt.Errorf("publish %s: %w", ev.Type, pubErr)}`.
- O `uow.Do` agora tem dois ramos enxutos.
- `classifyEstablishErrorReason` permanece como função pura no arquivo — **não** vira interface.
- Bloco final de métricas/logger no `Execute` permanece inalterado.

### 3. Refactor: `project_subscription_event.go`

- Colapsa as 5 structs idênticas em uma única `subscriptionRefPayload`.
- Substitui os 5 métodos por um único helper `extractSubscriptionRef`.
- Extrai função pura `planEntitlementUpsert(p)` que decide pending vs active sem IO.

### 4. Refactor: `project_auth_event.go`

- Remove a struct local `projectAuthEventPayload` (agora vive em `auth_event_payload.go`).
- `Execute` passa a ser: `parseAuthEvent(in.Payload)` → `repo.Insert(ctx, ev)`. As 4 chamadas de logger por campo são removidas.

### 5. Testes

Atualização proporcional, sem reescrita de suíte.

## Restrições respeitadas

- Sem `clock.Clock`, sem `now func()` injetado.
- Sem comentários em arquivos `.go` (R-ADAPTER-001.1).
- Sem `var _ Interface = (*Type)(nil)`.
- Adapters não são tocados — todas as mudanças ficam em `application/usecases/`.
- Nenhuma interface nova.
- DI manual em `module.go` preservada.
- Semântica de erros públicos inalterada.

## Arquivos críticos

- `internal/identity/application/usecases/auth_event_payload.go` (NOVO)
- `internal/identity/application/usecases/establish_principal.go` (slim)
- `internal/identity/application/usecases/project_auth_event.go` (slim)
- `internal/identity/application/usecases/project_subscription_event.go` (slim + colapso de structs)
- Testes correspondentes na mesma pasta

Fora do escopo: `module.go`, `upsert_user_by_whatsapp.go`, `infrastructure/`.

## Verificação

1. Gates de governança (zero comentários, R-ADAPTER-001.2).
2. `go build ./...`, `go test ./internal/identity/...`, `go vet ./...`.
3. Checklist R0–R7.
4. Confirmar redução de linhas e mesmas métricas/spans.
