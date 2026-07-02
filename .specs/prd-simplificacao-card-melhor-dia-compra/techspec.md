<!-- spec-hash-prd: 5384373f31eb476dea92e45e83a8872200c7a32d552818507909436676973663 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Simplificação do CRUD de `internal/card` e "Melhor Dia de Compra"

> PRD: `.specs/prd-simplificacao-card-melhor-dia-compra/prd.md` (spec-version 2, RF-01..RF-20).
> Governança obrigatória: `.agents/skills/go-implementation/SKILL.md` (Etapas 1-5, R0-R7),
> `domain-modeling.md` (DMMF adaptado), `.claude/rules/{go-adapters,go-testing,input-dto-validate,
> transactions-workflows,governance}.md`. Go 1.26.4. Zero comentários em `.go` de produção `[HARD]`.
> Todo código Go ancorado por leitura direta (file:line) do repositório atual.

## Resumo Executivo

Esta entrega reduz o domínio `internal/card` a três dados de negócio de entrada — **apelido**,
**banco emissor** e **dia de vencimento** — e move o dia de fechamento (`closing_day`) e o "melhor
dia de compra" para **derivação determinística** a partir de uma **tabela persistida de bancos**
(`mecontrola.banks`, banco→`days_before_due`, fallback 7 dias). O `closing_day` continua **persistido
como cache** na tabela `cards`, o que preserva integralmente o contrato de leitura consumido por
`internal/transactions` (ver ADR-002). O limite de cartão (`limit_cents`, `CardLimit`,
`UpdateCardLimit`, rota `PATCH /cards/{id}/limit`) é **removido por completo**, e a feature dependente
de alerta de limite de cartão em `internal/budgets` é removida **cirurgicamente**, preservando os
alertas de categoria e metas (ADR-004).

A estratégia segue DMMF seletivo: um novo value object `BankCode` (smart constructor + normalização,
Princípio 1/2), um novo domain service **puro** `PurchaseDayService.Decide` (Princípio 6, sem IO, sem
`context.Context`, recebe `now`/`tz` explícitos) que reutiliza a aritmética de calendário já provada
em `billing_cycle.go` (`clamp`/`advanceMonth`), e um output tipado `PurchaseDay{ClosingDay, BestDay}`.
A resolução do número de dias (IO) fica no shell do use case; o cálculo (puro) fica no domínio. O
consolidamento de `Name`+`Nickname` em um único campo (`Nickname`) e a adição de `bank` reescrevem
DTOs, mapper, repositório, handlers, OpenAPI, evento `card.invoice_due.v1` e o onboarding de
`internal/agents` (incluindo a correção do drift `ClosingDay = DueDay`).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos:**
- `internal/card/domain/valueobjects/bank_code.go` — VO `BankCode` (smart constructor, normalização determinística). RF-03, RF-20.
- `internal/card/domain/services/purchase_day.go` — domain service **puro** `PurchaseDayService.Decide` + output tipado `PurchaseDay{ClosingDay, BestDay int}`. RF-08, RF-11, RF-12.
- `internal/card/application/interfaces/bank_days_reader.go` — porta `BankDaysReader` (consumidor). RF-09, RF-10.
- `internal/card/infrastructure/repositories/postgres/bank_repository.go` — adapter Postgres do `BankDaysReader` (lookup em `mecontrola.banks`, fallback 7). RF-09, RF-10.
- `internal/card/application/usecases/best_purchase_day.go` — use case de consulta pura. RF-12, RF-13.
- `internal/card/application/dtos/input/best_purchase_day.go` + `output/best_purchase_day.go` — DTOs da consulta. RF-13.
- `internal/card/infrastructure/http/server/handlers/best_purchase_day.go` — handler `GET /cards/best-purchase-day`. RF-13.
- `migrations/000002_card_simplification.up.sql` / `.down.sql` — cria `mecontrola.banks` + seed, adiciona `cards.bank`, remove `cards.limit_cents`. RF-10, RF-17.

