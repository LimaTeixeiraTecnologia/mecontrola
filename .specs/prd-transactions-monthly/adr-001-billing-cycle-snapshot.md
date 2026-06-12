# ADR-001 — Snapshot estático de `BillingCycle` em `CardPurchase`

## Metadados

- **Título:** Snapshot estático de `closing_day`/`due_day` em `CardPurchase`; sem consumir eventos de `internal/card`
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Produto + Engenharia (autor: agente IA via skill `create-technical-specification`)
- **Relacionados:** PRD `.specs/prd-transactions-monthly/prd.md` (RT-20, OUT-05, OUT-17, RF-12); techspec `.specs/prd-transactions-monthly/techspec.md`.

## Contexto

`CardPurchase` precisa derivar a competência (`ref_month`) de cada parcela a partir das datas oficiais do cartão (`closing_day`, `due_day`), gerenciadas em `internal/card`. Existem duas formas conhecidas de obter esses dados:

1. **Snapshot estático no momento da criação** — ler uma vez via `CardLookup`, gravar nas colunas `card_closing_day`/`card_due_day` da `card_purchases`, e nunca mais consultar o cartão.
2. **Consumo reativo de eventos `card.updated.v1`** — manter um `card_billing_cycle_cache` local atualizado por consumer e reler em cada operação que toque a compra.

O MVP está sob pressão de prazo, há baixo histórico de mudanças frequentes de `closing_day`/`due_day` na operação real, e o consumidor de eventos exige contrato adicional do `internal/card` (que hoje não publica esse evento).

## Decisão

Adotar **snapshot estático** no momento da criação do `CardPurchase`. Os campos `card_closing_day` e `card_due_day` são lidos uma única vez via `CardLookup.GetForUser(ctx, cardID, userID)` e persistidos na própria tabela `card_purchases`. Edições subsequentes nas datas do cartão **não** retroagem na compra. Para corrigir, o usuário edita a compra (PATCH), que dispara `BillingCycleResolver` usando o snapshot vigente na compra.

Escopo da decisão:
- Apenas `internal/transactions`; `internal/card` permanece a fonte de verdade do cartão vigente.
- Vale para todos os fluxos: criação direta de `CardPurchase`, materialização de `RecurringTemplate` com `payment_method=credit_card`.

## Alternativas Consideradas

### A. Consumir eventos `card.updated.v1` e re-projetar compras afetadas
- **Vantagens**: usuário não precisa de ação manual para corrigir compras quando muda data do cartão.
- **Desvantagens**: exige novo evento contratual em `internal/card` + consumer dedicado em `transactions` + lógica de re-projeção em cascata em N compras (potencialmente milhares por cartão). Complexidade fora do MVP.
- **Motivo da rejeição**: custo alto vs benefício marginal (volumetria base do PRD não mostra mudança de datas como evento frequente); aumenta acoplamento entre módulos.

### B. Reler `closing_day`/`due_day` a cada operação na compra
- **Vantagens**: zero schema novo em `card_purchases`.
- **Desvantagens**: cada PATCH/GET vira chamada cross-module; falha de `internal/card` paralisa leitura; competência muda silenciosamente entre criação e edição (inconsistência grave).
- **Motivo da rejeição**: viola RT-20 e fere previsibilidade ("a fatura mudou sozinha sem eu editar nada").

## Consequências

### Benefícios Esperados

- Desacoplamento total no MVP: `transactions` não consome eventos de `card`.
- Auditabilidade: a competência de qualquer parcela é determinística a partir das colunas da `card_purchases`.
- Performance: zero round-trip ao `internal/card` em leitura/edição de compra.

### Trade-offs e Custos

- Compras antigas não refletem mudança de data do cartão. Usuário precisa editar a compra para forçar recálculo.
- UX confuso se o cliente não comunicar a regra; OUT-17 e mitigação em risco "snapshot estático causa surpresa" cobrem.

### Riscos e Mitigações

- **Risco**: usuário muda data, mas as parcelas antigas continuam na competência antiga; suporte recebe ticket.
  - **Mitigação**: documentar comportamento na release note + tooltip no front-end ("competência baseada nas datas do cartão no momento da compra").
- **Risco**: técspec v2 precisa reabrir essa decisão se telemetria mostrar > 5% de tickets sobre o tema.
  - **Plano de rollback**: ADR substituta com evento `card.updated.v1` + re-projeção; migração de schema adicional.

## Plano de Implementação

1. Adicionar colunas `card_closing_day SMALLINT NOT NULL` e `card_due_day SMALLINT NOT NULL` em `card_purchases` (migration `000014`).
2. `CreateCardPurchase` chama `CardLookup.GetForUser`; grava snapshot na compra.
3. `UpdateCardPurchase` lê snapshot da própria compra (sem re-chamar `CardLookup`) ao recalcular parcelas via `BillingCycleResolver`.
4. Testes de integração validam que mudança em `internal/card` não afeta compras existentes.

## Monitoramento e Validação

- Métrica `transactions_card_lookup_failure_total` para observar saúde da única chamada cross-module.
- Telemetria de tickets de suporte ("minha fatura está errada" / "mudei o vencimento e nada mudou") em homologação por ≥ 30 dias.

## Impacto em Documentação e Operação

- OUT-17 do PRD documenta o trade-off.
- Runbook `docs/runbooks/transactions.md` precisa de entrada "como corrigir competência de compra após mudança de cartão" (edição manual pelo usuário).
- Front-end precisa de tooltip explicativo.

## Revisão Futura

Revisitar em 6 meses (≈ 2026-12-12) ou imediatamente se:
- `> 5%` dos tickets do módulo forem sobre incoerência de competência após mudança de cartão.
- `internal/card` ganhar evento `card.billing_cycle_changed.v1` por outra demanda — re-projeção fica cheap.
