# Tarefa 3.0: Agregados User e WhatsAppHistoryEntry com construtores autossuficientes

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o agregado `User` e a entidade `WhatsAppHistoryEntry`. Construtores são **autossuficientes**: chamam `entities.NewID()` internamente para gerar UUID v4 e `time.Now().UTC()` inline para timestamps — sem `IDGenerator` injetado, sem captura de variável de tempo (R6.7, R6.8 do techspec; ADR-008). Inclui métodos de domínio que aplicam regras de negócio (`SetDisplayNameIfEmpty` para first-write-wins, `MarkDeleted`, `Reanimate`, `CanReanimate`) + getters + função `entities.Hydrate` para reconstrução vinda do repositório.

<requirements>
- RF-01: PK UUID v4 gerada pelo próprio domínio via `entities.NewID()`.
- RF-06: agregado expõe `Status` (`ACTIVE | DELETED`) e `DeletedAt`. Método `MarkDeleted(now time.Time)` seta os dois campos atomicamente; método `Reanimate(now time.Time)` zera `deleted_at` + status `ACTIVE` + zera `email`/`display_name`.
- RF-08-bis (first-write-wins): `User.SetDisplayNameIfEmpty(name string)` só popula quando o atual é vazio; método `User.SetEmailIfEmpty(e valueobjects.Email)` segue o mesmo padrão.
- RF-08-ter (reanimação ≤ 30d): `User.CanReanimate(now time.Time) bool` retorna `(now.Sub(u.deletedAt) <= ReanimationWindow) && !u.deletedAt.IsZero()`.
- ADR-006: `ReanimationWindow` consumida (importada de `domain/policies.go`).
- ADR-008: construtor `entities.New(whatsapp, opts...) User` **não** recebe `IDGenerator`; opções funcionais (`WithEmail`, `WithDisplayName`).
- WhatsAppHistoryEntry: `NewWhatsAppHistoryEntry(userID string, number string, reason string) WhatsAppHistoryEntry` gera ID via `entities.NewID()` e `linked_at` via `time.Now().UTC()` inline.
</requirements>

## Subtarefas

- [ ] 3.1 `internal/identity/domain/entities/user.go`:
  - Tipo `Status` (string) com `StatusActive`, `StatusDeleted`.
  - Struct `User` com campos privados (id, whatsapp, email, displayName, status, createdAt, updatedAt, deletedAt).
  - Construtor `New(whatsapp valueobjects.WhatsAppNumber, opts ...Option) User` chamando `NewID()` e `time.Now().UTC()` inline.
  - `Option` + `WithEmail(valueobjects.Email) Option` + `WithDisplayName(string) Option`.
  - Getters: `ID()`, `WhatsApp()`, `Email()`, `DisplayName()`, `Status()`, `CreatedAt()`, `UpdatedAt()`, `DeletedAt()`.
  - Métodos de domínio: `MarkDeleted(now time.Time)`, `Reanimate(now time.Time)`, `CanReanimate(now time.Time) bool`, `SetDisplayNameIfEmpty(name string)`, `SetEmailIfEmpty(e valueobjects.Email)`.
  - Função `Hydrate(id, whatsapp, email, displayName, status string, createdAt, updatedAt, deletedAt time.Time) User` para reconstrução do repo (não chama `NewID`/`Now`).
- [ ] 3.2 `internal/identity/domain/entities/whatsapp_history_entry.go`:
  - Struct `WhatsAppHistoryEntry` com campos exportados (ID, UserID, Number, Active, LinkedAt, UnlinkedAt, Reason) ou privados + getters — escolher conforme uso pelo repo.
  - Construtor `NewWhatsAppHistoryEntry(userID, number, reason string) WhatsAppHistoryEntry` gera `ID = NewID()`, `LinkedAt = time.Now().UTC()`, `Active = true`.
- [ ] 3.3 Testes de borda da janela de reanimação: igual a 30d, 30d - 1ns, 30d + 1ns, deletedAt zero.
- [ ] 3.4 Testes de first-write-wins: `SetDisplayNameIfEmpty` em campo vazio popula; em campo populado preserva.

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §Design por Superfície — Domínio (entities/user.go)](./techspec.md) — esqueleto canônico do agregado.
- [Runbook §5 + §5.1](../../docs/runbooks/handler-usecase-uow-repository.md) — padrão de construtor autossuficiente.
- [ADR-006](./adr-006-reanimation-window-constant.md) — consumo de `ReanimationWindow`.
- [ADR-008](./adr-008-repository-factory-per-module.md) — regra "sem IDGenerator injetado".

**Padrão do construtor (inegociável):**

```go
func New(whatsapp valueobjects.WhatsAppNumber, opts ...Option) User {
    u := User{
        id:        NewID(),                       // domínio se auto-serve
        whatsapp:  whatsapp,
        status:    StatusActive,
        createdAt: time.Now().UTC(),              // inline
        updatedAt: time.Now().UTC(),
    }
    for _, opt := range opts { opt(&u) }
    return u
}
```

**Reanimate semântica (RF-08-ter):**

```go
func (u *User) Reanimate(now time.Time) {
    u.status = StatusActive
    u.deletedAt = time.Time{}
    u.email = valueobjects.Email{}     // zera PII por LGPD
    u.displayName = ""
    u.updatedAt = now
}
```

## Critérios de Sucesso

- `go test -race -cover ./internal/identity/domain/entities/...` reporta cobertura ≥95% no agregado (CA-01).
- Testes de borda (`30d`, `30d-1ns`, `30d+1ns`, zero) validam `CanReanimate` corretamente.
- `User.MarkDeleted(now)` sempre seta `status=DELETED` E `deletedAt=now` juntos (invariante).
- `User.Reanimate(now)` sempre zera `email` e `displayName` (LGPD — RF-08-ter).
- `entities.New(...)` não compila se receber `IDGenerator` (verifica ausência).
- `go build ./...` verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `entities/user_test.go` — table tests cobrindo MarkDeleted, Reanimate (zera PII), CanReanimate (4 bordas), SetDisplayNameIfEmpty (vazio→popula, populado→preserva), SetEmailIfEmpty.
- [ ] `entities/whatsapp_history_entry_test.go` — construtor gera ID UUID válido + `LinkedAt` não-zero + `Active=true`.
- [ ] Sanidade compilação: `entities.New(vo)` aceita só VO + opts; chamada com `IDGenerator` falha em compile-time.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/domain/entities/user.go` (criar)
- `internal/identity/domain/entities/user_test.go` (criar)
- `internal/identity/domain/entities/whatsapp_history_entry.go` (criar)
- `internal/identity/domain/entities/whatsapp_history_entry_test.go` (criar)
- Dependências de import (já criadas em 2.0): `domain/entities/id.go`, `domain/valueobjects/{whatsapp_number,email}.go`, `domain/policies.go`.
