# Refatoração `internal/identity` — Plano Advisory

- **Data:** 2026-06-12
- **Modo:** `advisory` (sem execução; só roda em `execution` sob pedido explícito)
- **Task type:** Refatoração estrutural intra-módulo (não cross-module)
- **Skill obrigatória se promovido a `execution`:** `.agents/skills/go-implementation/SKILL.md` (Etapas 1–5, R0–R7) + `.agents/skills/refactor/SKILL.md`
- **Restrições não negociáveis:** preservar contratos públicos, comportamento observável (auth, entitlement, anonimização, persistência), DI manual, adapters finos, zero comentários em `.go`, sem `init()`/`panic`/`clock.Clock`.

---

## Context

`internal/identity` cruza fronteiras críticas (auth, PII, projections cross-module de billing/onboarding) e hoje concentra na camada `application` decisões que poderiam pertencer ao domínio. O mapeamento mostrou pontos onde DMMF agrega valor real **sem mudar comportamento**:

1. Decisão de fluxo upsert (`update | reanimate | create`) está como `switch/errors.Is` espalhado em `UpsertUserByWhatsApp.Execute`.
2. Decisão do evento de auth (`principal_established | unknown_user`) é construída em `EstablishPrincipal.resolvePrincipal` com helpers `newPrincipalEstablishedEvent`/`newUnknownUserEvent` no pacote `usecases`.
3. `interfaces.EntitlementRecord.Status` e `interfaces.SubscriptionProjectionRecord.Status` são `string`, perdendo a tipagem `domain.SubscriptionStatus` já existente.
4. Decisão de `pending vs final` em `planEntitlementUpsert` é uma união discriminada implícita (`isPending bool` + dois campos opcionais) que se beneficiaria de variantes tipadas no nível do domínio.
5. `entities.WhatsAppHistoryEntry.number` é `string` crua enquanto existe a VO `WhatsAppNumber` no mesmo módulo.
6. `eventType` em `ProjectSubscriptionEvent` é `string` switch sem enum no boundary.

O escopo desta refatoração é **isolar essas decisões em pontos puros do domínio/aplicação e tipar invariantes existentes** — preservando byte-a-byte os payloads de outbox, schemas de DB, respostas HTTP e contratos com outros módulos.

---

## Hotspots, invariantes e riscos

| # | Hotspot | Invariante | Risco de regressão |
|---|---------|------------|---------------------|
| H1 | `usecases/upsert_user_by_whatsapp.go:46-91` | Ordem: ativo → deletado dentro da janela → criar | Mudar ordem inverte semântica de `Reanimate`; mudar `time.Now().UTC()` por valor injetado quebra R6 (proibido abstrair tempo) |
| H2 | `usecases/establish_principal.go:179-221` | Sempre publica auth event **antes** de retornar; falha de publish é `errOutboxPublish` e classifica reason | Mover decisão para domínio sem manter publish dentro do UoW perde atomicidade |
| H3 | `usecases/project_subscription_event.go:55-104` | Eventos billing/onboarding desconhecidos → `nil` (idempotência); `SubscriptionBound` com `subscription_id` vazio → `nil` | Qualquer reordenação do `switch` pode quebrar o contrato de "no-op silencioso" |
| H4 | `interfaces/entitlement_repository.go:8-14` | `Status` é string crua que vem da projection (cross-module) | Tipar como `domain.SubscriptionStatus` exige normalização na borda; se a projection emitir status fora do enum, hoje passa, depois falharia |
| H5 | `entities/whatsapp_history_entry.go` | `number` é raw porque o histórico inclui números antigos que **podem não passar mais na regex atual** | Trocar para VO pode rejeitar histórico legado válido — **manter raw** |
| H6 | `repositories/postgres/user_repository.go:38-46` upsert com `COALESCE` | `SetDisplayNameIfEmpty`/`SetEmailIfEmpty` no domínio + `COALESCE` no SQL formam guarda dupla — comportamento atual depende das duas | Remover `COALESCE` requer auditoria de todos os call sites; **fora de escopo** |

**Cruzamentos cross-module (read-only nesta refactor):**
- `outbox.Publisher` (`internal/platform/outbox`) — payloads de `auth.*` e `user.deleted` **não podem mudar**.
- `SubscriptionProjectionReader` recebe dados produzidos por `internal/billing` — mapeamento `string → SubscriptionStatus` deve ser tolerante (status desconhecido = projeção é ignorada ou registrada, mas nunca panica).
- `auth.Principal` consumido por handlers HTTP fora do módulo — assinatura imutável.

