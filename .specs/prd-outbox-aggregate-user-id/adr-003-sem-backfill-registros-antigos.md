# ADR-003 — Registros antigos permanecem com aggregate_user_id NULL, sem backfill

## Metadados

- **Título:** Não fazer backfill via `payload->>'user_id'` em registros pré-migration
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) Itens Fora de Escopo; [techspec](techspec.md)

## Contexto

Após a migration `000017` aplicar, registros pré-existentes em `outbox_events` terão `aggregate_user_id NULL`. Há três opções:
- **Backfill SQL**: rodar `UPDATE outbox_events SET aggregate_user_id = (payload->>'user_id')::uuid WHERE aggregate_user_id IS NULL AND payload ? 'user_id'` em batches.
- **Backfill via aplicação**: script Go que lê registros pendentes, faz upsert do user_id.
- **Sem backfill**: aceitar NULL nos registros antigos; housekeeping (`DeletePublishedBatch`) eventualmente os limpa.

## Decisão

**Sem backfill.**

Justificativa operacional:
1. **Maioria dos registros antigos está em `status=Published`** (status=3). Housekeeping já tem rotina (`DeletePublishedBatch`) que limpa após retenção (~30 dias). Esses registros serão deletados antes de qualquer query operacional importante usar `aggregate_user_id`.
2. **Registros pendentes (`status=1`) são poucos em estado estacionário** (dispatcher processa rapidamente). Aceitar NULL temporariamente para esses não impacta operação.
3. **Registros falhos (`status=4`)** são exceção e exigem investigação manual de qualquer forma; ausência de user_id top-level não impede `payload->>'user_id'` como fallback.
4. **Custo do backfill é não-trivial**: UPDATE em milhares/milhões de linhas causa bloat de tabela, locks, replicação lag. Sem ganho proporcional.

## Alternativas Consideradas

1. **Backfill SQL atômico** — `UPDATE ... WHERE aggregate_user_id IS NULL`. **Rejeitada**: lock + bloat + replicação lag em tabelas grandes; ganho marginal.
2. **Backfill em batches via cron** — processo gradual. **Rejeitada**: adiciona job operacional; mesmos ganhos marginais.
3. **Backfill apenas para `status=1` pendentes** — surgical, baixo risco. **Rejeitada**: lista provavelmente vazia (dispatcher é rápido); over-engineering para edge case.

## Consequências

### Benefícios Esperados

- Migration aplica em segundos (ALTER TABLE ADD COLUMN UUID NULL é metadata-only em Postgres).
- Zero job operacional adicional.
- Zero risco de bloat/locks.

### Trade-offs e Custos

- Queries operacionais por `aggregate_user_id` durante a janela de retenção (~30 dias) podem perder registros antigos. Aceito: queries operacionais por user em outbox são raras; quando necessárias, fallback `OR (payload->>'user_id')::uuid = $1` está disponível.
- Métrica `has_user_id="false"` em registros antigos pode mascarar bug em novos callers. **Mitigação**: métrica é INSERT-time (não scan), portanto reflete apenas registros novos. Registros antigos não entram na métrica.

### Riscos e Mitigações

- **R-01**: alguém futuramente espera que `aggregate_user_id` esteja sempre presente (post-v2 NOT NULL). **Mitigação**: ADR-001 cravou critério "30 dias com `has_user_id="false"` = 0" antes de aplicar NOT NULL — após esse período, registros antigos já saíram da tabela via housekeeping.
- **R-02**: incidente de produção exige investigação imediata por user em janela de 24h, registros antigos faltam user_id. **Mitigação**: fallback SQL via payload JSON está documentado no runbook de incidente.

## Plano de Implementação

Nenhum. Esta ADR documenta a ausência de implementação de backfill.

## Monitoramento e Validação

- Confirmar via SELECT que registros pós-migration têm `aggregate_user_id` populado.
- Após 30 dias, confirmar que `SELECT count(*) FROM outbox_events WHERE aggregate_user_id IS NULL` retorna 0 ou próximo.

## Impacto em Documentação e Operação

- Runbook de incidente: nota sobre fallback `(payload->>'user_id')::uuid` para registros pré-migration.

## Revisão Futura

Revisar se:
- Houver demanda real por consulta histórica por user no outbox.
- Mudar política de retenção (`DeletePublishedBatch` parar de limpar).
- Data sugerida: ao atingir critério v2 da ADR-001.
