# Agentes de IA — Módulo `identity`

## Papel do módulo

O módulo `identity` é a fundação hexagonal do MeControla para gerenciamento de usuários.
Entrega o agregado `User`, os Value Objects `WhatsAppNumber`, `Email` e `UserStatus`,
o domain service puro `IsEntitled` e o contrato mínimo `Subscription`, os ports
`UserRepository` e `IDGenerator`, e o adapter Postgres `PgxUserRepository`.

---

## Contratos (ports)

### `UserRepository` — `application/interfaces/user_repository.go`

```go
type UserRepository interface {
    UpsertByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (*entities.User, error)
    FindByID(ctx context.Context, id entities.UserID) (*entities.User, error)
    FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (*entities.User, error)
    SoftDelete(ctx context.Context, id entities.UserID) error
    LinkNewNumber(ctx context.Context, id entities.UserID, number valueobjects.WhatsAppNumber, reason string) error
}
```

### `IDGenerator` — `application/interfaces/id_generator.go`

```go
type IDGenerator interface {
    NewUserID() string
}
```

Mocks gerados por mockery declarados em `mockery.yml` raiz:
```bash
mockery --config mockery.yml --dry-run  # valida zero diff
mockery --config mockery.yml            # regenera
```

---

## Regra de logging PII (`_masked`)

Nunca logar `whatsapp_number` ou `email` em claro. Usar sempre as funções do pacote
`internal/platform/observability/mask` com chave sufixada em `_masked` (ADR-004):

```go
slog.String("whatsapp_number_masked", mask.WhatsApp(user.WhatsAppNumber().String()))
slog.String("email_masked", mask.Email(user.Email().String()))
```

O `piiHandler` global aplica `[REDACTED]` nas chaves originais como rede de segurança.

---

## ADR-008 — `RehydrateUser` restrito ao mapper

`entities.RehydrateUser(params)` só pode ser chamado em `infrastructure/repositories/postgres/mapper.go`.
Aceita `Status` e `DeletedAt` arbitrários — esses já foram validados pelo banco.
Código de `application` usa `NewUser` no caminho de criação. Nunca usar `RehydrateUser`
fora do mapper.

---

## Referências

- PRD: `.specs/prd-identity-foundation/prd.md`
- Techspec: `.specs/prd-identity-foundation/techspec.md`
- ADR-004: mascaramento PII
- ADR-008: restrição do mapper

Fronteiras de import enforçadas por `depguard` em `.golangci.yml` — ver `README.md` para tabela completa.
