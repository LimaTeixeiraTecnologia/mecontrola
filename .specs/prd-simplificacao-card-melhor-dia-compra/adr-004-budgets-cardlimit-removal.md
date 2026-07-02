# ADR-004 — Remoção cirúrgica do alerta de limite de cartão em `internal/budgets`

## Metadados

- **Título:** Remover a feature de alerta de threshold por limite de cartão, preservando categoria e metas
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** JailtonJunior (owner), time de plataforma
- **Relacionados:** PRD (RF-05, RF-15, RF-19), techspec.md

## Contexto

`limit_cents` é removido de `internal/card` (RF-05). Hoje `internal/budgets` **depende criticamente**
desse campo: o alerta de threshold por limite de cartão lê `cards.limit_cents` via
`card_threshold_reader.go` (query com JOIN em `mecontrola.cards`, `c.limit_cents`) e o usecase
`evaluate_threshold_alerts.go` monta snapshots de cartão (`buildCardSnapshots`). Porém o mesmo usecase
avalia **três** tipos no tipo fechado `services.ThresholdAlertKind`: `ThresholdAlertCategory`
(orçamento por categoria) e `ThresholdAlertGoal` (metas) — feature core do módulo — além de
`ThresholdAlertCardLimit`. Remover a dependência de `cards.limit_cents` **não pode** amputar categoria/
metas. O usuário decidiu **remover** a feature de alerta por limite de cartão (não realocá-la).

## Decisão

Remover **cirurgicamente** apenas a trilha `ThresholdAlertCardLimit`, reduzindo o tipo fechado
`ThresholdAlertKind` a `Category` e `Goal` (DMMF state-as-type preservado). Lista fechada:

**Remover arquivos:** `internal/budgets/application/interfaces/card_threshold_reader.go`;
`internal/budgets/infrastructure/repositories/postgres/card_threshold_reader.go`;
`internal/budgets/application/interfaces/mocks/card_threshold_reader.go`.

**Editar:** `domain/services/threshold_workflow.go` (remover const `ThresholdAlertCardLimit` e seus
`case` em `String()` e `thresholdForKind()`); `application/interfaces/repository_factory.go` (remover
método `CardThresholdReader`); `infrastructure/repositories/factory.go` (remover impl);
`application/usecases/evaluate_threshold_alerts.go` (remover `cardReader`, `activeCards`,
`buildCardSnapshots`; guarda vira `if len(active) == 0`); `application/usecases/notify_threshold_alert.go`
(remover `case ThresholdAlertCardLimit`); `infrastructure/repositories/postgres/
threshold_alert_sent_repository.go` e `infrastructure/messaging/database/consumers/
threshold_alert_notifier.go` (remover `case "card_limit_near"`); `module.go` (remover
`cfg.Card`/`cardRatio` do `ThresholdConfig`); regenerar `mocks/repository_factory.go`.

**Testes:** remover `TestDecideAlerts_CardLimit` (`threshold_workflow_test.go`) e os 3 testes de cartão
em `evaluate_threshold_alerts_test.go`; ajustar os 5 testes que faziam `EXPECT().CardThresholdReader(...)`.

## Alternativas Consideradas

- **Remover a trilha de cartão (escolhida).** Vantagens: alinha com a decisão do usuário; elimina a
  dependência de `cards.limit_cents`; escopo fechado e auditável. Desvantagens: perde-se o alerta de
  limite de cartão como funcionalidade.
- **Realocar o limite para budgets (origem própria).** Vantagens: preserva a funcionalidade. Desvantagens:
  novo input/coluna e UX de limite fora do escopo desta entrega; rejeitada pelo usuário.
- **Manter `limit_cents` em cards fora de escopo.** Contradiz RF-05; rejeitada no PRD.

## Consequências

### Benefícios Esperados

- `cards.limit_cents` deixa de ser lido por qualquer módulo (fecha RF-05).
- `ThresholdAlertKind` mais simples; menos superfície de código.

### Trade-offs e Custos

- Perda do alerta de limite de cartão. Reintrodução futura exigirá nova origem de dado (fora de escopo).

### Riscos e Mitigações

- **Risco:** amputar categoria/metas por engano. **Mitigação:** `buildSnapshots` (categoria/metas)
  **não é tocado**; testes de categoria/metas permanecem verdes; gate de build + suíte.
- **Risco:** linhas `budget_alerts_sent.kind='card_limit_near'` órfãs. **Mitigação:** inertes (sem
  produtor/consumidor após a remoção); sem uso em produção; sem limpeza necessária.
- **Rollback:** reverter o conjunto de edições; nenhuma migration envolvida em budgets.

## Plano de Implementação

1. Remover os 3 arquivos e editar os pontos listados.
2. Regenerar mocks (mockery).
3. Ajustar/remover testes; rodar suíte de budgets.
4. Auditar dashboards/alertas para o label `card_limit_near`.

## Monitoramento e Validação

- Métrica `budgets_threshold_alerts_dispatched_total` deixa de emitir `kind="card_limit_near"`;
  `category_threshold` e `goal_achieved` seguem. Validar painéis/queries que filtravam o valor removido.
- Sucesso: suíte de budgets verde; grep por `ThresholdAlertCardLimit`/`CardThresholdReader`/
  `card_limit_near`/`ActiveCardForScan` retorna vazio.

## Impacto em Documentação e Operação

- `docs/dashboards/*budgets*`, `docs/alerts/*budgets*`, runbook de budgets: remover referências a
  `card_limit_near`.

## Revisão Futura

- Revisitar se o produto decidir reintroduzir alerta de limite de cartão com origem de dado própria de
  budgets.