**Modificados (card):** `domain/entities/card.go`, `domain/services/{decide_create_card,decide_update_card,decide_invoice_due_alerts}.go`, `application/usecases/{create_card,update_card,evaluate_invoice_due_alerts}.go`, `application/dtos/input/{create_card,update_card}.go`, `application/dtos/output/card.go`, `application/mappers/card_mapper.go`, `application/interfaces/repository.go`, `infrastructure/repositories/postgres/card_repository.go`, `infrastructure/http/server/handlers/{create,update}.go`, `infrastructure/http/server/router.go`, `messaging/database/producers/invoice_due_publisher.go`, `messaging/database/consumers/invoice_due_notifier.go`, `application/usecases/notify_invoice_due.go`, `module.go`, `openapi.yaml`.

> **Cadeia `invoice_due` (RF-05, RF-19, ADR-005):** `decide_invoice_due_alerts.go:14-16,22-23,52-53`
> tem dois structs (candidato + alerta) com `CardName` e `LimitCents`; `evaluate_invoice_due_alerts.go:102-104`
> os popula de `c.Name`/`c.LimitCents`. Após esta entrega: `LimitCents` é **removido** desses structs, do
> payload do producer, do input/consumer e do texto de `NotifyInvoiceDue` (RF-19). O campo `CardName` é
> **renomeado para `CardNickname`** e a chave JSON `card_name` do payload `card.invoice_due.v1` vira
> **`card_nickname`** (producer `invoice_due_publisher.go:22-29`, consumer `invoice_due_notifier.go:23-30`,
> `NotifyInvoiceDueInput`), alimentado por `c.Nickname.String()`. Sem cartões/uso em produção e consumidor
> no mesmo repo ⇒ renomeação sem versionamento de evento; elimina campo semanticamente enganoso (regra HARD
> "zero campo morto/enganoso" — ADR-005).

**Removidos (card):** `domain/valueobjects/card_limit.go` (+test), `domain/valueobjects/card_name.go` (+test), `application/dtos/input/update_card_limit.go`, `application/usecases/update_card_limit.go` (+test), `infrastructure/http/server/handlers/update_card_limit.go` (+test), método `Card.UpdateLimit` (`entities/card.go:93-98`), método `CardRepository.UpdateLimitByIDForUser` + query `card_repository.go:346-353`.

**Removidos/Modificados (budgets — ADR-004):** ver a lista fechada em §"Remoção cirúrgica em budgets".

**Modificados (agents):** `application/workflows/onboarding_workflow.go`, `application/interfaces/types.go`, `infrastructure/binding/card_manager_adapter.go`. RF-16.

### Fluxo de Dados

**Criar cartão (RF-01, RF-04):**
```
POST /cards {nickname, bank, due_day}
  -> createHandler (adapter fino) -> input.CreateCard.Validate()
  -> CreateCard.Execute (shell):
       days := BankDaysReader.DaysBeforeDue(ctx, bankCode)   [IO, fallback 7]
       pd   := PurchaseDayService.Decide(dueDay, days, now, tz)   [PURO -> closing/best]
       cmd  := CreateCardCommand{UserID, Nickname, Bank, Cycle{ClosingDay: pd.ClosingDay, DueDay}}
       card := CreateCardDecider.Decide(cmd, id, now)   [PURO]
       repo.Insert(card)   [IO]
  -> output.Card {..., closing_day: derived, best_purchase_day: derived}
```

**Consulta pura de melhor dia (RF-12, RF-13):**
```
GET /cards/best-purchase-day?bank=nubank&due_day=20
  -> handler -> input.BestPurchaseDay.Validate()
  -> BestPurchaseDay.Execute: days := reader.DaysBeforeDue(...); pd := PurchaseDayService.Decide(...)
  -> output {closing_day: 13, best_purchase_day: 14}   (não persiste, não exige cartão)
```

**Compra de cartão em transactions (RF-14 — inalterado):**
```
create_card_purchase -> CardLookup.GetForUser -> lê cycle.ClosingDay/DueDay do cartão
  -> BillingCycleResolver usa ClosingDay+DueDay (agora derivados-e-cacheados) -> contrato preservado
```

## Design de Implementação

### Interfaces Chave

Porta consumidora (DMMF: interface no consumidor, R6.3). O fallback de 7 dias é resolvido **dentro do
adapter** — o consumidor recebe sempre um número de dias válido, nunca "não encontrado".

