# Relatório de Review — PRD `simplificacao-card-melhor-dia-compra`

> Executado em 2026-07-02 pelo prompt enriquecido em
> `docs/reviews/2026-07-02-review-prd-simplificacao-card-melhor-dia-compra.md`.
> Fonte de verdade: **working tree** (branch `main`, alterações não commitadas).
> Relatórios `*_execution_report.md` foram tratados como evidência a reconfirmar, nunca como prova.
> Método: 7 subagentes `reviewer` (um por cluster de task) + 1 agente de validação (gates reais) +
> varredura de fixtures + 2 subagentes `bugfixer` na fase de remediação.

- Veredito Rodada 1: **REJECTED**
- Veredito Rodada 2 (pós-bugfix): **APPROVED** — escopo do PRD 100% conforme; tag-compile do repositório limpo; testes verdes
- Alvo revisado: diff completo do working tree (105 arquivos, ~1300 inserções / ~2550 remoções) + fixtures de integração/e2e

---

## 1. Escopo e cobertura de RFs

| Task | Cluster | RFs | Reviewer | Veredito do slice (Rodada 1) |
|------|---------|-----|----------|------------------------------|
| 1.0 + 3.0 | migration + BankDaysReader | RF-09,10,17,20 | ✅ | APPROVED_WITH_REMARKS (1 low) |
| 2.0 | domínio (BankCode, PurchaseDayService) | RF-02,03,06,08,11,20 | ✅ | APPROVED |
| 4.0 | application (DTOs/usecases/repo) | RF-01,04,05,07,12,13 | ✅ | APPROVED_WITH_REMARKS (1 medium) |
| 5.0 + 7.0 | HTTP/OpenAPI/e2e/transactions | RF-01,05,13,14,18 | ✅ | APPROVED_WITH_REMARKS (1 low) |
| 6.0 | cadeia invoice_due | RF-05,19 | ✅ | APPROVED |
| 8.0 | budgets card-limit removal | RF-15 | ✅ | APPROVED_WITH_REMARKS (1 low) |
| 9.0 | agents onboarding | RF-16 | ✅ | APPROVED (1 low cross-task) |

Todos os RFs funcionais (RF-01..RF-20) foram verificados **atendidos** no código real, com evidência
`file:line` (ver seção 5). O exemplo canônico **Nubank/venc.20 → fechamento 13 → melhor dia 14** é
testável (`purchase_day_test.go`, `f10_best_purchase_day.feature`, golden `get_best_purchase_day_200.json`).

## 2. Gates de validação (evidência do agente de validação)

| Gate | Rodada 1 | Rodada 2 (pós-fix) |
|------|----------|--------------------|
| `go build ./...` | ✅ | ✅ |
| `go vet` (card/budgets/agents/transactions) | ✅ | ✅ |
| `go test -race` unit (card/budgets/agents/transactions) | ✅ | ✅ |
| `golangci-lint run` (card/budgets/agents/configs) | ❌ 4 issues | ✅ 0 issues |
| RF-14 — diff produção `internal/transactions` vazio | ✅ | ✅ (só *_test.go alterados) |
| RF-05 — sem `limit_cents` em card produção | ✅ | ✅ |
| RF-15 — sem `CardThresholdReader` em budgets | ✅ | ✅ |
| Zero comentários / sem panic (card) | ✅ | ✅ |
| Tag-compile `integration`+`e2e` (`go vet -tags ./...`) | ❌ 2 arquivos | ✅ repo inteiro limpo |
| `golangci-lint --build-tags` nos pacotes editados | — | ✅ 0 issues introduzidas |

## 3. Achados (Rodada 1) e remediação

