# ADR-003 — Melhor dia de compra como domain service puro reutilizando aritmética de calendário

## Metadados

- **Título:** Cálculo de fechamento/melhor-dia como `PurchaseDayService.Decide` puro (DMMF Princípio 6)
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** JailtonJunior (owner), time de plataforma
- **Relacionados:** PRD (RF-08, RF-11, RF-12, RF-13), techspec.md, ADR-001, ADR-002; `domain-modeling.md`

## Contexto

RF-08 exige uma função pura (sem IO, sem `context.Context`, determinística) que, dados
`days_before_due` e `due_day`, calcule `fechamento = due_day - days_before` e `melhor_dia = fechamento + 1`.
RF-11 exige tratamento de virada de mês e dias inválidos (fora de 1..31, meses curtos), reutilizando a
lógica `clamp`/`advanceMonth` já existente em `internal/card/domain/services/billing_cycle.go:62-76`.
RF-12 exige que a consulta seja pura e não dependa de cartão persistido. A subtração `due_day -
days_before` pode resultar em dia ≤ 0 (ex.: `due=3`, `days=7`), exigindo wrap para o mês anterior, cujo
comprimento varia.

## Decisão

Criar `internal/card/domain/services/purchase_day.go` com `PurchaseDayService.Decide(dueDay,
daysBeforeDue int, now time.Time, tz *time.Location) PurchaseDay`, onde `PurchaseDay{ClosingDay, BestDay
int}`. O cálculo usa **aritmética de data real** contra o mês de referência de `now` no timezone
configurado (`internal/card/domain/services/timezone.go`): constrói a data de vencimento com
`clamp(dueDay, ...)`, subtrai `daysBeforeDue` dias (`AddDate(0,0,-daysBeforeDue)`) para obter o
fechamento, e soma 1 dia para o melhor dia; extrai `.Day()` de cada. `now`/`tz` são parâmetros
explícitos (DMMF Princípio 6 — sem `time.Now()` no domínio; memória `feedback_no_time_abstraction`). A
resolução de `daysBeforeDue` (com fallback 7) é feita **fora** desta função, no shell do use case, via
`BankDaysReader` (ADR-001) — a função pura recebe o número já resolvido.

## Alternativas Consideradas

- **Aritmética de data real com `clamp`/`advanceMonth` (escolhida).** Vantagens: reutiliza código provado;
  trata wrap e meses curtos naturalmente; determinística com `now` explícito. Desvantagens: `closing_day`
  como `int` é aproximado quando há wrap (depende do comprimento do mês de referência).
- **Aritmética modular pura em dia-do-mês (sem data).** Ex.: `closing = due - days; if closing<=0
  closing += 30`. Vantagens: independe de `now`. Desvantagens: `30` é arbitrário; diverge do
  comportamento de `computeCycle`/`clamp`; introduz uma segunda convenção de calendário no módulo.
  Rejeitada por inconsistência com a base existente.
- **Persistir `best_purchase_day` como coluna.** Vantagens: leitura trivial. Desvantagens: dado derivável
  de `closing_day`; coluna redundante. Rejeitada — computa-se na resposta a partir de `closing_day`.

## Consequências

### Benefícios Esperados

- Função pura testável sem mock; critério de aceitação Nubank/20→(13,14) direto (RF-13).
- Reuso de `clamp`/`advanceMonth` mantém uma só convenção de calendário no módulo.

### Trade-offs e Custos

- `closing_day` armazenado é aproximado para `due_day` pequeno (wrap): o dia-do-mês depende do mês de
  referência. Para os 8 bancos seed (7-8 dias), só ocorre com `due_day ≤ 8`.

### Riscos e Mitigações

- **Risco:** interpretação ambígua de `closing_day` sob wrap. **Mitigação:** teste de borda explícito
  (`due=3`/7d, `due=31` em mês curto); comportamento documentado, não é gap.
- **Rollback:** função nova, sem dependências externas; remoção isolada.

## Plano de Implementação

1. `purchase_day.go` com `Decide` + `PurchaseDay`; reutiliza `clamp` (mesmo pacote `services`).
2. Testes unitários (tabela): 8 bancos, fallback, wrap, mês curto, exemplo do PRD.
3. Consumido por `CreateCard`/`UpdateCard` (grava `ClosingDay`) e `BestPurchaseDay` (consulta pura).

## Monitoramento e Validação

- Cobertura unitária do domínio puro; sem instrumentação no domínio (Princípio 6).
- Sucesso: todos os cenários da tabela de teste passam de forma determinística com `now` fixo.

## Impacto em Documentação e Operação

- OpenAPI expõe `best_purchase_day` na resposta e no endpoint `GET /cards/best-purchase-day`.

## Revisão Futura

- Revisitar se o produto exigir `closing_day`/`best_purchase_day` exatos por mês (ex.: agenda por
  competência) em vez de dia-do-mês representativo.