```go
package interfaces

type BankDaysReader interface {
    DaysBeforeDue(ctx context.Context, bank valueobjects.BankCode) (int, error)
}
```

O adapter consulta `mecontrola.banks WHERE code = bank.LookupKey()`; ausência de linha ⇒ retorna **7**
(fallback, não erro). O consumidor recebe sempre um `int` válido.

Domain service **puro** (Princípio 6 — sem `context.Context`, sem IO, `now`/`tz` explícitos):

```go
package services

type PurchaseDay struct {
    ClosingDay int
    BestDay    int
}

type PurchaseDayService struct{}

func (PurchaseDayService) Decide(dueDay, daysBeforeDue int, now time.Time, tz *time.Location) PurchaseDay {
    ref := now.In(tz)
    due := time.Date(ref.Year(), ref.Month(), clamp(dueDay, ref.Year(), ref.Month()), 0, 0, 0, 0, tz)
    closing := due.AddDate(0, 0, -daysBeforeDue)
    best := closing.AddDate(0, 0, 1)
    return PurchaseDay{ClosingDay: closing.Day(), BestDay: best.Day()}
}
```

`clamp`/`advanceMonth`/`daysInMonth` já existem em `billing_cycle.go:62-76`; `Decide` reutiliza `clamp`
(mesma unidade de compilação `services`) — aritmética de data real cobre virada de mês e meses curtos
(RF-11). Nubank/venc.20/7d: `due=dia20`, `closing=dia13`, `best=dia14` (RF-13, critério de aceitação).

### Modelos de Dados

**VO `BankCode`** (`bank_code.go`) — smart constructor que **preserva o texto original** para exibição e
**deriva a chave de lookup** por normalização (RF-03, RF-20, ADR-001). Texto livre: qualquer valor
não-vazio é aceito; banco desconhecido **não** é erro (o fallback ocorre no reader, não aqui).

```go
package valueobjects

type BankCode struct{ display string }

func NewBankCode(raw string) (BankCode, error) {
    display := strings.TrimSpace(raw)
    if display == "" || normalizeBank(display) == "" {
        return BankCode{}, domain.ErrInvalidBank
    }
    return BankCode{display: display}, nil
}

func (b BankCode) String() string   { return b.display }
func (b BankCode) LookupKey() string { return normalizeBank(b.display) }
```

`cards.bank` persiste o **texto original** (`String()`, ex.: `"Banco do Brasil"`); o adapter
`BankDaysReader` consulta `mecontrola.banks` por `LookupKey()` (ex.: `"banco-do-brasil"`).
`normalizeBank` = trim + lowercase + remoção de acentos/diacríticos + colapso de espaços internos para
`-` (`"NuBank"` → `"nubank"`). Determinística e única, reutilizada por cadastro, endpoint de consulta e
onboarding. A resposta da API exibe `bank` = texto original.

**Entidade `Card`** (`entities/card.go`) — remove `LimitCents` e o VO `CardName`; consolida em
`Nickname`; adiciona `Bank`:

```go
type Card struct {
    ID        uuid.UUID
    UserID    uuid.UUID
    Nickname  valueobjects.Nickname
    Bank      valueobjects.BankCode
    Cycle     valueobjects.BillingCycle
    Version   int64
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt *time.Time
}
```

`NewCardInput`/`HydrateCard`/`HydrateCardWithVersion` perdem `Name` e `LimitCents`, ganham `Bank`. O
método `Card.UpdateLimit` (`card.go:93-98`) é removido.

**Tabela `mecontrola.banks`** (nova, RF-10) — lookup de derivação, **sem** FK a partir de `cards`
(ADR-001). Seed idempotente no padrão `billing_plans`/`categories` (`ON CONFLICT DO NOTHING`).

```sql
CREATE TABLE IF NOT EXISTS mecontrola.banks (
    code            TEXT     NOT NULL PRIMARY KEY,
    name            TEXT     NOT NULL,
    days_before_due SMALLINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT banks_code_len_chk  CHECK (char_length(code) BETWEEN 1 AND 64),
    CONSTRAINT banks_name_len_chk  CHECK (char_length(name) BETWEEN 1 AND 128),
    CONSTRAINT banks_days_chk      CHECK (days_before_due BETWEEN 1 AND 28)
);
INSERT INTO mecontrola.banks (code, name, days_before_due) VALUES
    ('nubank','Nubank',7), ('itau','Itaú',8), ('santander','Santander',8),
    ('bradesco','Bradesco',7), ('banco-do-brasil','Banco do Brasil',7),
    ('caixa','Caixa',7), ('inter','Inter',7), ('c6-bank','C6 Bank',7)
ON CONFLICT (code) DO NOTHING;
```