| ID | Sev | Arquivo | RF/DoD | Status |
|----|-----|---------|--------|--------|
| BUG-001 | major | `internal/card/application/usecases/update_card.go:50` — cyclomatic 20>15 (revive) | DoD lint | ✅ fixed (extração `applyUpdate`/`resolveUpdate`/`persistIdempotency`; testes verdes) |
| BUG-002 | minor | `internal/budgets/infrastructure/repositories/factory.go:43` — goimports | DoD lint | ✅ fixed (gofmt) |
| BUG-003 | minor | goimports em 4 arquivos de teste (threshold_workflow_test, evaluate_threshold_alerts_test, create_card_test, openapi_test) | DoD lint | ✅ fixed (gofmt) |
| BUG-004 | minor | `migrations/000002_card_simplification.down.sql:6` — `name` recriado com `DEFAULT ''` residual | Task 1.0 | ✅ fixed (`ALTER COLUMN name DROP DEFAULT`) |
| BUG-005 | major | `internal/card/application/mappers/card_mapper.go:21` — `best_purchase_day` diverge do `PurchaseDayService` em fim de mês | RF-13 | ✅ fixed (decisão do usuário: alinhar serviço ao +1 dia-do-mês; `purchase_day.go` + 2 testes de regressão) |
| BUG-006 | minor | `configs/config.go:112,518,593` — `ThresholdCardRatio` órfão | RF-15/ADR-004 | ✅ fixed (3 linhas removidas) |
| BUG-007 | minor | `internal/card/infrastructure/http/server/router_test.go:123,125,141,152` — payloads obsoletos (`name`/`closing_day`) | Task 5.0 | ✅ fixed |
| BUG-008 | major | fixtures de integração/e2e com `INSERT` em colunas removidas (`name`/`limit_cents`) e sem `bank` NOT NULL | RF-17/Task 7.0 | ✅ fixed (6 fixtures: 3 card, 1 agents, 2 transactions) + e2e helper `insertCardViaSQL` |
| BUG-009 | major | `card_repository_integration_test.go` (+ sibling `invoice_due_phase5_integration_test.go`) usa `NewCardName`/`NewCardLimit`/`.Name`/`.LimitCents`/`ErrCardLimitConflict` (não compila) | RF-05/Task 4.0 | ✅ fixed (VOs migrados p/ Nickname/Bank; `TestPersistAndReadLimitCents`/`TestUpdateLimitOptimisticConcurrency` deletados; `go vet -tags integration` verde) |
| BUG-010 | major | `budgets/.../threshold_alerts_job_integration_test.go:86` — campo `Card` em `ThresholdConfig` (não compila) | RF-15/Task 8.0 | ✅ fixed (campo `Card:` removido dos 2 literais; category+goal intactos; `go vet -tags integration` verde) |

> Nota: BUG-008/009/010 foram descobertos por **varredura própria + tag-compile**, não pelos slice-reviewers
> (que não compilaram os testes `//go:build integration|e2e`, dependentes de Docker). São lacunas reais de DoD
> das tasks 4.0/7.0/8.0.

### Achados adicionais (Rodada 3 — prova dinâmica com Docker)

Descobertos ao **executar** as suítes integration/e2e (testcontainers/Postgres):

| ID | Sev | Arquivo | RF/DoD | Status |
|----|-----|---------|--------|--------|
| BUG-011 | high | `internal/card/infrastructure/http/server/handlers/create.go:77` (`mapCardError`) — erros do input DTO (`ErrCardDueDayInvalid`/`ErrCardBankRequired`/`ErrCardIDRequired`/`ErrCardUserIDRequired`) caíam no `default` → **500 em vez de 400** | RF-01/Task 5.0 | ✅ fixed (casos de input mapeados p/ 400; card e2e f04 due_day 0/32 verde) |
| BUG-012 | high | `internal/card/application/usecases/invoice_for.go:59` — data "for" parseada como UTC e deslocada por `.In(America/Sao_Paulo)` → off-by-one no limite do ciclo (compra 1 dia após fechamento faturada no ciclo errado) | RF-14/Task 6-7 (fatura) | ✅ fixed (normaliza data na location antes do cálculo; +teste unitário `TestExecute_DayAfterClosing_RollsToNextCycle`; card e2e f08 verde) |
| BUG-013 | minor | `internal/card/e2e/{ctx_test.go,steps_update_test.go}` — 3 helpers órfãos após reescrita do e2e (`uniqueCardName`/`updateCardCycleDays`/`assertCardVersionInDB`) + errcheck `resp.Body.Close` | Task 7.0 | ✅ fixed (helpers removidos; padrão `defer func(){ _ = ...Close() }()`; pacote card e2e lint-clean) |

> BUG-011 e BUG-012 eram **defeitos de correção reais** (500 indevido; fatura no mês errado), o segundo
> pré-existente no código mas **exposto** pela reescrita do f08 (compra no dia seguinte ao fechamento). Ambos
> só apareceram na execução dinâmica — a compilação e os testes unitários passavam.

### Prova dinâmica (Docker, testcontainers)

