# Relatorio de Bugfix

- Total de bugs no escopo: 3
- Corrigidos: 3
- Testes de regressao adicionados: 2 (property-based test corrigido + PII regression mantido)
- Pendentes: 0
- Estado final: done

## Bugs

### BUG-001
- ID: BUG-001
- Severidade: major
- Origem: finding de review — Task 10.0 marcada `done` sem evidencia de SLO
- Estado: fixed
- Causa raiz: Status da task 10.0 foi registrado como `done` no `tasks.md` e no `10.0_execution_report.md` apesar de 3 itens do DoD estarem pendentes (thresholds k6, screenshots Grafana, marcacao de M-02/M-03/M-04). A execucao real em homologacao nao foi realizada por falta de ambiente.
- Arquivos alterados:
  - `.specs/prd-card-crud-mvp/tasks.md`
  - `.specs/prd-card-crud-mvp/10.0_execution_report.md`
- Teste de regressao: N/A (correcao de documentacao/status)
- Validacao: Inspecao manual do `tasks.md` e do `10.0_execution_report.md` confirmando status `blocked` e DoD com itens pendentes explicitados.

### BUG-002
- ID: BUG-002
- Severidade: minor
- Origem: RF-45 — property-based test incompleto
- Estado: fixed
- Causa raiz: A funcao `f` em `TestBillingCycle_InvoiceFor_PropertyBased` verificava `!purchaseDayTime.After(closingDayTime)` (purchase <= closing) em vez de `due_date >= purchase_date`. Alem disso, a invariante (d) de RF-45 (`closing_date.day == min(closing_day, daysInMonth(...))`) nao era verificada, exceto para o caso especial `closing_day == due_day` que segue conven propria.
- Arquivos alterados:
  - `internal/card/domain/services/billing_cycle_test.go`
- Teste de regressao: O proprio `TestBillingCycle_InvoiceFor_PropertyBased` foi corrigido para validar as invariantes (a), (b) e (d) de RF-45. A funcao `TestBillingCycle_InvoiceFor_DueDateNeverBeforeClosingDate` e `TestBillingCycle_InvoiceFor_DueDateNeverBeforePurchaseDate` ja existiam como testes de regressao adicionais.
- Validacao:
  - `go test -race -count=1 -run TestBillingCycle_InvoiceFor_PropertyBased ./internal/card/domain/services/...` -> PASS (1.441s)
  - `go test -race -count=1 ./internal/card/domain/services/...` -> PASS (1.315s)

### BUG-003
- ID: BUG-003
- Severidade: minor
- Origem: RF-35 — helper `redactCardLogFields` nao invocado em producao
- Estado: fixed
- Causa raiz: O helper `RedactCardLogFields` existia em `internal/card/infrastructure/observability/redact.go`, mas nenhum handler de producao o invocava. Os handlers logavam `card_id` e `user_id` manualmente. Embora isso nao vazasse PII hoje, a governanca do PRD exige o helper obrigatorio para prevenir regressao futura.
- Arquivos alterados:
  - `internal/card/infrastructure/observability/redact.go` (adicionado `RedactOutputCardLogFields`)
  - `internal/card/infrastructure/http/server/handlers/create.go` (usa `cardobs.RedactOutputCardLogFields(out)`)
  - `internal/card/infrastructure/http/server/handlers/update.go` (usa `cardobs.RedactOutputCardLogFields(out)`)
- Teste de regressao: `TestM07_NoPIIInHandlerLogs` em `handlers/pii_regression_test.go` continua PASS, garantindo que `name`/`nickname` nao aparecem nos logs apos a mudanca.
- Validacao:
  - `go build ./internal/card/infrastructure/http/server/handlers/...` -> exit 0
  - `go test -race -count=1 ./internal/card/infrastructure/http/server/handlers/...` -> PASS (1.469s)
  - `go test -race -count=1 -run TestM07_NoPIIInHandlerLogs ./internal/card/infrastructure/http/server/handlers/...` -> PASS (1.259s)

## Comandos Executados
- `go build ./...` -> exit 0
- `go vet ./...` -> exit 0
- `go test -race -count=1 ./internal/card/... ./internal/platform/idempotency/...` -> todos os pacotes PASS (17 pacotes, 0 falhas)

## Riscos Residuais
- BUG-001: Task 10.0 permanece `blocked` ate que ambiente de homologacao esteja disponivel para execucao dos scripts k6.
- BUG-003: Handlers `get`, `list`, `delete` e `invoice_for` nao foram alterados porque nao logam `output.Card` (get nao loga sucesso; list loga `count`; delete/invoice_for logam apenas `card_id`/`user_id`). O padrao de redact esta estabelecido para handlers que manipulam `output.Card`.
