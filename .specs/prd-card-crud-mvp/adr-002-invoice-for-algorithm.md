# Registro de Decisão Arquitetural (ADR-002)

## Metadados

- **Título:** Algoritmo `InvoiceFor` — auto-detect de convenção + clamp por mês + regra `closing == due`
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Jailton (tech lead)
- **Relacionados:** `.specs/prd-card-crud-mvp/prd.md` (F-03, RF-01–RF-08, OBJ-05), `.specs/prd-card-crud-mvp/techspec.md`

## Contexto

Determinar em qual fatura uma compra cai é a regra de negócio crítica do módulo de transações futuro. A função precisa ser:

- Pura, determinística, sem IO, sem `time.Now`, sem `clock.Clock` (R6.7).
- Reusável tanto por endpoint HTTP quanto por porta Go (`CardLookup`).
- Resiliente a edge cases de calendário (fev/28, fev/29, meses de 30 dias, virada de ano, DST histórico BR).
- Capaz de cobrir convenções concretas de bancos brasileiros: Nubank (fechamento no mesmo mês), Itaú/Bradesco (fechamento no mês anterior).

O PRD impõe (RF-04): se `closing_day > due_day` → fechamento no mês anterior; se `closing_day < due_day` → mesmo mês; se `closing_day == due_day` → fechamento no dia anterior ao vencimento.

## Decisão

Implementar `BillingCycle.InvoiceFor(purchase time.Time, cycle valueobjects.BillingCycle, tz *time.Location) Invoice` em `internal/card/domain/services/billing_cycle.go` com:

1. Normalização: `purchase = purchase.In(tz)` e operação em granularidade de dia.
2. `clamp(d, year, month) = min(d, daysInMonth(year, month))` aplicado tanto a `closing_day` quanto a `due_day`.
3. Auto-detect de convenção pela relação `closing_day` vs `due_day`:
   - `closing_day > due_day` → vencimento no mês seguinte ao fechamento (Itaú/Bradesco).
   - `closing_day < due_day` → vencimento no mesmo mês do fechamento (Nubank).
   - `closing_day == due_day` → fechamento = vencimento − 1 dia (convenção determinística; documentada).
4. Se `purchase.day > closing_date.day` no mês corrente → avança fechamento/vencimento em um ciclo (mês+1), aplicando o clamp novamente.
5. Retorna `Invoice{ClosingDate, DueDate}` sempre em SP.

Sem panic; entradas inválidas (data zero, day fora de `[1,31]`) já foram barradas no construtor do VO `BillingCycle`. Se chegarem mesmo assim, retorno é o mais conservador (fatura corrente).

## Alternativas Consideradas

1. **Exigir convenção explícita do usuário** (`closing_convention enum`) — Vantagens: zero ambiguidade no `closing == due`; Desvantagens: campo extra no payload, UX pior (usuário típico não sabe a terminologia), 3 fluxos de validação. Rejeitada.
2. **Algoritmo "data de corte = `due_day - N`"** (calculado pelo período de graça) — Vantagens: 1 só campo; Desvantagens: não cobre bancos onde `closing_day > due_day` (Itaú); requer modelagem adicional. Rejeitada.
3. **Recusar `closing_day == due_day` no construtor do VO** — Vantagens: elimina convenção implícita; Desvantagens: alguns bancos legitimamente operam assim; cria fricção para usuário. Rejeitada — preferimos convenção documentada + teste exaustivo.

## Consequências

### Benefícios Esperados

- Cobre os 3 cenários reais de bancos brasileiros sem campo extra.
- Função reentrante e pura → trivialmente testável (≥ 50 fixtures + property-based `MaxCount=10000`).
- p99 < 10 ms (RF — alvo M-04) viável: zero alocação fora de `time.Date`.

### Trade-offs e Custos

- A convenção `closing == due` exige documentação clara em manual de usuário e em mensagens de UI; ambíguo se não comunicado.
- Property-based test eleva tempo de CI em ~1–2s (aceitável).

### Riscos e Mitigações

- **DST histórico BR** → testes com `2018-10-21` e `2018-11-04`; `time.LoadLocation` via `sync.Once`.
- **`tzdata` ausente em container slim** → Dockerfile deve declarar `tzdata`; validar em smoke test do build.
- **Edge case 31 de dezembro com `closing_day=31`** → coberto por fixtures de virada de ano.

## Plano de Implementação

1. VOs `CardName`, `Nickname`, `BillingCycle` com validação no construtor.
2. `domain/services/timezone.go` (`SaoPauloLocation` via `sync.Once`).
3. `domain/services/billing_cycle.go` puro.
4. Testes: ≥ 50 fixtures table-driven + property-based com 4 invariantes (a–d do RF-45).
5. Endpoint `GET /cards/{id}/invoices?for=<date>` + porta `CardLookup` consumindo `InvoiceFor`.

Adoção concluída quando: cobertura ≥ 95% no `billing_cycle.go` + property-based verde em CI.

## Monitoramento e Validação

- Span `card.domain.invoice_for` com atributo `outcome=current|next` (ramo de cálculo).
- Log `card.invoice_for.computed` (sem PII).
- Critério de revisão: 0 bug report de "fatura errada" em 30 dias após release.

## Impacto em Documentação e Operação

- `docs/runbooks/card-rollback.md` não impactado.
- Adicionar nota em onboarding do módulo de transações sobre porta `CardLookup`.

## Revisão Futura

Revisitar se:

- Banco brasileiro relevante adotar convenção fora dos 3 cenários cobertos.
- Política de cartão por parcelas (compra parcelada) exigir variante de `InvoiceFor`.
- Houver mudança de timezone canônico (improvável; mantemos SP).