**Tabela `mecontrola.cards`** (alteração, RF-17) — sem cartões em produção ⇒ sem backfill:
```sql
ALTER TABLE mecontrola.cards DROP CONSTRAINT cards_limit_cents_chk;
ALTER TABLE mecontrola.cards DROP COLUMN limit_cents;
ALTER TABLE mecontrola.cards DROP CONSTRAINT cards_name_len_chk;
ALTER TABLE mecontrola.cards DROP COLUMN name;
ALTER TABLE mecontrola.cards ADD COLUMN bank TEXT NOT NULL DEFAULT '';
ALTER TABLE mecontrola.cards ADD CONSTRAINT cards_bank_len_chk CHECK (char_length(bank) BETWEEN 1 AND 64);
ALTER TABLE mecontrola.cards ALTER COLUMN bank DROP DEFAULT;
```
> `name` é **dropado** (ADR-005: identificação consolidada em `nickname`; o índice único parcial
> `cards_user_nickname_active_uniq_idx` por `(user_id, nickname)` permanece). O `closing_day`
> **permanece** (agora populado por derivação, não por entrada — ADR-002). O `.down.sql` recria
> `limit_cents`, `name` e seus CHECK, e remove `bank`/`banks`.

### Contrato do repositório (`application/interfaces/repository.go`)

`CardRepository` (`repository.go:16-25`) perde `UpdateLimitByIDForUser`; `Insert`/`UpdateByIDForUser`/
scans passam a manipular `bank` no lugar de `name`+`limit_cents`. Queries a atualizar em
`card_repository.go`: INSERT (`:39-43`), GET (`:81-85`), UPDATE (`:288-299`), LIST (`:194-215`),
FIND_DUE (`:127-134`), `scanCard` (`:419-470`); remover UPDATE_LIMIT (`:346-353`).

### Endpoints de API

- `GET /api/v1/cards/best-purchase-day?bank=<str>&due_day=<1..31>` — consulta pura; resp `{closing_day, best_purchase_day}` (RF-13). **Sob o grupo autenticado** `/api/v1/cards` (herda `gatewayAuth` + `RequireUser` + rate limit de `router.go:70-74`), apesar de o cálculo não usar o principal — sem nova superfície pública.
- `POST /api/v1/cards` — body `{nickname, bank, due_day}`; resp `Card` com `closing_day`/`best_purchase_day` derivados (RF-01, RF-18).
- `PUT /api/v1/cards/{id}` — body `{nickname?, bank?, due_day?}`; recomputa `closing_day` ao mudar `bank`/`due_day` (RF-07).
- **Removida:** `PATCH /api/v1/cards/{id}/limit` (RF-05).
- OpenAPI (`openapi.yaml`): `CreateCardRequest`/`UpdateCardRequest` perdem `closing_day`/`limit_cents` (entrada) e ganham `bank`; `UpdateCardLimitRequest` removido; `Card` (resposta) perde `limit_cents`, ganha `best_purchase_day`, mantém `closing_day` (derivado). RF-18.

> Nota de ordering de rota chi: registrar `GET /best-purchase-day` **antes** de `Route("/{id}")` em
> `router.go:76-89`, senão `best-purchase-day` é capturado como `{id}`. Gate de teste em `router_test.go`.

## Pontos de Integração

- **`internal/transactions` (RF-14):** consome `cycle.ClosingDay`/`DueDay` via
  `card_lookup_adapter.go:56` e `billing_cycle_resolver.go:17-18`. Como ambos permanecem persistidos e
  legíveis, **nenhuma alteração** em transactions. Gate: suíte de transactions verde sem diff.
