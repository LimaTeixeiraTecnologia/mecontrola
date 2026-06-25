# ADR-007 — Remoção de Eventos Órfãos (inclusive cross-module) com Guarda Anti-Falso-Positivo

## Metadados

- **Título:** Eliminar producers/consumers de eventos sem par, confirmados por constante de event-type
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma + donos de transactions/budgets
- **Relacionados:** PRD (RF-38, RF-41), techspec §"Eventos órfãos", `R-TXN-WORKFLOWS-001`

## Contexto

A descoberta encontrou eventos publicados sem consumidor real. Há **risco de falso positivo**: vários
eventos são publicados via **constante de event-type da entidade** (não por arquivo nomeado
`producer`), e um grep ingênuo por nome de arquivo marca falso-órfão (ex.: `onboarding.splits_calculated`
parece sem producer, mas é publicado por `save_onboarding_budget_splits.go` e **consumido** por
budgets). O PRD pede limpeza **inclusive cross-module**, com guarda anti-falso-positivo.

## Decisão

Remover eventos órfãos **confirmados por constante de event-type** cruzada com **registros reais de
consumer** (`EventHandlerRegistration`/handlers de módulo), excluindo testes/e2e/`.feature`:

**[REMOVER]** (producer real, zero consumer):
- `agent.intent.rejected.v1` (e provável `agent.intent.executed.v1` — confirmar no mesmo passo) —
  `intent_event_publisher.go`.
- `budgets.budget_activated.v1` — `budget_activated_publisher.go` + interface + chamadas em
  `activate_budget.go` e `edit_category_percentage.go`.
- `transactions.recurring_template.{created,updated,deleted}.v1` — `recurring_template_event_publisher.go`
  + interface + mock + 3 usecases.

**[MANTER]** (têm par produtor+consumidor — guarda anti-falso-positivo):
- `transactions.card_purchase.deleted.v1` (consumido pelo recompute mensal).
- `onboarding.splits_calculated`, `onboarding.card_registered`, `onboarding.completed` (consumidos por budgets/card/agent).

**[REMOVER]** (corrigido pela Errata 2026-06-25 #3 — supera a linha `[MANTER]` original):
- `external.expense.v1` — consumer-sem-producer (zero producer registrado); pipeline inteiro de ingestão removido (`IngestExternalExpense`, `ExternalExpenseConsumer`, `IngestExternalExpenseCommand`, `ExternalExpenseStrategy`).
- `onboarding.income_registered` — producer-sem-consumer (zero consumer registrado); renda flui via `onboarding.splits_calculated`. Publish removido de `save_onboarding_income.go`; entidade `IncomeRegistered` removida.

**[REMOVER com Telegram]** (ADR-005): `agent.telegram.inbound.v1` (par publisher+consumer).

Cada remoção cross-module respeita as regras do módulo dono (`R-TXN-WORKFLOWS-001`: producers só
mapeiam domain event; remover producer não pode deixar usecase chamando publisher inexistente).

## Alternativas Consideradas

- **Não mexer em eventos**: mantém código morto; contraria "0 lacunas". Rejeitada.
- **Remover por nome de arquivo (grep ingênuo)**: gera falso positivo (remove evento com par).
  Rejeitada — viola "0 falso positivo".

## Consequências

### Benefícios Esperados

- Remoção de outbox/handlers mortos; contrato de eventos enxuto e verdadeiro.

### Trade-offs e Custos

- Toca transactions/budgets (cross-module); exige confirmação por constante e cuidado com o outbox.

### Riscos e Mitigações

- **Risco:** remover evento consumido externamente (fora do repo). **Mitigação:** confirmar por
  constante + ausência de consumer **e** ausência de contrato externo documentado; na dúvida, manter.
- **Risco:** usecase chamando publisher removido. **Mitigação:** remover a chamada no mesmo PR;
  `go build` falha se sobrar. **Rollback:** reverter PR.

## Plano de Implementação

1. Para cada candidato, localizar a constante e contar consumers reais (excluindo testes). 2. Remover
   producer + interface + mock + wiring + chamadas nos usecases, por bloco. 3. `go build`/`go test`.
4. `deadcode`/`staticcheck` para resíduo.

## Monitoramento e Validação

- Sucesso: zero producer sem consumer e zero consumer sem producer (confirmado por constante); build/
  test verdes; outbox sem tipos órfãos.

## Impacto em Documentação e Operação

- Atualizar catálogo de eventos/diagramas; runbooks de messaging.

## Revisão Futura

- Reavaliar `agent.intent.executed.v1` se um consumidor de auditoria for introduzido.
