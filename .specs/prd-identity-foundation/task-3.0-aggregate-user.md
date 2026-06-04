# Tarefa 3.0: Agregado `User` + `UserID` + `RehydrateUser` + sentinelas de domain

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o agregado `User` em `internal/identity/domain/entities/user.go` com invariantes protegidos no construtor, métodos com intenção de negócio (`MarkAsAdmin`, `RevokeAdmin`, `UpdateEmail`, `SoftDelete`), `UserID` (tipo encapsulado de UUID v4) e o construtor `RehydrateUser` exclusivo de infrastructure (ADR-008). Zero getters mecânicos para mutação; mutações sempre via método com semântica. Sentinelas de domain centralizados em `domain/errors.go`.

<requirements>
- RF-01: `UserID` baseado em UUID v4 imutável, construído via `NewUserID(string) (UserID, error)`.
- RF-06: campo `IsAdmin bool` mutável apenas via `MarkAsAdmin(bool)`; sem tabela de roles.
- RF-07 (método de domínio): `SoftDelete(at time.Time) error` define `DeletedAt` e transita status para `DELETED`; falha se já deletado.
- `NewUser(p NewUserParams) (*User, error)` rejeita `WhatsAppNumber` zero e timestamps zero.
- `RehydrateUser(p RehydrateUserParams) *User` é construtor exclusivo de mapper (sem validação de timestamps — aceita estado vindo do banco). Documentar no godoc com `// Uso restrito ao mapper de infrastructure.`
- OC #9: sem setters; mutações via métodos com intenção (`UpdateEmail`, `MarkAsAdmin`, `RevokeAdmin`, `SoftDelete`).
- R2: sem alias-de-campo (`nome := u.Name` proibido).
</requirements>

## Subtarefas

- [ ] 3.1 Criar `internal/identity/domain/errors.go` com sentinelas: `ErrInvalidUserID`, `ErrUserRequiresNumber`, `ErrUserRequiresTimestamps`, `ErrUserAlreadyDeleted`.
- [ ] 3.2 Criar `internal/identity/domain/entities/user.go` com `UserID`, `NewUserID` (valida UUID v4 via `github.com/google/uuid`), `User`, `NewUserParams`, `NewUser`, `RehydrateUserParams`, `RehydrateUser`, e métodos: `ID()`, `WhatsAppNumber()`, `Email()`, `IsAdmin()`, `Status()`, `DeletedAt()`, `IsDeleted()`, `MarkAsAdmin(at time.Time)`, `RevokeAdmin(at time.Time)`, `UpdateEmail(e valueobjects.Email, at time.Time)`, `SoftDelete(at time.Time) error`.
- [ ] 3.3 Promover `github.com/google/uuid` para dependência direta em `go.mod`.
- [ ] 3.4 Criar `internal/identity/domain/entities/user_test.go` com `UserSuite` (testify/suite) table-driven cobrindo: `NewUser` sucesso, `NewUser` rejeita number zero, `NewUser` rejeita timestamps zero, `SoftDelete` sucesso, `SoftDelete` em user já deletado retorna `ErrUserAlreadyDeleted`, `MarkAsAdmin`/`RevokeAdmin` alteram `is_admin` e `updated_at`, `UpdateEmail` substitui campo e atualiza `updated_at`, `RehydrateUser` aceita estado arbitrário sem validar timestamps.
- [ ] 3.5 Criar `internal/identity/domain/entities/user_id_test.go` cobrindo `NewUserID` com UUID v4 válido, UUID v3/v5 rejeitado, string vazia rejeitada, formato inválido rejeitado.

## Detalhes de Implementação

Ver techspec §"Modelos de Dados" subseções `Agregado entities.User` e tabela final do agregado. ADR-008 documenta o contrato de `RehydrateUser`.

## Critérios de Sucesso

- `NewUserID` valida que o UUID é versão 4 — UUID v3/v5 rejeitado.
- `User` não expõe nenhum setter; campos não exportados.
- `SoftDelete` em user já soft-deletado retorna `ErrUserAlreadyDeleted` (sem mutação).
- `RehydrateUser` aceita timestamps zero (delegação para banco) e status arbitrário do enum.
- `domain/entities` não importa `application`, `infrastructure`, `platform` ou `configs` (depguard).
- Cobertura `go test -cover ./internal/identity/domain/entities/...` ≥ 95%.

## Definition of Done (DoD)

- [ ] `go test -race -count=1 ./internal/identity/domain/entities/...` passa.
- [ ] `go test -cover ./internal/identity/domain/entities/...` reporta ≥ 95%.
- [ ] `grep -rn '^func ' internal/identity/domain/entities/*.go | grep -vE '_test.go|func New|func Rehydrate'` retorna apenas métodos com receiver (R1).
- [ ] `golangci-lint run ./internal/identity/domain/...` passa.
- [ ] `go.mod` lista `github.com/google/uuid` como direta (sem `// indirect`).
- [ ] Nenhum campo público em `User` (verificar via `go doc ./internal/identity/domain/entities User`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit suite para `User`: construtor, mutadores, `SoftDelete` happy/erro.
- [ ] Unit suite para `UserID`: aceita v4, rejeita v3/v5/inválido.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/domain/errors.go` (novo)
- `internal/identity/domain/entities/user.go` (novo)
- `internal/identity/domain/entities/user_test.go` (novo)
- `internal/identity/domain/entities/user_id_test.go` (novo)
- `go.mod` (alterado — promover `google/uuid`)