---

## Plano mínimo seguro (passos ordenados, advisory)

Cada passo é **comportamentalmente equivalente**, justificado por DMMF e com saída testável isolada. Todos os passos são reversíveis individualmente.

### Passo 1 — Tipagem semântica de `EntitlementRecord.Status` no boundary
**O que:** Trocar `Status string` por `Status domain.SubscriptionStatus` em `interfaces.EntitlementRecord` e `interfaces.SubscriptionProjectionRecord`. Adicionar smart constructor `domain.ParseSubscriptionStatus(s string) (SubscriptionStatus, error)` para a borda. Mapear na borda do `postgres` repo e no `SubscriptionProjectionReader` (apenas o adapter, que lê do DB).

**Ganho DMMF:** "Make illegal states unrepresentable" — qualquer ponto que recebe `EntitlementRecord` ganha invariante de status válido sem branching defensivo.

**Trade-off:** Status desconhecido vindo do DB (legado) precisa ser tratado explicitamente no adapter. Estratégia segura: aceitar `Unknown` como variante sentinela ou logar e abortar a projeção daquele evento individual (preservando idempotência atual `nil`).

**Files:** `application/interfaces/entitlement_repository.go`, `application/interfaces/subscription_projection_reader.go`, `domain/entitlement.go` (smart constructor), `infrastructure/repositories/postgres/entitlement_repository.go`, `infrastructure/repositories/postgres/subscription_projection_reader.go`, `usecases/project_subscription_event.go`, `usecases/decide_user_entitlement.go`.

**Evidência:** unit tests novos para `ParseSubscriptionStatus`; integration test existente de `project_subscription_event` deve passar idêntico.

---

### Passo 2 — Decisão de upsert como função pura no domínio
**O que:** Extrair de `UpsertUserByWhatsApp.Execute` (linhas 46–91) uma função pura `domain.DecideUserUpsert(existing *User, deleted *User, displayName string, email Email, now time.Time) UserUpsertAction`, onde `UserUpsertAction` é uma união discriminada:

```go
type UserUpsertAction interface{ isUserUpsertAction() }
type UpdateExisting struct { User User }
type ReanimateDeleted struct { User User }
type CreateNew struct { Candidate User }
```

(implementação via interface selada idiomática Go — método não exportado `isUserUpsertAction()`).

**Ganho DMMF:** Decisão isolada, sem `tx`/`ctx`/`io`. Testável em mesa. Application só orquestra o IO no UoW e persiste a variante escolhida.

**Trade-off:** Mais um arquivo no domínio (~50 linhas). DMMF justifica porque os três caminhos têm semântica exclusiva e hoje são revelados só pelo retorno da consulta — risco de adicionar uma 4ª variante (ex.: "reanimate fora da janela → erro explícito") sem perceber.

**Files:** novo `domain/user_upsert.go`, refactor de `usecases/upsert_user_by_whatsapp.go` (Execute fica linear).

**Evidência:** unit tests cobrindo as 3 variantes + cenário "deletado fora da janela → CreateNew"; integration test `upsert_user_by_whatsapp` inalterado.

---

### Passo 3 — `entitlementPlan` como união discriminada no domínio
**O que:** Substituir `entitlementPlan{record, pendingRaw, isPending}` (`project_subscription_event.go:29-33`) por:

```go
type EntitlementProjection interface{ isEntitlementProjection() }
type EntitlementFinal struct { Record interfaces.EntitlementRecord }
type EntitlementPending struct { SubscriptionID, FunnelToken string; Raw []byte }
```

E `domain.PlanEntitlement(projection SubscriptionProjectionRecord) (EntitlementProjection, error)` puro.

**Ganho DMMF:** Elimina `bool isPending` + campos opcionais (anti-pattern primitive obsession). Cada variante carrega só o que precisa.

**Trade-off:** Tipo interface no caminho quente — custo de alocação desprezível dado que é 1 projection por evento.

**Files:** novo `domain/entitlement_projection.go`, refactor de `usecases/project_subscription_event.go`.

**Evidência:** unit tests para `PlanEntitlement` (pending vs final); integration test existente preservado.

---

### Passo 4 — Decisão do evento de auth no domínio
**O que:** Extrair de `EstablishPrincipal.resolvePrincipal` (linhas 179–221) uma função `domain.DecideAuthOutcome(user *User, found bool, now time.Time) AuthOutcome`, onde:

