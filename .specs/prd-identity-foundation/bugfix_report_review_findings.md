# Bugfix Report — E1 identity-foundation (achados do review)

- **Origem:** Skill `review` em 2026-06-09 contra `.specs/prd-identity-foundation/`
- **Escopo fixo:** apenas achados 1 (CA-01 cobertura) e 2 (lint). Achado 3 (drift outbox em `mark_user_deleted.go`) intencionalmente fora.
- **Total no escopo:** 2 | **Corrigidos:** 2 | **Falhos:** 0 | **Skipped:** 0

## Bug B1 — Cobertura de `IsEntitled` em 57.9% viola CA-01/M-01

- **Origem:** Review finding #1 (RF-12, CA-01, M-01 do PRD)
- **Severity:** critical
- **Status:** fixed
- **File:** `internal/identity/domain/entitlement_test.go`
- **Reproduction:** `go test -count=1 -coverprofile=/tmp/ent.out ./internal/identity/domain/... && go tool cover -func=/tmp/ent.out | grep IsEntitled` → `57.9%`
- **Expected:** `IsEntitled` em 100.0% com todas as 11 transições do RF-12 + `sub == nil` cobertas por subtestes nomeados
- **Actual antes do fix:** 4/11 transições + 1 ramo de PastDue não exercitadas

### Causa raiz

A suíte `TestIsEntitled` continha apenas 5 cenários: `nil`, `ACTIVE > now`, `PAST_DUE` sem grace (com `graceEnd < now`), `CANCELED_PENDING > now` e status desconhecido. Faltavam:

1. `ACTIVE` com `period_end <= now` → `(false, expired)`
2. `TRIALING > now` → `(true, trialing)`
3. `TRIALING <= now` → `(false, expired)`
4. `PAST_DUE` com `grace > now` → `(true, past_due_grace)`
5. `PAST_DUE` com `grace.IsZero()` → `(false, past_due_no_grace)` (ramo `!grace.IsZero()` curto-circuita antes de `After`)
6. `CANCELED_PENDING <= now` → `(false, expired)`
7. `EXPIRED` → `(false, expired)`
8. `REFUNDED` → `(false, refunded)`

A função `IsEntitled` é pura e tem 13 statements ramificáveis (`return`s); cada `return` precisa ser executado pelo menos uma vez para 100% statement coverage.

### Correção mínima

Adicionados 8 subtestes parametrizados em `entitlement_test.go` reutilizando a infraestrutura existente (`domainmocks.NewSubscription`, switch que aciona `PeriodEnd()`/`GracePeriodEnd()` apenas para o status pertinente). Sem alteração de assinatura, constantes ou comportamento runtime. Sem comentários adicionados. Sem abstração de tempo (usa `time.Time` literal já existente no Suite).

### Teste de regressão

A própria adição é regressão — qualquer remoção futura de subteste reduz a cobertura. Gate validado por `go tool cover -func`.

### Evidência

```text
$ go test -count=1 -coverprofile=/tmp/ent.out ./internal/identity/domain/...
ok  ... internal/identity/domain    coverage: 100.0% of statements
$ go tool cover -func=/tmp/ent.out | grep IsEntitled
internal/identity/domain/entitlement.go:35: IsEntitled  100.0%
```

---

## Bug B2 — `golangci-lint` quebra CI em `./internal/identity/...`

- **Origem:** Review finding #2 (CA-02, governança Go)
- **Severity:** major
- **Status:** fixed
- **Files:**
  - `internal/identity/domain/entities/auth_event.go:69`
  - `internal/identity/infrastructure/messaging/database/consumers/mocks/anonymize_user_auth_events_use_case.go:6,15`
  - `internal/identity/infrastructure/messaging/database/consumers/mocks/project_auth_event_use_case.go:6,15`
  - `internal/identity/application/usecases/establish_principal_test.go:43-52`
- **Reproduction:** `golangci-lint run --timeout=180s ./internal/identity/...` → `6 issues`
- **Expected:** `0 issues`

### Causa raiz

Quatro defeitos independentes:

1. **goimports x3** — alinhamento de método `Reason()` em `auth_event.go` empurrou as 4 linhas anteriores fora do padrão `gofmt`; nos dois mocks regenerados, o bloco de imports estava com a ordem `local → external` (`internal/...` antes de `stretchr/testify/mock`), invertida em relação ao `local-prefixes: github.com/LimaTeixeiraTecnologia/mecontrola` declarado em `.golangci.yml`.
2. **staticcheck QF1008 x2** — `m.Mock.Test(t)` referencia o campo embeddado redundantemente; o método `Test` é promovido pelo embedding e a forma idiomática é `m.Test(t)`. Era resíduo de geração antiga do `mockery`.
3. **unused x1** — `(*EstablishPrincipalSuite).mustHydratedUser` introduzido em iteração anterior e nunca chamado pelo `TestExecute` (que define um `hydratedUser` inline via `entities.Hydrate`).

