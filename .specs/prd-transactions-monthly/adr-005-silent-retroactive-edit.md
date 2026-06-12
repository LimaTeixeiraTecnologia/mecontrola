# ADR-005 — Edição retroativa silenciosa em faturas fechadas

## Metadados

- **Título:** Edição de `CardPurchase` cascateia em faturas fechadas sem confirmação; response/evento expõem `ref_months_affected`
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Produto + Engenharia
- **Relacionados:** PRD RF-17, RF-36, OUT-04; techspec "usecases.UpdateCardPurchase"; ADR-003 (single event); ADR-006 (DMMF — cascata calculada no `Decide*` puro).

## Contexto

Quando o usuário edita uma compra parcelada (`PATCH /v1/card-purchases/{id}`) cujas parcelas caem em meses já passados (faturas "fechadas"), três opções existem:

1. **Permitir sem aviso** — backend cascateia em todas as faturas; cliente decide se mostra aviso.
2. **Bloquear** — backend rejeita edição que toque fatura fechada; usuário precisa estornar+recriar.
3. **Flag opt-in (`cascade_closed=true`)** — backend só cascateia se cliente sinalizar.

Produto decidiu pela opção 1 (mensagem do usuário em 2026-06-12), reconhecendo o trade-off de OUT-04: ausência de imutabilidade pós-pagamento no MVP.

## Decisão

`UpdateCardPurchase` cascateia silenciosamente em todas as parcelas, **inclusive em faturas com `ref_month < today`**, dentro da mesma transação SQL. Não há flag, prompt ou modo bloqueante no backend.

Para compensar a opacidade da operação:

- **Response do PATCH** inclui `ref_months_affected: []string` (ordenado ascendente, deduplicado) — união das competências antigas e novas.
- **Evento `transactions.card_purchase.updated.v1`** carrega o mesmo array (ADR-003) para que consumidores reconciliem todas as projeções afetadas.
- Front-end pode (mas não é obrigado a) usar `ref_months_affected` para mostrar modal "isso altera N faturas, incluindo meses passados".

## Alternativas Consideradas

### A. Bloquear edição em fatura fechada
- **Vantagens**: integridade contábil percebida ("o que está fechado, fica fechado").
- **Desvantagens**: usuário força fluxo via DELETE + CREATE para corrigir erro de digitação; complexidade UX; quebra o modelo mental "edito e pronto".
- **Motivo da rejeição**: produto priorizou edição direta; correção de erros é caso de uso mais comum que reabertura contábil.

### B. Flag opt-in (`cascade_closed=true`)
- **Vantagens**: controle explícito; UX gradual.
- **Desvantagens**: dois caminhos no backend; clientes desatualizados experimentam comportamento inconsistente.
- **Motivo da rejeição**: produto preferiu API única; cliente decide a UX.

## Consequências

### Benefícios Esperados

- API simples: 1 operação cobre todos os casos de edição.
- Implementação backend mais barata (uma única lógica de cascata).
- `ref_months_affected` no evento elimina ambiguidade para o consumer.

### Trade-offs e Custos

- Usuário pode editar uma compra de 12 meses atrás e o resumo daquele mês muda — comportamento "surpresa" se o cliente não exibir aviso.
- Não há trilha de "como era antes" no MVP (sem audit log de versões anteriores do `CardPurchase` além do outbox de eventos).
- Se cliente front-end ignorar `ref_months_affected`, telas de meses passados em cache exibem dado obsoleto até refresh.

### Riscos e Mitigações

- **Risco**: usuário fecha o app sem ver o impacto retroativo da edição → desconfiança ("por que meu fechamento de 3 meses atrás mudou?").
  - **Mitigação**: cliente DEVE consumir `ref_months_affected` para invalidar cache de meses; tooltip no front-end "edição altera faturas históricas".
  - **Mitigação**: outbox preserva trilha completa (`created → updated`) para auditoria via SQL no runbook.
