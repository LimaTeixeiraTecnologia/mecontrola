# Plano — Refactor `internal/budgets/application/usecases`

## Context

O pacote `internal/budgets/application/usecases` acumulou **5.730 linhas** distribuídas em 14 usecases, e a inspeção mostrou três tipos de inchamento:

1. **Parsing duplicado**: cada `Execute` repete o mesmo bloco `uuid.Parse → NewProducerSource → NewExternalTransactionID → NewCompetence → categorias.Validate → ParseRootSlug`, com 8 sentinel errors locais por arquivo (ex: `upsert_expense.go:121-172`, `delete_expense.go:107-129`, `ingest_external_expense.go:156-192`, `get_monthly_summary.go:48-56`).
2. **Regras de domínio vazadas no application**: máquina de estado de versões (`apply_pending_event.go:74-104`), resolução de estado de alerta com rate-limit (`evaluate_alert.go:187-200`), validação de fonte para recorrência (`create_recurrence.go:101-124`), dispatch de mutation kind (`ingest_external_expense.go:118-127`) — todos são *invariants* de domínio mas vivem no usecase.
3. **Boilerplate transversal**: `Execute` + `ExecuteWithTx` duplicados em `upsert_expense.go` e `delete_expense.go`; envelope de outbox construído duas vezes; span+logger+wrap repetido em todo arquivo.

O resultado é que **adicionar um campo novo a `Expense` exige editar pelo menos 4 usecases**, e regras de negócio (ex: "o que conta como conflito de tombstone?") estão espalhadas — não há um único lugar para ler a verdade.

Objetivo: aplicar DDD tático para puxar regras para o domínio, criar **Domain Command Factories** que substituam o parsing manual, e quebrar `Execute` em funções privadas curtas com early return (estilo idiomático Go, sem generics novos, sem Functional Options). Preservar o contrato externo (assinaturas `Execute`, handlers, módulo) — refactor de estrutura interna, comportamento idêntico.

Decisões do usuário fixadas para este plano:
- **Escopo**: amplo, todos os 14 usecases.
- **Parsing**: Domain Command Factories no pacote `domain/entities` (ou `domain/commands`).
- **Pipeline**: funções privadas pequenas + early return; sem kleisli/generics.
- **Construtores**: manter `New*` explícito; sem Functional Options.

Restrições governamentais aplicadas:
- `go-implementation` SKILL Etapas 1-5; carregar somente `architecture.md` (obrigatório) + `interfaces.md` (factories novas) + `examples-domain-flow.md` (esqueleto domain→service→usecase) + `testing.md` quando reescrever suites. Máximo 4 simultâneas conforme R-economia.
- R-ADAPTER-001 não se aplica (usecases não são adapter), mas R-ADAPTER-001.1 (zero comentários) vale para todo `.go` produzido.
- R6 (interface no consumidor) e R7.6 (`errors.Join`) já cobertos pelo design; preservar.
- Sem `init()`, sem `panic`, sem abstrair tempo (memory `feedback_no_time_abstraction`), sem `var _ Interface = (*T)(nil)` (memory `feedback_no_interface_assertion`).

## Estratégia em camadas

### Camada 1 — Domain Command Factories (novo pacote `internal/budgets/domain/commands`)

Cria um único lugar para transformar primitivos vindos do DTO em um agregado de VOs validado. Cada Command é um struct imutável carregando os VOs prontos para uso pelos usecases e pelas entidades.

Commands a criar:

| Command | Origem (DTO) | Conteúdo (VOs já validados) |
|---|---|---|
| `UpsertExpenseCommand` | `input.UpsertExpenseInput` | `UserID uuid.UUID, Source ProducerSource, ExtID ExternalTransactionID, SubcategoryID uuid.UUID, RootSlug RootSlug, Competence Competence, AmountCents Cents, OccurredAt time.Time, ExpectedVersion *int64` |
| `DeleteExpenseCommand` | `input.DeleteExpenseInput` | `UserID, Source, ExtID, ExpectedVersion` |
| `CreateBudgetCommand` | `input.CreateBudgetInput` | `UserID, Competence, TotalCents Cents, Allocations []AllocationCommand` |
| `IngestExternalExpenseCommand` | `input.IngestExternalExpenseInput` | `UserID, Source, ExtID, MutationKind, Version int64, Payload []byte` |
| `EvaluateAlertCommand` | `input.EvaluateAlertInput` | `UserID, Competence, NowUTC time.Time` |
| `ApplyPendingEventCommand` | `input.ApplyPendingEventInput` | `EventID uuid.UUID` |
| `CreateRecurrenceCommand` | `input.CreateRecurrenceInput` | `UserID, SourceCompetence, TargetCompetence, Mode` |
| `GetMonthlySummaryCommand` | `input.GetMonthlySummaryInput` | `UserID, Competence` |
| `ListAlertsCommand` | `input.ListAlertsInput` | `UserID, Cursor, Limit` |

Cada Command tem um único construtor `NewXxxCommand(in input.XxxInput) (XxxCommand, error)` que:
- chama os `New*`/`Parse*` dos VOs já existentes (`valueobjects.NewCompetence`, `NewProducerSource`, etc.) em sequência;
- retorna **um único erro sentinel por campo** com `errors.Join` quando múltiplos campos forem inválidos (R7.6);
- agrega na própria struct, sem aliases sem transformação (R2).

Erros sentinel **movidos** de cada usecase (`ErrUpsertExpenseInvalidUserID`, `ErrUpsertExpenseInvalidSubcategory`, …) para `commands/errors.go` — exportados, comparáveis com `errors.Is`. Os 8 sentinels duplicados em `upsert_expense.go:21-35` viram 5-6 sentinels canônicos compartilhados (`ErrCommandInvalidUserID`, `ErrCommandInvalidCompetence`, etc.) que cobrem todos os commands.

A factory **não** acessa I/O. O caso de `ValidateExpenseSubcategory` (que fala com `categories`) sai do Command e fica no usecase — Command só faz parsing puro. O Command então recebe `RootSlug` como argumento separado quando aplicável, ou o usecase resolve `RootSlug` antes de montar o agregado.

### Camada 2 — Domain Services novos (em `internal/budgets/domain/services`)

Pega regras hoje inline no application e move para o domínio, junto de `allocation_distributor` e `threshold_evaluator` já existentes.

| Service novo | Substitui | Contrato |
|---|---|---|
| `PendingEventOutcome` | `apply_pending_event.go:74-104` (state machine de versão) | `Decide(event PendingEvent, current *Expense) Outcome` retornando `OutcomeCreate / OutcomeUpdate / OutcomeDelete / OutcomeNoop / OutcomeReject` |
| `AlertStateResolver` | `evaluate_alert.go:187-200, 246-251` | `Resolve(expCompetence, cutoff Competence, prior []Alert) AlertState` |
| `RecurrenceSource` | `create_recurrence.go:101-124` | `Validate(source Budget) error` — encapsula 3 regras de fonte + soma de basis_points |
| `ExternalExpenseStrategy` | `ingest_external_expense.go:118-127, 210-239` | `Plan(cmd IngestExternalExpenseCommand, current *Expense) Action` (`ActionCreate / ActionUpdate / ActionDelete / ActionQueuePending`) |
| `BudgetCloneForRecurrence` | `create_recurrence.go:150-175, 221-235` (rebuild de alocações duplicado) | `Clone(source Budget, target Competence) (Budget, error)` |

Cada service é uma struct sem estado com método `Decide`/`Plan`/`Resolve` — função pura. Sem dependências de infra. Permite teste unitário por table-driven (R4).

### Camada 3 — Refactor dos usecases

Para cada usecase (14 arquivos), aplicar **o mesmo template**:

```go
func (uc *UpsertExpense) Execute(ctx context.Context, in input.UpsertExpenseInput) (output.ExpenseOutput, error) {
    ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.upsert_expense")
    defer span.End()

    cmd, err := commands.NewUpsertExpenseCommand(in)
    if err != nil {
        return output.ExpenseOutput{}, err
    }

    rootSlug, err := uc.resolveRootSlug(ctx, cmd.SubcategoryID)
    if err != nil {
        return output.ExpenseOutput{}, err
    }

    expense, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Expense, error) {
        return uc.persist(ctx, tx, cmd, rootSlug)
    })
    if err != nil {
        uc.logFailure(ctx, span, cmd, err)
        return output.ExpenseOutput{}, err
    }

    return mappers.ExpenseOutput(expense), nil
}
```

Funções privadas curtas por arquivo:
- `resolveRootSlug(ctx, subID) (RootSlug, error)` — única chamada `categories.Validate*`;
- `persist(ctx, tx, cmd, rootSlug) (Expense, error)` — orquestra `existing := repo.Get(...)`, decide via domain service, chama `repo.Insert/Update`, dispara outbox;
- `publishCommitted(ctx, tx, expense, cmd, kind) error` — envelope outbox;
- `logFailure(ctx, span, cmd, err)` — único lugar com `span.RecordError` + `logger.Warn`.

`ExecuteWithTx` em `upsert_expense.go:97-119` e `delete_expense.go:77-98` (que duplica metade do `Execute` para pular `uow.Do`) vira **um helper único** `executeInTx(ctx, tx, cmd) (Expense, error)` chamado tanto pelo `Execute` (dentro do `uow.Do`) quanto pelo `ExecuteWithTx` externo.

### Camada 4 — Mappers e Outbox helpers (novo `internal/budgets/application/mappers`)

- `mappers.ExpenseOutput(e entities.Expense) output.ExpenseOutput` substitui `mapExpenseOutput` duplicado em `upsert_expense.go:308-325`.
- `mappers.BudgetOutput`, `mappers.AlertOutput`, `mappers.SummaryOutput` consolidam todos os mappers de saída hoje espalhados (linhas 113-138 em `create_budget.go`, 87-128 em `list_alerts.go`, 150-200 em `get_monthly_summary.go`).
- `mappers.AlertStateString` (`list_alerts.go:127-139`) idem.

Outbox envelope (`upsert_expense.go:283-306` + idêntico em `delete_expense.go:161-173`) vira `interfaces.NewExpenseCommittedEnvelope(cmd, expenseID, kind, now, cutoff)` no próprio pacote `interfaces`, já que o tipo `ExpenseCommittedEnvelope` mora lá.

## Arquivos a criar

```
internal/budgets/domain/commands/
  errors.go                          (sentinels compartilhados)
  upsert_expense.go                  (UpsertExpenseCommand + NewUpsertExpenseCommand)
  delete_expense.go
  create_budget.go
  ingest_external_expense.go
  evaluate_alert.go
  apply_pending_event.go
  create_recurrence.go
  get_monthly_summary.go
  list_alerts.go
  *_test.go                          (table-driven, testify/suite — R4)

internal/budgets/domain/services/
  pending_event_outcome.go + _test.go
  alert_state_resolver.go + _test.go
  recurrence_source.go + _test.go
  external_expense_strategy.go + _test.go
  budget_clone_for_recurrence.go + _test.go

internal/budgets/application/mappers/
  expense.go
  budget.go
  alert.go
  summary.go
  *_test.go
```

## Arquivos a modificar

