# Módulo `identity`

Responsabilidade: gerenciamento do agregado `User` — cadastro via WhatsApp, soft delete com histórico, mascaramento PII e verificação de elegibilidade por assinatura.

Este módulo segue o **layout hexagonal canônico** do MeControla:

```
internal/identity/
├── domain/           # Regras de negócio puras (sem IO)
├── application/      # Casos de uso + ports (interfaces)
└── infrastructure/   # Implementações concretas (Postgres, UUID)
```

---

## Value Objects

| VO | Arquivo | Invariante |
|----|---------|-----------|
| `WhatsAppNumber` | `domain/valueobjects/whatsapp_number.go` | E.164 BR — normaliza 10/11/12/13 dígitos; rejeita não-BR |
| `Email` | `domain/valueobjects/email.go` | RFC via `net/mail`; normalizado em lowercase; exige TLD |
| `UserStatus` | `domain/valueobjects/user_status.go` | Enum `ACTIVE`/`BLOCKED`/`DELETED`; zero-value reservado a `UNKNOWN` |

---

## Regra `IsEntitled`

`EntitlementChecker.IsEntitled(subscription, now)` é uma função pura em `domain/services/entitlement.go`.
Consome o contrato mínimo `Subscription` (interface declarada em `domain/services/subscription.go`).
A implementação concreta de `Subscription` vive em `internal/billing` (Épico E2) — sem import cíclico.

| Status | Condição de elegibilidade |
|--------|--------------------------|
| `Trialing`, `Active` | `now.Before(CurrentPeriodEnd())` |
| `PastDue`, `CanceledPending` | `now.Before(GracePeriodEnd())` |
| `Expired`, `Refunded`, `Unknown` | sempre `false` |
| `nil` | sempre `false` |

---

## Convenção `_masked` para logging de PII

Nunca passar `whatsapp_number` ou `email` em claro em logs.
Usar `mask.WhatsApp(number)` e `mask.Email(email)` do pacote `internal/platform/observability/mask`,
com chave de atributo sufixada em `_masked`:

```go
slog.String("whatsapp_number_masked", mask.WhatsApp(user.WhatsAppNumber().String()))
slog.String("email_masked", mask.Email(user.Email().String()))
```

O `piiHandler` global aplica `[REDACTED]` nas chaves `whatsapp_number` e `email` como rede de segurança (ADR-004).

---

## Regra `RehydrateUser` — exclusivo do mapper

`entities.RehydrateUser(params)` é o único construtor de reidratação do agregado `User`.
Aceita `Status` e `DeletedAt` arbitrários (já validados pelo banco via CK constraint).
**Uso restrito ao mapper em `infrastructure/repositories/postgres/mapper.go`.**
Código de `application` nunca deve chamar `RehydrateUser` diretamente — use `NewUser` no caminho de criação (ADR-008).

---

## Fronteiras `depguard`

| Pacote | Pode importar | Proibido |
|--------|--------------|---------|
| `domain` | stdlib | `application`, `infrastructure`, `internal/platform/*`, `configs/*` |
| `application` | `domain` | `infrastructure`, bibliotecas de IO concretas |
| `infrastructure` | `domain`, `application`, `internal/platform/*` | cross-module direto (`billing`, `onboarding`, `finance`) |

Regras enforçadas em `.golangci.yml` via `depguard` (RF-16):
- `domain-no-infrastructure`: domain não importa infrastructure de nenhum módulo.
- `application-no-infrastructure`: application não importa infrastructure.
- `identity-no-finance`: identity não importa finance diretamente.
- `finance-no-identity`: finance não importa identity diretamente.

---

## Referências

- PRD: `.specs/prd-identity-foundation/prd.md`
- Techspec: `.specs/prd-identity-foundation/techspec.md`
- ADR-004: mascaramento PII via pacote `mask`
- ADR-008: `RehydrateUser` restrito ao mapper

---

## Comandos `ai-spec` recomendados

```bash
# Iniciar novo ciclo de feature neste módulo
ai-spec create-prd

# Derivar especificação técnica do PRD aprovado
ai-spec create-technical-specification

# Decompor em tarefas incrementais
ai-spec create-tasks

# Executar tarefa isolada
ai-spec execute-task

# Verificar drift entre spec e tasks
ai-spec check-spec-drift .specs/prd-identity-<feature>/tasks.md
```