- **`internal/budgets` (RF-15 — remoção cirúrgica):** ver §abaixo.
- **`internal/agents` (RF-16):** onboarding passa a coletar `bank`; adapter corrige o drift e remove `LimitCents`.

### Remoção cirúrgica em budgets (ADR-004)

Preserva `ThresholdAlertCategory` e `ThresholdAlertGoal` (feature core intacta). **Remover arquivos:**
`internal/budgets/application/interfaces/card_threshold_reader.go`,
`internal/budgets/infrastructure/repositories/postgres/card_threshold_reader.go`,
`internal/budgets/application/interfaces/mocks/card_threshold_reader.go`. **Editar:**
`domain/services/threshold_workflow.go` (remover const `ThresholdAlertCardLimit` e seus `case` em
`String()` e `thresholdForKind()`), `application/interfaces/repository_factory.go` (remover método
`CardThresholdReader`), `infrastructure/repositories/factory.go` (remover impl), `application/usecases/
evaluate_threshold_alerts.go` (remover `cardReader`, `activeCards`, `buildCardSnapshots`; ajustar guarda
para `if len(active) == 0`), `application/usecases/notify_threshold_alert.go` (remover `case`),
`infrastructure/repositories/postgres/threshold_alert_sent_repository.go` e
`infrastructure/messaging/database/consumers/threshold_alert_notifier.go` (remover `case
"card_limit_near"` em parse), `module.go` (remover `cfg.Card`/`cardRatio` do `ThresholdConfig`), mocks
regenerados. **Testes:** remover `TestDecideAlerts_CardLimit` e os 3 testes de card em
`evaluate_threshold_alerts_test.go`; ajustar os 5 testes que faziam `EXPECT().CardThresholdReader(...)`.
Impacto de observabilidade: o label `kind="card_limit_near"` deixa de ser emitido em
`budgets_threshold_alerts_dispatched_total` (ver §Monitoramento).

## Abordagem de Testes

### Testes Unitários

Padrão canônico `testify/suite` (`.claude/rules/go-testing.md`, R-TESTING-001): whitebox, `fake.NewProvider()`, `dependencies` struct com IIFE por mock, SUT instanciado dentro de `s.Run`.

- **`PurchaseDayService.Decide`** (domain puro, sem mock): tabela cobrindo Nubank/20→(13,14); Itaú/8d/venc.10→(2,3); venc. pequeno com wrap (venc.3/7d → mês anterior); venc.31 em mês curto (clamp); todos os 8 bancos; fallback (dias=7). Determinístico com `now` fixo (sem `time.Now()`).
- **`BankCode.NewBankCode`/`normalizeBank`**: acentos, caixa, espaços→`-`, vazio→erro.
- **`BankDaysReader` (adapter)**: retorna dias do seed; banco ausente → **7** (fallback), não erro. (integração — ver abaixo.)
- **`CreateCard`/`UpdateCard`**: mock de `BankDaysReader` + repo; verificam que `closing_day` gravado = derivado; update recomputa ao mudar `bank`/`due_day`; erro do reader propaga.
- **`BestPurchaseDay.Execute`**: mock reader; valida output sem persistência.
- **`CreateCardDecider`/`UpdateCardDecider`**: pureza mantida; sem `Name`/`LimitCents`.
- **budgets:** suíte de `evaluate_threshold_alerts` continua verde para categoria/metas após remoção.

### Testes de Integração

Critérios do template atendidos (fronteira de IO crítica em Postgres; migration com seed; contrato consumido cross-module) ⇒ **integration tests recomendados** (`//go:build integration`, testcontainers já em uso — `migrations_integration_test.go`, `card_repository_integration_test.go`).

- **Migration** `000002`: `migrations_integration_test.go` — up/down/up idempotente; asserir `mecontrola.banks` presente com 8 linhas e `days_before_due` corretos; `cards.limit_cents` ausente; `cards.bank` presente com CHECK.
- **`bank_repository`**: seed lido; código inexistente → fallback 7.
- **`card_repository`**: insert/get/list/update com `bank`, sem `limit_cents`.
- **transactions**: suíte existente verde sem alteração (prova RF-14).

### Testes E2E

