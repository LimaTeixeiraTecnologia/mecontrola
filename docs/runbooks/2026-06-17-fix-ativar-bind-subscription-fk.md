# Fix: `ATIVAR` → "Sistema temporariamente indisponivel" (FK no bind da subscription)

**Data:** 2026-06-17
**Módulo:** `internal/onboarding`
**Skill obrigatória:** `go-implementation` (Etapas 1–5, gates R0–R7)
**Tipo:** bugfix de consistência transacional (causa raiz observada)

---

## Sintoma

No WhatsApp, usuário envia `ATIVAR <token>` e recebe **"Sistema temporariamente indisponivel. Tente novamente em alguns minutos."**, mesmo após o deploy do commit `b23defd` (fix do nested tx).

A mensagem genérica só é enviada quando `consumeUseCase.Execute` retorna erro real
(`whatsapp_message_processor.go:95-101`) — nunca um outcome de domínio (token expirado/não-pago/
inválido têm mensagens próprias). Logo, a falha estava dentro de `BindAndConsume`.

## Causa raiz (observada, não inferida)

`BindAndConsume` (`internal/onboarding/application/binding/subscription_binding.go`) roda dentro da
transação `tx1` do `ConsumeMagicToken` (aberta por `consumeUoW.Do`):

1. `UpsertUserByWhatsApp` cria o usuário **dentro de `tx1`** (não commitado). Já usava
   `database.FromContext` (b23defd).
2. `subscriptionBinder.BindUser` fazia `UPDATE billing_subscriptions SET user_id = <novoUserID>`.

O defeito estava no wiring em `internal/onboarding/module.go:175`:

```go
subscriptionBinder := postgres.NewSubscriptionBinder(o11y, mgr.DBTX(context.Background()))
```

`mgr.DBTX(context.Background())` é avaliado **uma vez no startup**, sem tx no contexto → o manager
retorna o **pool** (devkit `manager.go:171`). O binder usava esse pool em toda chamada, então
`BindUser` rodava **numa conexão separada, fora de `tx1`** → não enxergava o usuário não-commitado →
violava a FK `billing_subscriptions_user_id_fkey → users(id)`:

```
subscription_binder.bind_user: ERROR: insert or update on table "billing_subscriptions"
violates foreign key constraint "billing_subscriptions_user_id_fkey" (SQLSTATE 23503)
```

Isso sobe como `onboarding/binding: bind subscription: %w` → `execErr != nil` → mensagem genérica.

**Por que só apareceu depois do b23defd:** antes, `UpsertUserByWhatsApp` falhava primeiro com
`ErrNestedTransaction`. O fix do upsert fez o usuário entrar em `tx1` e **expôs** este bug latente do
binder. Atinge sempre a **primeira ativação** (usuário novo, INSERT invisível ao pool).

## Fix

Tornar o binder **tx-aware por chamada**, igual a `managerPublisher.Publish` (`module.go`):
resolver `mgr.DBTX(ctx)` em cada chamada em vez de capturar o pool no construtor.

- `subscription_binder.go`: campo `mgr manager.Manager`; `BindUser` usa `b.mgr.DBTX(ctx).ExecContext(...)`.
- `module.go:175`: `NewSubscriptionBinder(o11y, mgr)`.

Cobre **ambos** os caminhos de ativação (WhatsApp `ConsumeMagicToken` e Telegram
`ActivateTelegramByToken`), pois ambos usam o mesmo `bindingService`/binder.

## Prova (anti-falso-positivo)

Teste de integração (testcontainers + Postgres 16) — `subscription_binder_integration_test.go`:

- `TestBindUser_SeesUserCreatedInSameTx`: insere usuário **dentro da tx** e exige que `BindUser` o
  enxergue. **Falha no código antigo** (FK 23503 reproduzida) e **passa no novo** — prova de regressão.
- `TestBindUser_RollsBackWithTxOnLaterError`: erro posterior na tx desfaz o bind (atomicidade).
- `TestBindUser_SubscriptionNotFound`: `rows==0` → erro "subscription not found".

Unit (`subscription_binder_test.go`): resolução per-call de `mgr.DBTX(ctx)`, propagação de erro, not found.

## Auditoria de bugs irmãos

5 subagentes (3 por módulo + 2 adversariais) auditaram `internal/{billing,onboarding,identity}` pela
mesma classe (pool capturado no startup escrevendo em tx externa; `uow.Do` aninhado sem guard).
**Resultado: 0 bugs confirmados.**

- Únicos `DBTX(context.Background())` remanescentes: `billing/module.go:50` (kiwifyDBTX) e
  `onboarding/module.go:236` (cleanup) — jobs/crons/boot standalone, sem tx externa → corretos.
- Os 7 consumers do outbox rodam dentro da tx do dispatcher (`dispatcher.go:53`) mas são adapters
  finos `consumer → usecase → repo(mgr.DBTX(ctx))` per-call, sem `uow.Do` aninhado nem write-on-pool.
- Todas as chamadas cross-módulo dentro de `uow.Do` participam da tx (binder per-call; publisher
  per-call; identity `UpsertUserByWhatsApp` com guard `FromContext`; repos via `tx` por parâmetro).

Notas (não bloqueiam, fora de escopo): `LinkChannelToUser`/`MarkUserDeleted` (identity) sem chamador
(possível código morto); se `SendSubscriptionNotification` passar a escrever no banco, deverá usar
`mgr.DBTX(ctx)` (roda na tx do dispatcher).

## Validação executada

- `go build ./...` ✅ · `go vet` (3 módulos) ✅
- Unit + `-race` (`billing`, `onboarding`, `identity`) ✅
- Integração binder `-tags integration` 3/3 ✅
- Gates: R0 (sem `init()`), R7 (sem `interface{}`), R-ADAPTER-001.1 (zero comentários),
  R-ADAPTER-001.2 (sem SQL direto em handlers/consumers/jobs) ✅

## Verificação pós-deploy (gap B, opcional)

```bash
ssh root@187.77.45.48 "docker logs mecontrola-server-1 --tail=80 | grep consume_failed"
```
Antes: erro com `bind subscription: ... billing_subscriptions_user_id_fkey`. Depois: ausente.
E2E: runbook `2026-06-17-e2e-fase2-webhook-ativacao.md`, Passos 6–7, com usuário novo.
