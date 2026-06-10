# ADR-005 — Job periódico `pending_events_reaper` para retomada e expiração de eventos

## Metadados

- **Título:** Job único cobrindo aplicação tardia e expiração de 24h de eventos pendentes
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time MeControla / AI Agent
- **Relacionados:** [PRD v24](./prd.md) (RF-38, RF-39, RF-39a, RF-39c, RT-30), [techspec.md](./techspec.md)

## Contexto

- Eventos cross-module podem chegar fora de ordem (lacuna de versão) ou antes da criação canônica da despesa. RF-38 permite manter em `pending` por até 24h; RF-39 exige transição para `expired` após esse prazo sem aplicabilidade.
- A tabela `budgets_expense_events_pending` mantém máquina `pending → applied | failed | expired` (RF-39a). `failed` é reservado para erro **permanente** (validação de schema, autorização, identidade canônica inválida, versão definitivamente impossível) — decidido no momento da ingestão pelo consumer.
- Não há broker externo (OUT-09); o repositório usa `internal/platform/worker` para schedulers in-process (RT-30).
- RT-25 limita cardinalidade de métricas — labels permitidos: `module`, `state`, `source`.

## Decisão

Implementar **um único** job `budgets_pending_events_reaper` em `internal/budgets/infrastructure/jobs/handlers/` com `Schedule()` configurável (default 30s, env `BUDGETS_PENDING_REAPER_INTERVAL`).

Comportamento por execução:

1. SELECT pendentes ordenados por `received_at` ASC com limite (default 200; FOR UPDATE SKIP LOCKED para suportar futura paralelização).
2. Para cada evento:
   - Se `now() - received_at > 24h` → transição para `expired` com `reason="ttl_24h"`. Métrica + log estruturado. Não cria nem altera estado financeiro.
   - Caso contrário, chama `apply_pending_event` use case:
     - Carrega o agregado de despesa por identidade canônica.
     - Se `expected_version == current_version + 1` (para create: `expected_version == 1` e despesa inexistente; para update/delete: `expected_version == current_version + 1`) → aplica a mutação (mesmo caminho de `UpsertExpense`/`DeleteExpense`, em tx) e transição para `applied`.
     - Se `expected_version <= current_version` → transição para `applied` com `reason="version_obsolete_idempotent"` (já aplicado por outra rota) sem alterar estado financeiro.
     - Se ainda houver lacuna (`expected_version > current_version + 1`) → permanece em `pending`, sem incrementar `received_at`.
3. Métricas e logs por execução (contagens por outcome).

## Alternativas Consideradas

1. **Tentativa reativa após cada commit**.
   - Vantagens: latência menor para aplicar pendentes do mesmo usuário.
   - Desvantagens: não cobre expiry de 24h sem segundo mecanismo; acopla caminho de escrita à fila; duplica lógica de transição.
   - Rejeitada.

2. **Híbrido (reativo + cron de expiry)**.
   - Vantagens: mistura dos dois.
   - Desvantagens: dois caminhos de transição a manter; risco de race em transitions concorrentes.
   - Rejeitada por sobrecarga no MVP.

## Consequências

### Benefícios Esperados

- Um único ponto de execução, fácil de raciocinar e observar.
- Expiry e retomada compartilham mesmo SELECT (eficiência).
- Compatível com paralelização horizontal via `FOR UPDATE SKIP LOCKED` quando volume crescer.

### Trade-offs e Custos

- Latência mínima de até 30s entre chegada e aplicação. Aceitável para o MVP — eventos cross-module são tipicamente intra-segundo, e pendentes são casos de borda.
- Polling consome 1 query/30s; trivial.

### Riscos e Mitigações

- **Risco:** lote grande paralisar o job.
  - **Mitigação:** limite por execução (200) + horizonte de execução curto; métrica `budgets_pending_oldest_seconds` alerta crescimento anormal.
- **Risco:** retry repetido por evento sempre falhar.
  - **Mitigação:** estado `pending` mantém `received_at` original; após 24h vai para `expired`. Erros permanentes detectáveis são marcados como `failed` no ingress (não no reaper), evitando retry infinito.

## Plano de Implementação

1. Migration cria `budgets_expense_events_pending` + índice parcial `WHERE state = 1`.
2. Repositório `pending_event_repository.go` com `Insert`, `ListReady(limit)`, `Transition`.
3. Use case `apply_pending_event.go` (decide outcome).
4. Job handler `pending_events_reaper.go` implementando interface `worker.Job` (Name/Schedule/Run).
5. Wire-up em `cmd/worker/worker.go` na lista `jobs`.
6. Integration test cobre os três outcomes principais (`applied`, `obsolete idempotent`, `expired`).

## Monitoramento e Validação

- `budgets_pending_events_total{state,source}` — gauge derivada de query agregada (job de scrape opcional).
- `budgets_pending_reaper_processed_total{outcome}` — contador por execução.
- `budgets_pending_oldest_seconds{source}` — idade do mais antigo em `pending`.

## Impacto em Documentação e Operação

- Esquema documentado.
- Runbook: como investigar evento expirado (inspeção SQL — endpoint admin HTTP fora do MVP por OUT-16).
- Configurar env `BUDGETS_PENDING_REAPER_INTERVAL` e `BUDGETS_PENDING_TTL_HOURS` (default 24).

## Revisão Futura

- Reavaliar intervalo se `budgets_pending_oldest_seconds` p99 sustentadamente > 60 s.
- Reavaliar quando endpoint admin HTTP for adicionado pós-MVP (substituir consulta SQL por listagem paginada).