- **Risco**: cascata em compra de 24 parcelas com `ref_months_affected` espalhado em 24 meses sobrecarrega o consumer de `MonthlySummary`.
  - **Mitigação**: debounce/coalescing (ADR-004) coalescá os 24 recomputes; integration test cobre.
- **Risco**: regulatório futuro exigir imutabilidade pós-pagamento.
  - **Plano de rollback**: OUT-04 já reconhece a dívida; ADR substituta com gate "edição em fatura fechada exige nova compra de ajuste" + migration de schema para `is_closed`.

## Plano de Implementação

1. `UpdateCardPurchase.Execute`:
   - Load `oldPurchase`, `oldItems`.
   - Construir `commands.UpdateCardPurchase` via `commands.NewUpdateCardPurchase(raw, principal)` (smart constructors + `errors.Join`, ADR-006 §1/§5; tipos em `internal/transactions/domain/commands/`).
   - Chamar `decision := workflow.DecideUpdate(oldPurchase, oldItems, cmd, snapshot, now)` — função pura em `internal/transactions/domain/services/card_purchase_workflow.go` que calcula `newItems`, `RefMonthsAffected = union(oldItems.refMonths, newItems.refMonths)` (deduped, ascending) e produz `decision.Event = entities.CardPurchaseUpdated{...}`. `snapshot` é `valueobjects.CardBillingSnapshot` (audit fix #2; nome único em todo o módulo).
   - **Fórmula de ajuste de `items_total_cents` (audit fix #4)**: para cada `ref_month` em `RefMonthsAffected`, `decision.InvoiceDeltas[ref_month] = sum(new_items[ref_month]) − sum(old_items[ref_month])`. Resultado positivo (acréscimo), negativo (remoção/redução) ou zero (skip UPDATE). Calculado dentro do `Decide*`, nunca no use case ou producer.
   - TX: `ReplaceItems(purchaseID, decision.Items)` (DELETE+INSERT) → para cada `(invoiceID, delta)` em `decision.InvoiceDeltas`: `invoices.ApplyDelta(ctx, invoiceID, delta, expectedVersion)` com optimistic locking (`409 conflict` em race) → `publisher.Publish(ctx, db, decision.Event)`.
   - **Proibido**: calcular `RefMonthsAffected` ou `InvoiceDeltas` no use case ou producer; ambos DEVEM vir prontos do `Decide*` (gate ADR-006).
2. Response inclui `ref_months_affected` no body de PATCH.
3. Documentar em OpenAPI o campo + comentário sobre "inclui meses passados se aplicável".
4. Integration test obrigatório: PATCH de compra 12x mudando `total_amount_cents` → todas as 12 `card_invoices.items_total_cents` atualizadas; eventos publicados; `ref_months_affected` cobre todos os meses.

## Monitoramento e Validação

- Métrica `transactions_card_purchases_updated_total{installments_bucket}` distingue volume por tamanho de parcela.
- Telemetria de tickets de suporte ("meu resumo do mês X mudou sozinho") em homologação → indicador de UX inadequada no cliente.
- Métrica futura (não-MVP): `transactions_card_purchase_retroactive_edit_total` se justificar refinamento.

## Impacto em Documentação e Operação

- OUT-04 do PRD documenta o trade-off.
- Runbook `transactions.md`: cenário "usuário reclama que mês fechado mudou" → SQL para inspecionar eventos `transactions.card_purchase.updated.v1` recentes do usuário.
- Front-end: contrato consumer de `ref_months_affected` precisa estar claro nas notas técnicas de release.

## Revisão Futura

Revisitar em 6 meses (≈ 2026-12-12) ou imediatamente se:
- Houver regulação/política obrigando imutabilidade pós-pagamento.
- Tickets sobre "fatura antiga mudou" excederem 2% do volume.
- Cliente front-end pedir flag opt-in para diferenciar UX de correção vs ajuste retroativo.