- 14 arquivos de usecase em `internal/budgets/application/usecases/` — todos seguem o template da Camada 3.
- `internal/budgets/application/interfaces/outbox.go` (ou onde vive `ExpenseCommittedEnvelope`) — adicionar construtor `NewExpenseCommittedEnvelope`.
- Suites de teste correspondentes (14 `*_test.go`) — adaptar para o novo template. Boa parte simplifica: deixam de testar parsing (coberto agora em `commands/*_test.go`) e foca apenas em I/O + orquestração.
- `internal/budgets/module.go:158-186` — **sem mudança de assinatura**, apenas remover wiring de helpers internos eventuais. Construtores `New*` continuam idênticos.

## Sequência sugerida (paralelizável em fases)

Conforme memory `feedback_subagents_orchestration`, paralelizar por categoria. Sequenciamento mínimo:

1. **Fase 1 — Domain isolado** (paralelo, 1 subagent por grupo):
   - Subagent A: pacote `domain/commands` completo (9 commands + testes).
   - Subagent B: pacote `domain/services` (5 services novos + testes).
   - Subagent C: pacote `application/mappers` (4 mappers + testes).
2. **Fase 2 — Outbox helper** (sequencial, depende só de Fase 1): construtor `NewExpenseCommittedEnvelope`.
3. **Fase 3 — Usecases** (paralelo, 1 subagent por cluster identificado pela análise):
   - Cluster A (expense lifecycle): `upsert_expense`, `delete_expense`, `apply_pending_event`, `ingest_external_expense`.
   - Cluster B (budget lifecycle): `create_budget`, `activate_budget`, `delete_draft_budget`, `create_recurrence`, `create_or_auto_draft_for_expense`.
   - Cluster C (alert/query): `evaluate_alert`, `list_alerts`, `get_monthly_summary`.
   - Cluster D (housekeeping): `purge_retention`, `run_pending_events_reaper`, `signal_abandoned_drafts`.
4. **Fase 4 — Validação consolidada** (sequencial): Etapa 5 do go-implementation.

## Verificação

Por fase + final:

```bash
# R-ADAPTER-001.1 — zero comentários
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/budgets/ | grep -Ev "(//go:|//nolint:|// Code generated)"

# R0, R5.12, R5.11, R7.1 (greps do build.md)
grep -rn "func init()" internal/budgets/
grep -rn "panic(" internal/budgets/ --include="*.go" | grep -v _test.go
grep -rn "interface{}" internal/budgets/ --include="*.go"

# Build + vet + test + race
go build ./...
go vet ./...
go test ./internal/budgets/... -count=1 -race
mockery --config mockery.yml --dry-run
```

Validação de comportamento (preservação):
- Suite de testes existente dos 14 usecases deve passar sem regressão. Onde a suite testava parsing inline, mover a asserção para o teste do Command correspondente — a contagem de cenários cobertos não pode cair.
- Integration tests existentes em `internal/budgets/...` permanecem verdes.

## Não-objetivos

- Não alterar contratos públicos de handler/módulo/repository.
- Não introduzir Functional Options nem generics novos (decidido).
- Não tocar em `internal/budgets/infrastructure/**` exceto `interfaces/` para o helper de envelope.
- Não otimizar performance — refactor estrutural, comportamento idêntico.
- Não adicionar comentários novos em código de produção (R-ADAPTER-001.1).

## Riscos

- **Escopo grande**: 14 usecases + 9 commands + 5 services + 4 mappers. Mitigado pela divisão em clusters paralelos da Fase 3 e pela ordem domain-first (Fases 1-2 destravam tudo).
- **Erros sentinel deslocados**: callers externos (handlers HTTP) podem comparar com os sentinels antigos via `errors.Is`. Auditar `internal/budgets/infrastructure/http/server/handlers/` antes de mover; manter alias `var ErrUpsertExpenseInvalidUserID = commands.ErrCommandInvalidUserID` em `usecases/errors.go` se houver uso externo.
- **`RootSlug` resolvido fora do Command**: cria dois pontos de validação. Mitigação: documentar no Command que `RootSlug` é resolvido pelo usecase via `CategoriesReader` e injetado no agregado de domínio na hora do `entities.NewExpense`.