```go
type AuthOutcome interface{ isAuthOutcome() }
type PrincipalEstablished struct { UserID string; OccurredAt time.Time }
type UnknownUser struct { OccurredAt time.Time }
```

Application converte `AuthOutcome` → `outbox.Event` via mappers já existentes (`newAuthEventOutbox` continua na borda, mas decide com base na variante, não com `if !found`).

**Ganho DMMF:** "Workflow → output event tipado". Decisão deixa de ser implícita em um boolean.

**Trade-off:** Mantém `newAuthEventOutbox` na aplicação porque o payload é JSON contratual com consumers de outros módulos — mover serialização para o domínio violaria fronteiras.

**Files:** novo `domain/auth_outcome.go`, refactor de `usecases/establish_principal.go` (helpers `newPrincipalEstablishedEvent`/`newUnknownUserEvent` viram um único `mapAuthOutcomeToOutbox`).

**Evidência:** unit tests para `DecideAuthOutcome`; integration test existente do principal flow preservado; payload do outbox diff’ado byte-a-byte.

---

### Passo 5 — Enum tipado para `eventType` em `ProjectSubscriptionEvent`
**O que:** Criar `application.SubscriptionEventType` (string-backed enum) com `ParseSubscriptionEventType(s string) (SubscriptionEventType, bool)`. O `switch` em `Execute` passa a operar sobre a variante; default continua `nil` (idempotência preservada).

**Ganho DMMF:** Boundary tipado evita typo silencioso quando billing/onboarding evoluírem o catálogo de eventos.

**Trade-off:** Pequeno; mantém constantes string como detalhe interno do parse.

**Files:** `application/dtos/input/project_subscription_event.go` (ou novo `application/subscription_event_type.go`), `usecases/project_subscription_event.go`.

**Evidência:** unit tests do parse; integration test inalterado.

---

## Explicitamente fora de escopo

- **H5 (`WhatsAppHistoryEntry.number`)**: manter `string`. Histórico pode conter números pré-normalização; substituir por VO arrisca rejeitar dados válidos legado. Documentar como decisão.
- **H6 (`COALESCE` no upsert SQL)**: dupla guarda intencional; auditar todos os call sites é fora do escopo desta refactor.
- **UserID como VO**: candidato razoável, mas exige tocar `auth.Principal.UserID uuid.UUID`, `entities.User.id string`, `AuthEvent.userID *uuid.UUID` e DTOs cross-module. Pacote grande de mudanças sem ganho proporcional nesta janela. **Adiar.**
- **`domain/services` populado**: hoje vazio; criar serviços sem consumidor real viola R-ADAPTER-001 (interfaces sem consumidor). As decisões dos passos 2–4 ficam como funções puras em arquivos do domínio, não em `services/`.
- **Anonimização/PII**: já está em `domain/pii/mask.go` corretamente. Sem ganho DMMF identificado.
- **Refactor de `module.go`**: DI manual já segue padrão obrigatório do repositório.

---

## Validações proporcionais (caso `execution`)

Por passo:
1. `task test` no módulo identity.
2. `task lint` global.
3. Integration tests existentes do identity (`upsert`, `establish_principal`, `project_subscription_event`, `decide_user_entitlement`, housekeeping).
4. Gate R-ADAPTER-001.1 (zero comentários `.go`).
5. Gate R-ADAPTER-001.2 (adapters finos — sem SQL/branching novo em handler/consumer/job/producer).
6. Diff de payloads outbox via teste de snapshot (byte-a-byte).
7. Checklist R0–R7 de `references/build.md`.

Riscos residuais e suposições:
- Suposição: nenhum status novo será adicionado a `SubscriptionStatus` durante a refactor. Se acontecer, Passo 1 precisa absorver.
- Risco: integration test cobre paths felizes; caminhos de erro de outbox (Passo 4) precisam de teste novo se ainda não existirem.

---

## Saída final em `execution`

Quando promovido, anexar:
- Lista de arquivos alterados.
- Output de `task lint` + `task test` + integration tests.
- Diff de payload outbox antes/depois.
- Confirmação dos 7 gates de validação.
- Relatório de não-regressão.
- Status final: `done` | `needs_input` | `blocked` | `failed`.

## Status

`needs_input` — aguardando aprovação para promover qualquer passo a `execution`. Default permanece `advisory`.
