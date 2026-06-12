# ADR-002 — Materialização de recorrência via job diário por `day_of_month`

## Metadados

- **Título:** Job diário filtrando templates por `day_of_month == today` (vs lote único dia 1)
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Produto + Engenharia
- **Relacionados:** PRD RF-31, RF-32, AS-06; techspec seção "jobs/handlers/recurring_materializer_job.go"; ADR-006 (DMMF — decisão "materializa Transaction vs CardPurchase" vive em `RecurringWorkflow.DecideMaterializeForDay`).

## Contexto

`RecurringTemplate` deve ser materializado em `Transaction` ou `CardPurchase` no mês corrente sem duplicação. Discovery técnico (AS-06) levantou duas opções:

1. **Lote único no dia 1** — uma execução mensal materializa tudo; `occurred_at` deriva de `day_of_month`.
2. **Diário por `day_of_month`** — execução diária materializa apenas templates cujo `day_of_month == hoje`.

Produto decidiu pela opção 2 (mensagem do usuário em 2026-06-12): o lançamento "aparece" na data real esperada pelo usuário, alinhado ao mental model de "salário cai no dia 5".

## Decisão

`RecurringMaterializerJob` roda **diariamente** (`@daily` ou cron equivalente em `TransactionsConfig.RecurringMaterializerCron`). A cada execução:

1. Calcula `today := time.Now().In(saoPaulo)`.
2. Itera em batches via `FindActiveByDayOfMonth(ctx, today.Day(), today, cursor, batchSize)`.
3. Para cada template:
   a. Adquire **advisory lock** por `(template_id, ref_month)` via `RecurringMaterializationRepository.TryAdvisoryLock`.
   b. Tenta `InsertIfAbsent(ctx, template_id, ref_month, ...)`; se retornar `false`, incrementa `transactions_recurring_materialize_skipped_total{reason="already_materialized"}` e segue para o próximo.
   c. Se inserido, chama `CreateTransaction` ou `CreateCardPurchase` com `occurred_at = today 12:00 America/Sao_Paulo` e atualiza `materialized_*_id` na linha de `recurring_materializations`.
   d. Libera o lock.

Templates criados após o `day_of_month` do mês corrente **não são materializados retroativamente**; só serão materializados no próximo mês. Mudanças no template afetam apenas materializações futuras (RF-34).

## Alternativas Consideradas

### A. Lote único no dia 1 (recomendação original de AS-06)
- **Vantagens**: 1 execução/mês simplifica observabilidade; menor risco de spike de carga em 30 janelas distintas.
- **Desvantagens**: lançamento aparece no extrato no dia 1 e o cliente precisa filtrar mentalmente "esse só vai cair dia 15"; quebra o mental model do usuário.
- **Motivo da rejeição**: produto priorizou aderência ao calendário real.

### C. Lote único no dia 1 + retroativo para templates novos
- **Vantagens**: cobre criação tardia sem ação manual.
- **Desvantagens**: complexidade extra para tratar `day_of_month` no passado vs futuro do mês corrente; ambiguidade ("posso criar template dia 20 com `day_of_month=5`?").
- **Motivo da rejeição**: produto preferiu regra simples — sem retroatividade.

## Consequências

### Benefícios Esperados

- Aderência ao calendário esperado pelo usuário (UX).
- Idempotência **double-layer**: `pg_try_advisory_xact_lock` como first-cut (gate da tentativa de materialização — pulou? sai sem trabalho) + PK `(template_id, ref_month)` como race-final (`INSERT ... ON CONFLICT DO NOTHING`, captura concorrência que escapou do lock). Auditado: `InsertIfAbsent` silencioso anteriormente listado como "terceira camada" é o **mesmo mecanismo** da PK; manter dupla nomenclatura é false positive retórico (audit fix #6).
- Spike de carga distribuído ao longo do mês.

### Trade-offs e Custos

- 30 janelas de observação em vez de 1 → métricas precisam de rate/dia para detectar regressão; alerta deve considerar dias sem templates ativos (não-alarme).
- Spike concentrado quando muitos templates compartilham `day_of_month` (típico: dia 5 = salário, dia 10 = aluguel).
- Job de cron precisa rodar 365× em vez de 12× — overhead operacional baixo, mas existe.

### Riscos e Mitigações

- **Risco**: relógio do servidor em fuso errado cria materialização em dia errado.
  - **Mitigação**: cálculo SEMPRE via `time.Now().In(saoPauloLoc)`; validado em integration test que força o fuso do container.
- **Risco**: dois pods do worker rodam ao mesmo tempo após reload; ambos disparam o mesmo dia.
  - **Mitigação**: advisory lock por chave + PK no upsert; reprocesso é silencioso e idempotente.
- **Risco**: usuário cadastra template dia 20 com `day_of_month=15` esperando materialização no mês corrente.
  - **Mitigação**: documentação clara no PRD (RF-31); resposta da API inclui campo `next_materialization_at` (ajuste de techspec futuro se UX exigir).

## Plano de Implementação

1. Cron diário registrado via `job.NewAdapter` em `TransactionsModule.RecurringMaterializerJob`.
2. Schema com `recurring_materializations(template_id, ref_month) PRIMARY KEY` + função SQL `pg_try_advisory_xact_lock(hashtext(...))` para lock por chave.
3. Métricas dedicadas: `attempt_total`, `skipped_total{reason}`, `duration_seconds`.
4. Integration tests obrigatórios: (a) re-execução no mesmo dia → uma só inserção, (b) dois jobs concorrentes → um materializa, outro `skipped_total{reason="lock_not_acquired"}`.

## Monitoramento e Validação

- Alerta: `increase(transactions_recurring_materialize_attempt_total[26h]) == 0` é normal (dias sem templates ativos); usar `absent_over_time` apenas para detectar job parado.
- Alerta crítico: `transactions_recurring_materialize_skipped_total{reason="lock_not_acquired"}` > 1% das tentativas indica contenção real ou bug.
- Sucesso = zero duplicação detectada por job de reconciliação diária (ADR-005 relacionada).

## Impacto em Documentação e Operação

- Runbook `transactions.md`: entrada "templates não materializaram hoje" (verificar `day_of_month == today`).
- Front-end: ao criar template com `day_of_month` no passado do mês corrente, mostrar "será aplicado em <próximo mês>".

## Revisão Futura

Revisitar se:
- Volumetria crescer para > 100k templates ativos por dia (ajustar batch size ou paralelizar).
- Produto pedir materialização retroativa para template criado no meio do mês.
