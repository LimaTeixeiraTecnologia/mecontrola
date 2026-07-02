# ADR-002 — `closing_day` derivado e cacheado (não é entrada, não é dropado)

## Metadados

- **Título:** `closing_day` deixa de ser entrada do cliente e passa a ser derivado e persistido como cache
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** JailtonJunior (owner), time de plataforma
- **Relacionados:** PRD (RF-04, RF-07, RF-14), techspec.md, ADR-003

## Contexto

Hoje `closing_day` é informado pelo cliente no cadastro (`create_card.go`, `openapi.yaml`) e persistido
em `cards.closing_day`. Ele é **consumido por `internal/transactions`** para resolver a fatura de uma
compra: `card_lookup_adapter.go:56` lê `cycle.ClosingDay`/`DueDay`, e `billing_cycle_resolver.go:17-18`
usa ambos. O PRD (RF-04) determina que `closing_day` deixe de ser entrada e passe a ser derivado de
`banco + due_day`. Precisamos fazer isso **sem quebrar** o contrato de transactions (RF-14).

Duas formas de "não ser entrada": (a) **remover a coluna** e derivar sempre em tempo de leitura, ou
(b) **derivar no cadastro e persistir como cache**, mantendo a coluna legível.

## Decisão

`closing_day` passa a ser **derivado no cadastro/edição** (via `PurchaseDayService.Decide`, ADR-003) e
**persistido como cache** na coluna `cards.closing_day` existente. Deixa de ser campo de entrada em
`CreateCardRequest`/`UpdateCardRequest`, mas **permanece** como coluna e como campo derivado na resposta.
Em `UpdateCard`, quando `bank` ou `due_day` mudam, `closing_day` é **recomputado e re-persistido**
(RF-07). O contrato de leitura consumido por `internal/transactions` fica **inalterado** — nenhum arquivo
de transactions muda.

## Alternativas Consideradas

- **(b) Derivar e cachear (escolhida).** Vantagens: zero alteração em transactions; leitura barata;
  compatível com `computeCycle` atual. Desvantagens: cache pode desatualizar se a regra do banco mudar
  depois (mitigado por recomputo no update).
- **(a) Remover a coluna e derivar em runtime.** Vantagens: fonte única (sempre correto). Desvantagens:
  **quebra o contrato de transactions** (o card lookup precisaria recomputar e passar a depender de
  `banks` e de `now`), aumentando o raio de mudança e acoplando transactions à tabela de bancos. Rejeitada
  por violar o princípio de menor impacto e o RF-14.

## Consequências

### Benefícios Esperados

- RF-14 garantido por construção: transactions não é tocado.
- Simplicidade de leitura e de migração (coluna já existe).

### Trade-offs e Custos

- `closing_day` cacheado pode divergir se `banks.days_before_due` mudar após o cadastro (sem retro-
  alimentação automática). Aceito e documentado; recomputo apenas no update do próprio cartão.

### Riscos e Mitigações

- **Risco:** dado stale após alteração da regra de um banco. **Mitigação:** recomputo no `UpdateCard`
  (RF-07); reconciliação em massa declarada **fora de escopo** no PRD; sem cartões em produção hoje.
- **Rollback:** reverter para entrada manual exigiria reintroduzir o campo no contrato (coberto pelo
  `.down.sql` da migration que também restaura `limit_cents`).

## Plano de Implementação

1. `PurchaseDayService.Decide` (ADR-003) retorna `ClosingDay`.
2. `CreateCard`/`UpdateCard` gravam o `ClosingDay` derivado no `BillingCycle`.
3. Remover `closing_day` de `CreateCardRequest`/`UpdateCardRequest` e do `input.CreateCard`/`UpdateCard`.
4. Manter `closing_day` na resposta `Card` e nas queries/scan do repositório.

## Monitoramento e Validação

- Teste de integração de transactions permanece verde sem diff (prova do contrato preservado).
- Teste unitário: `UpdateCard` recomputa `closing_day` ao mudar `bank`/`due_day`.

## Impacto em Documentação e Operação

- OpenAPI: `closing_day` sai de entrada, permanece em resposta. Runbook de card, se existir.

## Revisão Futura

- Revisitar se a divergência de cache se tornar um problema real (ex.: mudança frequente de regra de
  banco) — nesse caso avaliar job de reconciliação ou derivação em runtime no card lookup.
