# Tarefa 2.0: Value Objects `WhatsAppNumber`, `Email`, `UserStatus` com normalizer/validator privados

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar três Value Objects imutáveis em `internal/identity/domain/valueobjects/`: `WhatsAppNumber` (E.164 BR via struct privada `whatsAppNormalizer`), `Email` (validação stdlib via struct privada `emailValidator`) e `UserStatus` (enum `iota+1`). R1 aplicado integralmente — nenhuma função top-level fora de `New*`. Cobertura 100% obrigatória nos construtores `NewWhatsAppNumber` e `NewEmail` (RF-17). Fuzz test em `NewWhatsAppNumber` garante que input arbitrário nunca panica.

<requirements>
- RF-02: `WhatsAppNumber` imutável construído via `NewWhatsAppNumber(input string) (WhatsAppNumber, error)`.
- RF-03: aceita 10/11/12/13 dígitos após limpeza; com `+55`, com formatação humana; saída sempre `+55DDXNNNNNNNN`.
- RF-04: injeta o 9 nono dígito em entradas de 10 e 12 dígitos começando `55`; rejeita formato fora desses casos com erro tipado.
- RF-05: `Email` imutável construído via `NewEmail(input string) (Email, error)` com validação mínima (`@` + TLD) e lowercase.
- RF-17 (parcial): cobertura 100% em `NewWhatsAppNumber` e `NewEmail`.
- R1 (`go-implementation`): helpers de normalização e validação são métodos de struct privada (`whatsAppNormalizer`, `emailValidator`), não funções top-level.
- R5.8: `UserStatus` usa `iota` com zero reservado para `UserStatusUnknown`.
- VOs expõem `String()`, `Equals(other)`, `IsZero()`.
</requirements>

## Subtarefas

- [ ] 2.1 Criar `internal/identity/domain/valueobjects/whatsapp_number.go` com `WhatsAppNumber`, `whatsAppNormalizer{}` (métodos `KeepDigits` e `NormalizeBR`), `NewWhatsAppNumber`, sentinelas `ErrEmptyWhatsAppNumber`/`ErrInvalidWhatsAppFormat`/`ErrUnsupportedCountry`.
- [ ] 2.2 Criar `internal/identity/domain/valueobjects/whatsapp_number_test.go` com `WhatsAppNumberSuite` (`testify/suite`) table-driven cobrindo: válidos (10, 11, 12, 13 dígitos; com/sem `+55`; formato humano `(11) 98888-7777`); inválidos (vazio, 9 dígitos, 14 dígitos, não-BR, só letras); idempotência.
- [ ] 2.3 Criar `internal/identity/domain/valueobjects/whatsapp_number_fuzz_test.go` com `FuzzNewWhatsAppNumber(f *testing.F)` e corpus seed cobrindo edges; assertiva: nunca panica.
- [ ] 2.4 Criar `internal/identity/domain/valueobjects/email.go` com `Email`, `emailValidator{}` (métodos `Parse` e `HasTLD`), `NewEmail`, sentinelas `ErrEmptyEmail`/`ErrInvalidEmail`.
- [ ] 2.5 Criar `internal/identity/domain/valueobjects/email_test.go` com `EmailSuite` table-driven (válidos com/sem maiúsculas, mixed case → lowercase; inválidos: sem `@`, sem TLD, vazio, espaços).
- [ ] 2.6 Criar `internal/identity/domain/valueobjects/user_status.go` com `UserStatus uint8`, constantes `UserStatusUnknown` (iota), `UserStatusActive`, `UserStatusBlocked`, `UserStatusDeleted`, métodos `String()` e função `ParseUserStatus(s string) (UserStatus, bool)`.
- [ ] 2.7 Criar `internal/identity/domain/valueobjects/user_status_test.go` cobrindo `String()` e `ParseUserStatus` (4 valores + valor inválido).

## Detalhes de Implementação

Ver techspec §"Modelos de Dados" subseções `Value Object valueobjects.WhatsAppNumber`, `Value Object valueobjects.Email`, `Value Object valueobjects.UserStatus`. Normalizer/validator privados conforme decisão de ADR R1 (techspec §"Considerações Técnicas").

## Critérios de Sucesso

- `go test -cover ./internal/identity/domain/valueobjects/...` reporta 100% em `NewWhatsAppNumber` e `NewEmail`.
- Fuzz `go test -run=^$ -fuzz=FuzzNewWhatsAppNumber -fuzztime=10s ./internal/identity/domain/valueobjects/` não detecta panic em 10s.
- Saídas determinísticas: mesma entrada → mesma `WhatsAppNumber.String()`.
- `whatsAppNormalizer` e `emailValidator` são tipos privados (lowercase) — não exportados.
- Nenhuma função top-level além de `New*` e `ParseUserStatus` (este último é exceção autorizada R1 — função utilitária de parsing sem estado).

## Definition of Done (DoD)

- [ ] `go test -cover ./internal/identity/domain/valueobjects/...` ≥ 100% em `NewWhatsAppNumber` e `NewEmail` (verificável via `go test -coverprofile=cover.out && go tool cover -func=cover.out | grep -E 'NewWhatsAppNumber|NewEmail'`).
- [ ] Fuzz `FuzzNewWhatsAppNumber` roda 10s sem crash.
- [ ] `WhatsAppNumberSuite` cobre ≥ 4 DDDs reais (11, 21, 31, 51) em cenários válidos.
- [ ] Todos os erros sentinelados retornam pelo mesmo construtor — `errors.Is(err, valueobjects.ErrInvalidWhatsAppFormat)` funciona.
- [ ] `golangci-lint run ./internal/identity/domain/valueobjects/...` passa (depguard valida que VO não importa `application`/`infrastructure`/`platform`).
- [ ] `grep -rn '^func [a-z]' internal/identity/domain/valueobjects/*.go | grep -v '_test.go'` retorna apenas `func New*` e `func ParseUserStatus` (R1 cumprido).
- [ ] `go vet ./...` passa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit table-driven (suite) para WhatsAppNumber, Email, UserStatus.
- [ ] Fuzz test em `NewWhatsAppNumber`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/domain/valueobjects/whatsapp_number.go` (novo)
- `internal/identity/domain/valueobjects/whatsapp_number_test.go` (novo)
- `internal/identity/domain/valueobjects/whatsapp_number_fuzz_test.go` (novo)
- `internal/identity/domain/valueobjects/email.go` (novo)
- `internal/identity/domain/valueobjects/email_test.go` (novo)
- `internal/identity/domain/valueobjects/user_status.go` (novo)
- `internal/identity/domain/valueobjects/user_status_test.go` (novo)
