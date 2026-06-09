# Tarefa 3.0: Domain do `card` — VOs, agregado, `InvoiceFor` puro, fixtures + property-based

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Entregar o pacote `internal/card/domain/` 100% puro: value objects (`CardName`, `Nickname`, `BillingCycle`), agregado `Card`, sentinels de erro, helper `SaoPauloLocation()` via `sync.Once` e a função pura `BillingCycle.InvoiceFor(purchase, cycle, tz) Invoice`. Cobertura ≥ 95% line coverage com ≥ 50 fixtures table-driven + property-based `quick.MaxCount=10000`.

<requirements>
- `domain` puro: NÃO importar `application`, `infrastructure`, `platform`, banco, HTTP, filas, drivers (R-FRONTEIRA).
- VOs validam invariantes no construtor: `CardName 1..64`, `Nickname 1..32`, `BillingCycle.ClosingDay/DueDay ∈ [1,31]`.
- `InvoiceFor` puro, determinístico, sem IO, sem `time.Now`, sem `clock.Clock` (R6.7).
- Algoritmo conforme RF-02..06 + ADR-002: auto-detect convenção, clamp por `daysInMonth`, regra `closing == due` → `fechamento = due − 1 dia`.
- `SaoPauloLocation()` via `sync.Once`; falha de load encerra processo com `os.Exit(1)` em helper `MustLoadSaoPauloOrExit()` chamado SOMENTE em `module.go` (Tarefa 9.0), NUNCA em `init()` (R0).
- Sem `panic` em runtime (R5.12). Erros via `errors.New`/`fmt.Errorf("ctx: %w", err)` (R7.6).
- Sem `var _ Interface = (*Type)(nil)` (R6.4).
- Zero comentários em arquivos `.go` de produção (`R-ADAPTER-001.1`).
- Helper `NewCardID()` (UUID v4) via `github.com/google/uuid`.
- Property-based test valida invariantes: (a) `due_date >= closing_date`, (b) `due_date >= purchase_date`, (c) idempotência, (d) clamp aplicado.
- ≥ 50 fixtures table-driven cobrindo: fev/28, fev/29 bissexto (2024/2028), abr/jun/set/nov (30 dias), virada dez→jan, `due == closing`, `due > closing`, `due < closing`, `closing=31`, `due=31`, DST histórico BR (2018-10-21, 2018-11-04), horário-padrão 2026.
</requirements>

## Subtarefas

- [ ] 3.1 `domain/valueobjects/card_name.go` + `nickname.go` + `billing_cycle.go` com validação no construtor + sentinels.
- [ ] 3.2 `domain/entities/card.go` — agregado `Card` com `New(input) (Card, error)` e `Hydrate(persisted) Card` para reidratação.
- [ ] 3.3 `domain/errors.go` — sentinels (`ErrCardNotFound`, `ErrNicknameConflict`, `ErrInvalidClosingDay`, `ErrInvalidDueDay`, `ErrInvalidCardName`, `ErrInvalidNickname`, `ErrInvalidPurchaseDate`).
- [ ] 3.4 `domain/services/timezone.go` — `SaoPauloLocation() *time.Location` via `sync.Once` + `MustLoadSaoPauloOrExit(slog.Logger)`.
- [ ] 3.5 `domain/services/billing_cycle.go` — `type Invoice struct{ ClosingDate, DueDate time.Time }` + `BillingCycle.InvoiceFor(purchase, cycle, tz) Invoice`.
- [ ] 3.6 Tests: `billing_cycle_test.go` table-driven (≥ 50 fixtures) + property-based via `testing/quick` (`MaxCount=10000`).
- [ ] 3.7 Tests: `card_name_test.go`, `nickname_test.go`, `billing_cycle_vo_test.go` cobrindo limites.
- [ ] 3.8 Tests: `card_test.go` cobrindo `New` e `Hydrate`.

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Visão Geral dos Componentes — Card — bounded context novo" e `adr-002-invoice-for-algorithm.md`. Não duplicar pseudo-código aqui.

## Critérios de Sucesso

- `go test -race -count=1 -cover ./internal/card/domain/...` ≥ 95% line coverage.
- `quick.Config{MaxCount: 10000}` roda sem falha em < 5s.
- `grep -rn "import" internal/card/domain/` mostra APENAS pacotes `std`, `github.com/google/uuid` e VOs internos.
- Property-based test prova invariantes (a)–(d) listadas em RF-45.
- `go vet ./internal/card/domain/...` + `golangci-lint run ./internal/card/domain/...` limpos.

### Definition of Done

- [ ] Pacote `internal/card/domain/` criado e commitado.
- [ ] Cobertura ≥ 95% reportada via `go test -cover` no CI.
- [ ] ≥ 50 fixtures explícitos contados no diff (`grep -c "^\s*{" billing_cycle_test.go`).
- [ ] Gate de zero comentários verde para `internal/card/domain/...`.
- [ ] RF-01..08, RF-15, RF-37, RF-38, RF-41, RF-42, RF-43, RF-44, RF-45 explicitamente apontados no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: table-driven ≥ 50 fixtures + property-based `quick.MaxCount=10000`.
- [ ] Testes de integração: N/A (`domain` é puro).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/card/domain/valueobjects/{card_name,nickname,billing_cycle}.go` (novo)
- `internal/card/domain/entities/card.go` (novo)
- `internal/card/domain/errors.go` (novo)
- `internal/card/domain/services/timezone.go` (novo)
- `internal/card/domain/services/billing_cycle.go` (novo)
- `internal/card/domain/**/*_test.go` (novo)
- `.specs/prd-card-crud-mvp/adr-002-invoice-for-algorithm.md`