### Correção mínima

- `goimports -local github.com/LimaTeixeiraTecnologia/mecontrola -w` aplicado aos 3 arquivos (reordena imports + realinha colunas de método).
- `m.Mock.Test(t)` → `m.Test(t)` nos dois mocks (idêntico em comportamento; o método continua sendo promovido pelo embedding de `mock.Mock`).
- Remoção do helper `mustHydratedUser` em `establish_principal_test.go`. Sem alteração do `TestExecute` que continua usando `entities.Hydrate` inline.

Sem alteração de contrato público, sem novos comentários, sem `init`/`panic`/asserção de interface.

### Teste de regressão

Lint é o gate. CI executa `golangci-lint run ./internal/identity/...`; qualquer reintrodução de `m.Mock.Test`, helper unused ou drift de import reativa as violações.

### Evidência

```text
$ golangci-lint run --timeout=180s ./internal/identity/...
0 issues.
```

---

## Validação Final

```text
$ go test -count=1 -race ./internal/identity/...
ok  internal/identity/application                       2.128s
ok  internal/identity/application/auth                  1.779s
ok  internal/identity/application/usecases              2.902s
ok  internal/identity/domain                            1.737s
ok  internal/identity/domain/entities                   2.079s
ok  internal/identity/domain/pii                        1.397s
ok  internal/identity/domain/valueobjects               2.430s
ok  internal/identity/infrastructure/http/server        3.305s
ok  internal/identity/infrastructure/http/server/handlers     3.151s
ok  internal/identity/infrastructure/http/server/middleware   3.154s
ok  internal/identity/infrastructure/jobs/handlers      3.032s
ok  internal/identity/infrastructure/messaging/database/consumers   3.136s
ok  internal/identity/infrastructure/repositories       3.216s
ok  internal/identity/infrastructure/repositories/postgres  3.306s

$ go vet ./internal/identity/...
(sem saída — clean)

$ go tool cover -func=/tmp/ent.out | grep -E "IsEntitled|NewWhatsAppNumber|NewEmail"
entitlement.go:35:        IsEntitled         100.0%
valueobjects/email.go:16: NewEmail           100.0%
valueobjects/whatsapp_number.go:20: NewWhatsAppNumber 100.0%

$ golangci-lint run --timeout=180s ./internal/identity/...
0 issues.
```

## Diff resumido

| Arquivo | Mudança | Linhas |
|---|---|---|
| `internal/identity/domain/entitlement_test.go` | +8 subtestes parametrizados (transições faltantes RF-12) | +72 |
| `internal/identity/domain/entities/auth_event.go` | goimports realinhou 5 métodos de uma linha | ±5 |
| `internal/identity/infrastructure/messaging/database/consumers/mocks/anonymize_user_auth_events_use_case.go` | imports reordenados + `m.Mock.Test(t)` → `m.Test(t)` | ±3 |
| `internal/identity/infrastructure/messaging/database/consumers/mocks/project_auth_event_use_case.go` | imports reordenados + `m.Mock.Test(t)` → `m.Test(t)` | ±3 |
| `internal/identity/application/usecases/establish_principal_test.go` | remoção do helper `mustHydratedUser` não utilizado | −10 |

## Invariantes preservadas

- Assinatura `IsEntitled(sub Subscription, now time.Time) (bool, Reason)` inalterada.
- Constantes `Reason*` inalteradas.
- Comportamento runtime de `IsEntitled` idêntico — apenas testes adicionados.
- Mocks mantêm a mesma superfície (`Execute`, `EXPECT`, etc.).
- Nenhuma mudança fora dos paths listados nos achados; drift do `mark_user_deleted.go` (achado #3) intacto.

## Riscos residuais

- **R-1 (baixo):** achado #3 do review (drift de outbox em `mark_user_deleted.go`, ~475 linhas não commitadas) segue aberto e fora deste fluxo; precisa de spec dedicada antes de merge.
- **R-2 (baixo):** mocks gerados ainda usam `interface{}` em vez de `any` (linter `golangci-lint` não sinaliza, apenas o LSP); cosmético, não bloqueia CI.

## Estado canônico final

**done** — escopo acordado (achados 1 e 2) corrigido e validado contra `go test -race`, `go vet`, `go tool cover -func`, `golangci-lint`.
