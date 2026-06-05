# Tarefa 2.0: Domínio puro — VOs, PII, entitlement, policies, entities.NewID

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar todo o domínio puro de identity sem dependência de application/infrastructure: Value Objects `WhatsAppNumber` (E.164 BR) e `Email`, helper de mascaramento de PII, contrato mínimo `Subscription` + função pura `IsEntitled` com 11 transições, constante `ReanimationWindow`, e função `entities.NewID()` (gera UUID via `uuid.NewString()`, sem DI). Cobertura de testes 100% nos VOs e em `IsEntitled` é requisito (CA-01).

<requirements>
- RF-03: `WhatsAppNumber` Value Object imutável, construtor normaliza para E.164 BR (`+55DDD9NNNNNNNN`), rejeita inválido com erro tipado.
- RF-04: APIs internas trafegam o VO, nunca `string`. Garantido em compile-time pela ausência de versões `string`.
- RF-05: `Email` Value Object imutável, normaliza lowercase, valida formato básico (presença de `@` e domínio plausível via `net/mail.ParseAddress`).
- RF-12: `IsEntitled(sub Subscription, now time.Time) (bool, Reason)` puro, cobre `sub == nil` + 11 transições enumeradas.
- RF-13: `Subscription` declarada como interface mínima (`Status() SubscriptionStatus`, `PeriodEnd() time.Time`, `GracePeriodEnd() time.Time`) em `domain/entitlement.go`.
- RF-14: helpers `Masked()` nos VOs + `pii.MaskDisplayName`.
- R6.8 (techspec): `entities.NewID() string` em `domain/entities/id.go` chama `uuid.NewString()` direto — proibido DI.
- ADR-006: `ReanimationWindow = 30 * 24 * time.Hour` como constante de domínio em `domain/policies.go`.
- ADR-001: `type Reason string` com constantes nomeadas.
- ADR-002: `Subscription` como interface (não struct); `SubscriptionStatus` é `string` (não iota) — interop JSON.
- ADR-003: `WhatsAppNumber.Masked()` retorna `+55 DD 9****-NNNN`; `Email.Masked()` retorna `<primeira-letra>***@<domínio>`; `pii.MaskDisplayName` retorna `<primeira-rune>****` (caso vazio → vazio, 1 rune → `*`).
</requirements>

## Subtarefas

- [ ] 2.1 `internal/identity/domain/entities/id.go` com `func NewID() string { return uuid.NewString() }` + comentário declarando "sem DI" (R6.8).
- [ ] 2.2 `internal/identity/domain/valueobjects/whatsapp_number.go` (struct privada `e164`, `NewWhatsAppNumber`, `String`, `Equal`, `Masked`, `normalizeRaw`) + testes parametrizados 100% (com/sem `+55`, formatação, fixo rejeitado, 9 dígitos rejeitado, multibyte, casos limítrofes).
- [ ] 2.3 `internal/identity/domain/valueobjects/email.go` (struct privada `addr`, `NewEmail`, `String`, `Equal`, `Masked`) + testes 100%.
- [ ] 2.4 `internal/identity/domain/pii/mask.go` com `MaskDisplayName(name string) string` cobrindo vazio, 1 rune, multibyte (acentos), 2+ runes + testes 100%.
- [ ] 2.5 `internal/identity/domain/entitlement.go` com `SubscriptionStatus` (string, 6 constantes), `Subscription` interface, `Reason` (string, 8 constantes), `IsEntitled` puro + testes parametrizados cobrindo `sub == nil` + 11 transições de RF-12.
- [ ] 2.6 `internal/identity/domain/policies.go` com `const ReanimationWindow = 30 * 24 * time.Hour`.

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §Design por Superfície — Domínio](./techspec.md) — snippets canônicos de `WhatsAppNumber`, `Email`, `pii.MaskDisplayName`, `entitlement.go`, `policies.go`.
- [ADR-001](./adr-001-reason-string-type.md), [ADR-002](./adr-002-subscription-contract-interface.md), [ADR-003](./adr-003-pii-masking-vo-methods.md), [ADR-006](./adr-006-reanimation-window-constant.md), [ADR-008](./adr-008-repository-factory-per-module.md).
- [Runbook §5.1](../../docs/runbooks/handler-usecase-uow-repository.md) — padrão `entities.NewID()`.

**Pontos críticos:**

- `WhatsAppNumber`: regex BR-only `^\+55\d{2}9\d{8}$`. Normalização: strip de espaços/parênteses/traços, prefixo `+55` injetado quando ausente, validação final pelo regex.
- `Email`: trim + lowercase + `net/mail.ParseAddress` para validação. Não trima espaço interno (responsabilidade do call-site).
- `IsEntitled`: cada caso retorna `(bool, Reason)` exato conforme RF-12. Caso `default` (status desconhecido) → `(false, ReasonExpired)` — comportamento conservador documentado.

## Critérios de Sucesso

- `go test -race -cover ./internal/identity/domain/...` reporta 100% em `valueobjects/`, `pii/` e `entitlement.go` (CA-01).
- `go build ./...` verde.
- `go vet ./...` verde.
- Nenhum import de `application` ou `infrastructure` em `domain/**` (validado posteriormente por `depguard` em 9.0, mas inspeção manual obrigatória aqui).
- Nenhuma dependência externa além de `github.com/google/uuid` e stdlib (`regexp`, `strings`, `net/mail`, `unicode/utf8`, `time`, `errors`, `fmt`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `valueobjects/whatsapp_number_test.go` — table tests parametrizados (100% linhas/branches).
- [ ] `valueobjects/email_test.go` — table tests (100%).
- [ ] `pii/mask_test.go` — table tests cobrindo vazio, 1 rune ASCII, 1 rune multibyte, 2+ runes, runa inválida UTF-8 (100%).
- [ ] `entitlement_test.go` — table test parametrizado com (`sub == nil`, 11 transições) → todos os pares `(entitled, reason)` esperados.
- [ ] `entities/id_test.go` — valida invariante (parse `uuid.Parse(NewID())` ok + versão = 4 + N chamadas geram N distintos).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/domain/entities/id.go` (criar)
- `internal/identity/domain/entities/id_test.go` (criar)
- `internal/identity/domain/valueobjects/whatsapp_number.go` (criar)
- `internal/identity/domain/valueobjects/whatsapp_number_test.go` (criar)
- `internal/identity/domain/valueobjects/email.go` (criar)
- `internal/identity/domain/valueobjects/email_test.go` (criar)
- `internal/identity/domain/pii/mask.go` (criar)
- `internal/identity/domain/pii/mask_test.go` (criar)
- `internal/identity/domain/entitlement.go` (criar)
- `internal/identity/domain/entitlement_test.go` (criar)
- `internal/identity/domain/policies.go` (criar)
- `internal/identity/domain/services/doc.go` (criar — placeholder vazio)