`internal/card/e2e/` (godog): atualizar `steps_create_test.go`/`steps_update_test.go`/`steps_read_list_test.go` para o novo contrato (`bank`, sem `limit_cents`/`closing_day` de entrada; resposta com `best_purchase_day`); remover passos de `update_card_limit`; novo cenário para `GET /best-purchase-day` (Nubank/20→14). Contrato: `contract_openapi_test.go`/`openapi_test.go` revalidam o `openapi.yaml`.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Migration `000002`** (banks + seed; altera cards) — base de tudo; valida via integration.
2. **Domínio card**: `BankCode` VO; `PurchaseDayService` puro (+ testes unitários); ajuste `entities/Card`, deciders, errors (`ErrInvalidBank`; remover `ErrCardLimit*`, `ErrInvalidCardName`).
3. **Porta + adapter**: `BankDaysReader` interface + `bank_repository` (fallback 7); wire no `factory.go`/`module.go`.
4. **Application card**: DTOs (`create/update` reescritos, `best_purchase_day` novos; remover `update_card_limit`); use cases (`CreateCard`/`UpdateCard` recompute; `BestPurchaseDay` novo; remover `UpdateCardLimit`); mapper; `repository.go` (remover `UpdateLimitByIDForUser`).
5. **Infra card**: `card_repository` SQL (bank, sem limit); handlers (`create`/`update` reescritos, `best_purchase_day` novo, remover `update_card_limit`); `router.go` (rota nova antes de `{id}`, remover `/limit`); producer/consumer/notify (remover `limit_cents`); `module.go` wiring.
6. **OpenAPI** + testes de contrato + e2e.
7. **budgets** remoção cirúrgica (ADR-004) — independente de 2-6, pode paralelizar; regenerar mocks.
8. **agents** onboarding (RF-16): schema+prompt+`DecideCardEntry(bank)`; `NewCard.Bank`/`Card.Bank`; adapter corrige drift (`DueDay` correto, `Bank`, remove `LimitCents`).
9. Validação final R0-R7 + gates de regra (§Conformidade).

### Dependências Técnicas

- Postgres (migration + testcontainers). `gen_random_uuid`/`pgcrypto` não é necessário (PK textual `code`).
- Mockery para regenerar mocks de card (`CardRepository`, novo `BankDaysReader`) e budgets (`RepositoryFactory` sem `CardThresholdReader`).

## Monitoramento e Observabilidade

- **Sem novas métricas de alta cardinalidade.** `bank` **não** pode ser label de métrica (herda R-TXN-004 / R-WF-KERNEL-001.4): cardinalidade aberta (texto livre). Se instrumentar a consulta de melhor-dia, usar contador simples sem label de banco, ou label `bank_known={true,false}` (cardinalidade 2).
- **Regressão de dashboard/alerta:** o label `kind="card_limit_near"` de `budgets_threshold_alerts_dispatched_total` deixa de existir. Auditar `docs/dashboards/*` e `docs/alerts/*budgets*` para remover séries/queries que filtram esse valor (evita painel órfão). `category_threshold` e `goal_achieved` permanecem.
- Spans já seguem o padrão `module.usecase.<nome>` (ex.: `card.usecase.best_purchase_day`), tracer via `o11y.Tracer().Start` no início do `Execute` (R-DTO-002: `Validate()` logo após `defer span.End()`).

## Considerações Técnicas

### Decisões Chave

- **ADR-001** — `bank` como texto normalizado no cartão, sem FK; `mecontrola.banks` é lookup de derivação com fallback 7. (`adr-001-bank-free-text-no-fk.md`)
- **ADR-002** — `closing_day` derivado **e cacheado** (não é entrada, não é dropado) para preservar o contrato de `internal/transactions`. (`adr-002-closing-day-derived-cached.md`)
- **ADR-003** — Melhor dia de compra como domain service puro reutilizando aritmética de calendário de `billing_cycle.go`; `best = closing + 1`; wrap por data real; `now`/`tz` explícitos. (`adr-003-purchase-day-pure-service.md`)
- **ADR-004** — Remoção cirúrgica do alerta de limite de cartão em budgets, reduzindo o tipo fechado `ThresholdAlertKind` a `Category`/`Goal`. (`adr-004-budgets-cardlimit-removal.md`)
- **ADR-005** — Consolidação de identificação em `Nickname` (dropar coluna/VO `Name`), mantendo o índice único parcial por `(user_id, nickname)`. (`adr-005-consolidate-nickname.md`)