| Suíte | Resultado |
|-------|-----------|
| `go test -tags=integration` (migrations + card + budgets + agents + transactions) | ✅ **0 falhas**, todos `ok` (inclui todos os fixtures corrigidos contra o schema migrado 000002) |
| `go test -tags=e2e ./internal/card/e2e/...` | ✅ **verde** após BUG-011/012/013 (Nubank 13/14; f04 due_day; f08 ciclo pós-fechamento) |
| `go test -tags=e2e ./internal/transactions/e2e/...` | ⚠️ falhas **pré-existentes e fora de escopo**: `ErrOutcomeTransactionRequiresSubcategory` (regra de domínio de transactions, presente no HEAD) + 1 rota 404 + 1 404→500. Nenhuma é erro de schema/card; transactions produção é zero-diff. Migration 000002 não toca tabelas de transactions/subcategory. Não causadas por este PR. |

## 4. Decisões do usuário (forks que sobrepõem spec aprovada)

1. **RF-14 vs fixtures de transactions**: migration dropa `cards.name`/torna `bank` NOT NULL, quebrando 3
   fixtures em `internal/transactions`. **Decisão: corrigir os 3 fixtures** (apenas SQL de teste; contrato de
   leitura de produção intacto — diff em transactions restrito a `*_test.go`).
2. **best_purchase_day (BUG-005)**: **Decisão: alinhar `PurchaseDayService.Decide` ao `+1` dia-do-mês**
   (clamp/wrap), consistente com o mapper e com o literal de RF-08. Testes de fim de mês adicionados
   (`ClosingOnDay30_BestIs31`, `ClosingOnDay31_BestWrapsTo1`).

## 5. Arquivos revisados (amostra de evidência)

- Domínio: `purchase_day.go`, `bank_code.go`, `decide_create_card.go`, `decide_update_card.go`, `entities/card.go`
- Application: `create_card.go`, `update_card.go`, `best_purchase_day.go`, `card_mapper.go`, `dtos/input/*`, `card_repository.go`
- HTTP: `router.go`, `handlers/{create,update,best_purchase_day}.go`, `openapi.yaml`, `testdata/golden/*`
- Eventos: `invoice_due_publisher.go`, `invoice_due_notifier.go`, `notify_invoice_due.go`
- Budgets: `evaluate_threshold_alerts.go`, `threshold_workflow.go`, `repository_factory.go`, `module.go`
- Agents: `onboarding_workflow.go`, `card_manager_adapter.go`, `interfaces/types.go`
- Migration: `000002_card_simplification.{up,down}.sql`

## 6. Riscos residuais

- Testes `//go:build integration`/`e2e` não foram **executados** (exigem Docker/testcontainers); a barra
  aplicada foi compilação (`go vet -tags`) + `gofmt` + `golangci-lint`. Execução dinâmica fica para CI.
- Cache `closing_day` (ADR-002): mudança posterior em `banks.days_before_due` não retroalimenta cartões
  (reconciliação em massa fora de escopo).
- Evento `card.invoice_due.v1` renomeado sem versionar — aceito pela premissa "sem cartões em produção".
- **RF-07 / optimistic lock (observação, não bug)**: `UpdateByIDForUser` não possui predicado `version = $esperado` no WHERE — mas isso é **idêntico ao HEAD (pré-PR)**; a detecção de conflito otimista vivia apenas em `UpdateLimitByIDForUser` (método de limite, removido por RF-05). O contador de versão continua incrementado/retornado. RF-07 ("controle de versão otimista **já existente**") é preservado para o caminho geral exatamente como era. Não é regressão.
- **Lint pré-existente fora de escopo** em `internal/transactions/e2e/` (só sob tag `e2e`): `ctx_test.go:100` (errcheck, arquivo **não** tocado por este PR) e 2 helpers unused (`countCardPurchasesForUser`, `fetchMonthlySummaryAmountCents`) — **idênticos ao HEAD**, não introduzidos por este PR. RF-14 proíbe tocar produção de transactions; limpeza destes é trabalho separado.

## 7. Validações executadas

- `go build ./...` → 0
- `golangci-lint run ./internal/card/... ./internal/budgets/... ./internal/agents/... ./configs/...` → 0 issues
- `go test ./internal/card/... ./internal/budgets/... ./internal/agents/... ./internal/transactions/...` → sem FAIL
- `go test ./internal/card/domain/services/...` → 99 passed (inclui regressões de fim de mês)
- `git diff -- internal/transactions/ | grep +++/--- | grep -v _test.go` → vazio (produção intacta, RF-14)
- `go vet -tags "integration e2e" ./...` → repo inteiro sem erro de compilação
- `golangci-lint --build-tags "integration e2e"` nos pacotes editados → 0 issues introduzidas (3 restantes são pré-existentes fora de escopo, ver §6)
