# ADR-004 — Debounce/coalescing por `(user_id, ref_month)` no consumer de `MonthlySummary`

## Metadados

- **Título:** Debounce de 1500 ms por chave `(user_id, ref_month)` no `MonthlySummaryRecomputeConsumer`
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Engenharia
- **Relacionados:** PRD RF-26, RT-12, RF-39; techspec "messaging/database/consumers/monthly_summary_recompute_consumer.go"; ADR-002.

## Contexto

Mutação em `Transaction` ou `CardPurchase` dispara recompute de `MonthlySummary` para todas as competências afetadas. Cenários de pico real:

- Job de recorrência diário materializa 30 templates de um mesmo usuário no mesmo `day_of_month` (ADR-002).
- Usuário cadastra em lote (front-end ou import futuro) 50 lançamentos no mesmo mês.
- PATCH de compra parcelada de 24x gera evento com `ref_months_affected` de até 24 competências; cada competência dispara 1 recompute.

Sem coalescing, cada evento vira 1 recompute completo (`SUM(transactions) + SUM(card_invoice_items)` por `(user_id, ref_month)`). 30 eventos → 30 recomputes redundantes da mesma chave em segundos. Custo desnecessário.

## Decisão

`MonthlySummaryRecomputeConsumer` aplica **debounce/coalescing** por chave `(user_id, ref_month)` em janela configurável `OutboxConfig.MonthlySummaryDebounceWindow` (default `1500 ms`):

1. Consumer recebe envelope do dispatcher.
2. Extrai `(user_id, ref_month)` de cada item em `ref_months_affected` (ou do payload direto em `transaction.*`).
3. Para cada chave:
   - Se já existe timer pendente → reset (`timer.Reset(window)`) e descarta o evento atual.
   - Se não existe → cria `time.Timer` com handler que chama `RecomputeMonthlySummary(ctx, user_id, ref_month)` e remove a chave do map ao final.
4. Mapa `map[key]*time.Timer` protegido por `sync.Mutex`.
5. Shutdown coordenado: `Stop()` cancela todos os timers pendentes e dispara recompute síncrono para cada chave pendente até `ShutdownTimeout` (graceful drain).

A janela nunca pode exceder o SLO RT-12 (`p95 < 5 s`); `1500 ms` deixa margem confortável.

## Alternativas Consideradas

### A. Sem debounce (recompute por evento)
- **Vantagens**: simples; latência de projeção mínima.
- **Desvantagens**: trabalho redundante massivo em picos previsíveis (ADR-002 + lote de import).
- **Motivo da rejeição**: custo desnecessário a cada execução de recorrência diária com `day_of_month` compartilhado.

### B. Recompute em batch periódico (a cada 5s, varre eventos pendentes)
- **Vantagens**: melhor agregação.
- **Desvantagens**: viola SLO `p95 < 5s` (worst-case ~10s entre commit e projeção); estado intermediário visível ao usuário por mais tempo.
- **Motivo da rejeição**: trade-off ruim entre eficiência e UX.

### C. Recompute incremental (delta) em vez de SUM completo
- **Vantagens**: O(1) por evento; sem precisar coalescing.
- **Desvantagens**: divergência cumulativa silenciosa (delta erra de centavos com soft-delete + edição); ainda precisa de reconciliação total. Aumenta complexidade do consumer.
- **Motivo da rejeição**: para o MVP, `SUM` completo + debounce é mais simples e auditável. Pode evoluir para incremental se SUM virar gargalo medido.

## Consequências

### Benefícios Esperados

- Picos de mesma chave colapsam em 1 recompute → uso de CPU/IO previsível.
- Latência de projeção ainda compatível com `p95 < 5s` (janela 1500 ms << 5000 ms).
- Métrica `transactions_monthly_summary_coalesce_factor` quantifica o ganho ao longo do tempo.

### Trade-offs e Custos

- Adiciona estado em memória no consumer (`map[key]*time.Timer`). Tamanho típico: ≤ 1000 chaves simultâneas (volumetria base); aceitável.
- Lifecycle do consumer precisa drenar timers pendentes no shutdown — risco de bug se mal implementado.
- Debounce muda semântica de "evento → recompute" para "evento → recompute eventual"; testes precisam usar `time.Sleep` mínimo (1500 ms + folga) ou injetar window pequena em ambiente de teste.

### Riscos e Mitigações

- **Risco**: timer não disparado por shutdown abrupto perde recompute.
  - **Mitigação**: outbox at-least-once + job de reconciliação diária (RF-27) cobrem; nenhum dado é perdido, apenas projeção fica desatualizada por até 1 dia. Alerta de drift detecta.
- **Risco**: vazamento de timers em shutdown sem `Stop()`.
  - **Mitigação**: integration test obrigatório com `consumer.Stop()` → assert que todos os timers foram cancelados e drain completou; gate de lifecycle de `graceful-lifecycle.md`.
- **Risco**: janela curta demais (1500 ms) anula ganho em picos espaçados de 2s.
  - **Mitigação**: janela é configurável via env (`OUTBOX_MONTHLY_SUMMARY_DEBOUNCE_WINDOW`); ajustável sem deploy. Monitor `coalesce_factor` define se precisa ajustar.

## Plano de Implementação

1. Estrutura `coalescer` com `map[key]*time.Timer` + `sync.Mutex`.
2. Método `Schedule(key, fn)`: reset timer se existir, criar se não.
3. Método `Stop()`: percorre map, para todos os timers e executa `fn` síncrono para cada chave restante até `ShutdownTimeout`.
4. Wire no consumer: para cada evento, extrai chaves de `ref_months_affected` e chama `Schedule`.
5. Integration test: 10 eventos da mesma chave em 200 ms → 1 recompute; 10 eventos de chaves distintas em 200 ms → 10 recomputes; shutdown durante pendência → drain.

## Monitoramento e Validação

- `transactions_monthly_summary_coalesce_factor` (Histogram): valor médio esperado > 1 em picos, ≈ 1 em uso esporádico.
- `transactions_monthly_summary_recompute_duration_seconds` p99 < 100 ms.
- `transactions_outbox_consumer_lag_seconds` p95 < 5 s (RT-12).
- Drift detectado pelo reconciler indica que coalescing/recompute falhou silenciosamente.

## Impacto em Documentação e Operação

- Runbook `transactions.md`: cenário "projeção parou de atualizar" → checar consumer + timer pendente + lag.
- Configs `OUTBOX_MONTHLY_SUMMARY_DEBOUNCE_WINDOW` documentado em `configs/config.go`.

## Revisão Futura

- Se `coalesce_factor` médio ficar consistentemente ≈ 1 (sem ganho), reduzir window para 500 ms ou remover debounce.
- Se `recompute_duration_seconds` virar gargalo (> 100 ms p99), avaliar recompute incremental (alternativa C rejeitada acima).