### Riscos Conhecidos

- **`closing_day` cacheado desatualiza** se a linha em `banks` mudar `days_before_due` depois do cadastro. Mitigação: recomputar `closing_day` no `UpdateCard` quando `bank`/`due_day` mudam (RF-07); a atualização de linha de `banks` não retroalimenta cartões existentes — comportamento documentado; reconciliação em massa é fora de escopo (PRD "Fora de Escopo").
- **Deriva de `closing_day` com `due_day` pequeno** (wrap de mês): o dia-do-mês depende do comprimento do mês de referência (`now`). Determinístico por design (ADR-003); coberto por teste de borda. Para os 8 bancos seed (7-8 dias) só ocorre com `due_day ≤ 8`.
- **Ordering de rota chi** (`best-purchase-day` vs `{id}`): mitigado por registro antes de `Route("/{id}")` + teste (`router_test.go`).
- **Mocks desatualizados** após remoção de métodos de interface: regenerar via mockery; gate de build.
- **Linhas órfãs** `budget_alerts_sent.kind='card_limit_near'` e eventos legados: inertes (sem produtor/consumidor); sem cartões/uso em produção ⇒ sem limpeza necessária.

### Conformidade com Padrões

- `.claude/rules/go-adapters.md` (R-ADAPTER-001): handlers/consumers/producers finos; **zero comentários** em `.go`; sem SQL fora de repositório. O novo handler `best_purchase_day` só mapeia→usecase.
- `.claude/rules/input-dto-validate.md` (R-DTO-001..004): `CreateCard`/`UpdateCard`/`BestPurchaseDay` com `Validate()` (`errors.Join`, campo nomeado), chamado logo após `defer span.End()`.
- `.claude/rules/go-testing.md` (R-TESTING-001): suites whitebox, `fake.NewProvider()`, IIFE por mock.
- `domain-modeling.md` (DMMF): `BankCode` smart constructor (P1/P2); `PurchaseDayService.Decide` puro (P6); output tipado; **anti-padrões proibidos** (Result/Either, currying, DSL) não usados.
- `go-implementation` R0 (sem `init()`), R5.12 (sem `panic` em produção), R6 (`context` só na fronteira; interface no consumidor `BankDaysReader`), R7 (`fmt.Errorf("...: %w")`; `errors.Join` nos `Validate`). `time.Now().UTC()` inline no shell; sem abstração de tempo (memória `feedback_no_time_abstraction`).
- `.claude/rules/transactions-workflows.md` (R-TXN-004): sem `bank`/`user_id` como label de métrica.
- Padrão `defer func() { _ = rows.Close() }()` no `bank_repository` (memória `feedback_rows_close_pattern`).

### Arquivos Relevantes e Dependentes

Card (mod.): `internal/card/{domain/entities/card.go, domain/services/{decide_create_card,decide_update_card,billing_cycle}.go, domain/errors.go, application/usecases/{create_card,update_card}.go, application/dtos/input/{create_card,update_card}.go, application/dtos/output/card.go, application/mappers/card_mapper.go, application/interfaces/repository.go, infrastructure/repositories/{factory.go,postgres/card_repository.go}, infrastructure/http/server/{router.go,handlers/{create,update}.go}, infrastructure/messaging/database/{producers/invoice_due_publisher,consumers/invoice_due_notifier}.go, application/usecases/notify_invoice_due.go, module.go, openapi.yaml}`.
Card (novo): `bank_code.go`, `purchase_day.go`, `bank_days_reader.go`, `bank_repository.go`, `best_purchase_day.go` (usecase+dtos+handler).
Card (removido): `card_limit.go`, `card_name.go`, `update_card_limit.go` (dto+usecase+handler).
Budgets: ver §"Remoção cirúrgica em budgets".
Agents: `application/workflows/onboarding_workflow.go`, `application/interfaces/types.go`, `infrastructure/binding/card_manager_adapter.go`.
Migrations: `migrations/000002_card_simplification.{up,down}.sql`.
Transactions: **nenhum** (RF-14 preservado) — verificar por ausência de diff.
