# Execution Report — Refactor Advisory internal/identity (H4 + H5)

Data: 2026-06-12
Plano: `docs/planos/2026-06-12-refactor-advisory-internal-identity.md`

## Escopo Entregue

### H4 — EntitlementPlan como discriminated union
- Arquivo: `internal/identity/application/usecases/project_subscription_event.go`
- Substituiu struct `entitlementPlan` (com flag `isPending`) por discriminated union:
  - Interface selada `EntitlementPlan` com `isEntitlementPlan()`.
  - Variant `PendingEntitlement{SubscriptionID, FunnelToken, PayloadRaw}`.
  - Variant `CommittedEntitlement{Record}`.
- Renomeou `planEntitlementUpsert` -> `decideEntitlementPlan` (decisão pura).
- `projectCurrent` agora usa `switch p := plan.(type)` com branches `PendingEntitlement` e `CommittedEntitlement`, mais `default` defensivo retornando erro.
- Logs preservados byte-idênticos (`subscription_id`, `user_id`, `status`).
- Comportamento de IO inalterado (mesmos métodos `UpsertPending`/`Upsert`).
- Teste atualizado: `project_subscription_event_test.go` agora valida variantes via type assertion.

### H5 — EntitlementDecider em domain/services
- Criado: `internal/identity/domain/services/entitlement_decider.go`
- `EntitlementDecider.Decide(sub, now) EntitlementDecision{Entitled, Reason}` delega para `domain.IsEntitled`.
- `domain.IsEntitled` preservado integralmente (teste `entitlement_test.go` segue verde).
- `decide_user_entitlement.go` atualizado: 2 chamadas a `domain.IsEntitled` substituídas por `services.EntitlementDecider{}.Decide`, desempacotando `decision.Entitled`/`decision.Reason`.
- DTO `output.EntitlementDecision` e construtor `NewEntitlementDecision` inalterados.

## Arquivos Modificados/Criados

Criados:
- `internal/identity/domain/services/entitlement_decider.go`

Modificados:
- `internal/identity/application/usecases/project_subscription_event.go`
- `internal/identity/application/usecases/project_subscription_event_test.go`
- `internal/identity/application/usecases/decide_user_entitlement.go`

## Validação

```
gofmt -w <arquivos>                                       OK
goimports -local ... -w internal/identity/                OK
go build ./internal/identity/...                          OK
go vet ./internal/identity/...                            OK
go test ./internal/identity/application/usecases/...      ok (0.505s)
go test ./internal/identity/domain/...                    ok (todos pacotes)
grep zero-comentários nos 3 arquivos alvo                 ZERO COMMENTS OK
```

## Critérios de Aceite

- H4 — `EntitlementPlan` como discriminated union -> comprovado: interface selada `EntitlementPlan` com variants `PendingEntitlement` e `CommittedEntitlement`; `decideEntitlementPlan` puro; `switch p := plan.(type)` em `projectCurrent`; testes adaptados passam (`go test ./internal/identity/application/usecases/...`).
- H5 — `EntitlementDecider` em `domain/services/` -> comprovado: arquivo `internal/identity/domain/services/entitlement_decider.go` criado delegando para `domain.IsEntitled`; `decide_user_entitlement.go` consome o decider nas 2 chamadas; `entitlement_test.go` segue verde (`go test ./internal/identity/domain/...`).
- Comportamento preservado -> comprovado: mesmos métodos de repositório (`UpsertPending`, `Upsert`), mesma assinatura de logs, mesma DTO de saída; suite de testes do módulo identity passa sem alteração de fixtures.
- Zero comentários -> comprovado: `grep` retornou `ZERO COMMENTS OK` nos 3 arquivos editados/criados.

## Riscos Residuais

- Nenhum identificado. Mudança puramente estrutural com testes verdes.

## Status

done
